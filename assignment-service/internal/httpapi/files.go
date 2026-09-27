// Package httpapi holds the two assignment routes that grpc-gateway cannot
// generate, because their HTTP shape is not a JSON body:
//
//	POST /api/assignments/{assignment_id}/submissions  multipart upload in
//	GET  /api/submissions/{submission_id}/file           binary file out
//
// Both RPCs are streaming, and both are deliberately left unannotated in
// assignment.proto so the generator emits no stubs for them - there is no
// sensible automatic mapping between a multipart form and a client-stream of
// chunk messages, or between a server-stream of chunks and a file download.
//
// They still go through gRPC rather than calling the server in-process, so
// common.AuthStreamInterceptor applies. That is the whole reason these are
// hand-written rather than shortcuts around the service: an in-process call
// would have no claims in context and would serve every submission to anyone.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/common"
)

// uploadChunkBytes is how much of the uploaded file goes into each gRPC
// message. 64 KiB matches what the frontend's SSR handler used and keeps the
// per-message allocation bounded.
const uploadChunkBytes = 64 * 1024

// InstallRoutes registers both handlers on mux. endpoint is the loopback
// address of the service's own gRPC server.
//
// It opens one gRPC client that both handlers share for the life of the
// process, the same way the generated stubs do - the connection is only
// loopback, so there is no TLS to configure and nothing to pool against a
// remote peer.
func InstallRoutes(ctx context.Context, mux *runtime.ServeMux, endpoint string) error {
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dialling %s for the file routes: %w", endpoint, err)
	}
	client := assignmentpb.NewAssignmentServiceClient(conn)

	// The client is closed when the process is shutting down. The context passed
	// to NewGateway's register function outlives the listener setup but not the
	// process, so this is a best-effort cleanup rather than a hard guarantee;
	// the loopback connection dies with the process anyway.
	mux.HandlePath(http.MethodPost, "/api/assignments/{assignment_id}/submissions",
		func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			handleUpload(ctx, client, w, r, pathParams["assignment_id"])
		})

	mux.HandlePath(http.MethodGet, "/api/submissions/{submission_id}/file",
		func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			handleDownload(ctx, client, w, r, pathParams["submission_id"])
		})

	return nil
}

// apiError is the shape every generated gateway error uses, so the frontend can
// read .success and .message without special-casing these two routes.
type apiError struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, statusCode int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(apiError{
		Code:    statusCode,
		Success: false,
		Message: fmt.Sprintf(format, args...),
	})
}

// httpStatusFor maps a gRPC code onto the HTTP status the generated routes
// would have produced, so a failure looks the same whichever route produced it.
func httpStatusFor(err error) int {
	switch status.Code(err) {
	case codes.OK:
		return http.StatusOK
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.InvalidArgument, codes.AlreadyExists, codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.DeadlineExceeded, codes.Unavailable:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// handleUpload accepts a multipart form upload and forwards it to the
// SubmitAssignment client stream in chunks.
//
// The uploader is never read from the form: SubmitAssignment takes the student
// from the verified token in the stream context, so there is no field here that
// could attribute the work to somebody else.
// outgoingContext copies the browser's bearer token into the outgoing gRPC
// metadata.
//
// This is not optional plumbing. The generated routes get it for free:
// common.IncomingHeaderMatcher maps the Authorization header onto the
// `authorization` metadata key and the runtime attaches it to the outgoing RPC.
// A hand-written HandlePath handler makes its own client call, so nothing
// performs that translation - without this, every request to these two routes
// reaches the server with no credentials and common.AuthStreamInterceptor
// rejects it with 401, which looks exactly like a broken token rather than a
// missing one.
//
// The request id is carried for the same reason, so the access log lines for
// these two routes join the same trace as every other request.
func outgoingContext(ctx context.Context, r *http.Request) context.Context {
	pairs := make([]string, 0, 4)

	if auth := r.Header.Get("Authorization"); auth != "" {
		pairs = append(pairs, common.MetadataKey, auth)
	}
	if id := r.Header.Get("X-Request-Id"); id != "" {
		pairs = append(pairs, common.RequestIDMetadataKey, id)
	}
	if len(pairs) == 0 {
		return ctx
	}

	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

func handleUpload(ctx context.Context, client assignmentpb.AssignmentServiceClient, w http.ResponseWriter, r *http.Request, assignmentID string) {
	if assignmentID == "" {
		writeError(w, http.StatusBadRequest, "missing assignment_id")
		return
	}

	ctx = outgoingContext(ctx, r)

	// Cap the body before parsing. MaxBytesReader has to wrap the original
	// body: it needs to hijack the connection to send its own 413 rather than
	// letting a truncated body look like a parse error.
	const maxUploadBytes = 10 << 20 // 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	file, header, err := r.FormFile("submission_file")
	if err != nil {
		// A body over the cap surfaces here as a *http.MaxBytesError. Reporting
		// that as 400 would blame the user's file rather than its size.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "submission exceeds the %d byte limit", maxUploadBytes)
			return
		}
		writeError(w, http.StatusBadRequest, "missing or unreadable submission_file")
		return
	}
	defer file.Close()

	filename := header.Filename
	if filename == "" {
		filename = "submission"
	}

	stream, err := client.SubmitAssignment(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not open the submit stream", "assignment_id", assignmentID, "err", err)
		writeError(w, http.StatusBadGateway, "could not start upload")
		return
	}

	// The first message has to be metadata: the server reads it before touching
	// the pipe, and a chunk-first stream is rejected as InvalidArgument.
	if err := stream.Send(&assignmentpb.SubmitAssignmentRequest{
		Payload: &assignmentpb.SubmitAssignmentRequest_Metadata{
			Metadata: &assignmentpb.SubmissionMetadata{
				AssignmentId: assignmentID,
				Filename:     filename,
				Size:         header.Size,
			},
		},
	}); err != nil {
		// A rejected stream means the token or the assignment was refused.
		// CloseAndRecv surfaces the real status, which is worth more to the
		// browser than a generic 502.
		if _, rerr := stream.CloseAndRecv(); rerr != nil {
			writeError(w, httpStatusFor(rerr), "%s", status.Convert(rerr).Message())
			return
		}
		slog.ErrorContext(ctx, "could not send submission metadata", "assignment_id", assignmentID, "err", err)
		writeError(w, http.StatusBadGateway, "could not upload file")
		return
	}

	buf := make([]byte, uploadChunkBytes)
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if err := stream.Send(&assignmentpb.SubmitAssignmentRequest{
				Payload: &assignmentpb.SubmitAssignmentRequest_Chunk{Chunk: chunk},
			}); err != nil {
				slog.ErrorContext(ctx, "could not send chunk", "assignment_id", assignmentID, "err", err)
				writeError(w, http.StatusBadGateway, "could not upload file")
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			slog.ErrorContext(ctx, "could not read the uploaded file", "assignment_id", assignmentID, "err", readErr)
			writeError(w, http.StatusBadRequest, "could not read the uploaded file")
			return
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		writeError(w, httpStatusFor(err), "%s", status.Convert(err).Message())
		return
	}

	sub := resp.GetSubmission()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":  true,
		"message":  "submission uploaded",
		"id":       sub.GetId(),
		"filename": sub.GetFilename(),
		"size":     sub.GetSize(),
	})
}

// handleDownload streams a submission's file out of DownloadSubmission.
//
// The stream's first frame is metadata rather than bytes, so the response
// headers can only be written once it has arrived. Nothing may be written to w
// before that point - a stray WriteHeader would commit a 200 with no
// Content-Type, and the frontend keys its error handling off that header.
func handleDownload(ctx context.Context, client assignmentpb.AssignmentServiceClient, w http.ResponseWriter, r *http.Request, submissionID string) {
	if submissionID == "" {
		writeError(w, http.StatusBadRequest, "missing submission_id")
		return
	}

	ctx = outgoingContext(ctx, r)

	stream, err := client.DownloadSubmission(ctx, &assignmentpb.DownloadSubmissionRequest{
		SubmissionId: submissionID,
	})
	if err != nil {
		// A failure before the first message still arrives as a gRPC error here,
		// which is the common case: 404 for an unknown or unowned submission,
		// 401 for a missing token.
		writeError(w, httpStatusFor(err), "%s", status.Convert(err).Message())
		return
	}

	first, err := stream.Recv()
	if err != nil {
		writeError(w, httpStatusFor(err), "%s", streamErrorMessage(err))
		return
	}

	meta := first.GetMetadata()
	if meta == nil {
		// The server always sends metadata first. Getting a chunk means the
		// stream is not the shape this handler expects, so fail loudly rather
		// than emitting a file with no name.
		writeError(w, http.StatusInternalServerError, "download stream did not start with metadata")
		return
	}

	filename := meta.GetFilename()
	if filename == "" {
		filename = "submission"
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepathExt(filename)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Content-Disposition has to carry the name through; sanitize it first
	// because it lands in a response header the browser parses.
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeFilename(filename)))
	if size := meta.GetSize(); size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	// The upload and download both cross the 10 MB limit's neighbourhood; a
	// reverse proxy with a small default buffer would otherwise truncate here.
	w.Header().Set("Accept-Ranges", "none")

	for {
		msg, rerr := stream.Recv()
		if rerr == io.EOF {
			return
		}
		if rerr != nil {
			// Headers are already committed, so the status cannot be changed -
			// the transfer is simply cut short and the browser sees a truncated
			// file. Nothing better is available once bytes are on the wire.
			slog.WarnContext(ctx, "download stream ended early", "submission_id", submissionID, "err", rerr)
			return
		}

		chunk := msg.GetChunk()
		if len(chunk) == 0 {
			continue
		}
		if _, werr := w.Write(chunk); werr != nil {
			// The browser hung up. Cancelling the RPC stops the server reading
			// the rest of the file out of storage for nobody.
			slog.InfoContext(ctx, "download client disconnected", "submission_id", submissionID)
			return
		}
	}
}

// streamErrorMessage prefers the gRPC status message and falls back to the
// transport error text, so a connection failure is not reported as an empty
// message to the browser.
func streamErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if msg := status.Convert(err).Message(); msg != "" {
		return msg
	}
	return err.Error()
}

// filepathExt returns the extension including the dot, using the last separator
// that is actually a path separator.
func filepathExt(name string) string {
	if i := strings.LastIndexAny(name, "./\\"); i >= 0 {
		return name[i:]
	}
	return ""
}

// sanitizeFilename strips anything that could break out of a quoted
// Content-Disposition value or a header line: path separators, quotes, CR and
// LF. The name is attacker-controlled - it is whatever the uploader called the
// file, or a header the client supplied.
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '"', '\r', '\n', 0:
			return -1
		}
		return r
	}, name)

	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "submission"
	}
	return name
}
