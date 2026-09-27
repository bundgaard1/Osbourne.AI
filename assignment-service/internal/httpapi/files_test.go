package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	assignmentpb "osbourne.local/assignment-service/gen/assignment"
	"osbourne.local/assignment-service/internal/database"
	"osbourne.local/assignment-service/internal/httpapi"
	"osbourne.local/assignment-service/internal/repository"
	"osbourne.local/assignment-service/internal/server"
	"osbourne.local/assignment-service/internal/service"
	"osbourne.local/common"
)

const testJWTSecret = "test-secret"

// newTestGateway mounts the generated JSON routes and the two hand-written file
// routes on a real gRPC server behind a real HTTP server.
//
// It uses a loopback TCP listener rather than bufconn because
// httpapi.InstallRoutes takes an endpoint address and builds its own client from
// it - the same call main.go makes. Testing the production signature means the
// test covers that dialing, rather than a parallel wiring the test invented.
//
// common.AuthStreamInterceptor is installed exactly as main.go does it, because
// the point of these routes is that they go *through* gRPC: with the
// interceptor there are claims in the stream context, and without it
// SubmitAssignment refuses and DownloadSubmission is an open door.
func newTestGateway(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()

	db, err := database.NewGORMDB(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}

	storage, err := repository.NewLocalFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("init file storage: %v", err)
	}

	svc := service.NewAssignmentService(
		repository.NewGORMAssignmentRepository(db),
		repository.NewGORMSubmissionRepository(db),
		storage,
		nil,
	)

	grpcServer := grpc.NewServer(
		grpc.ChainStreamInterceptor(common.AuthStreamInterceptor(testJWTSecret)),
	)
	assignmentpb.RegisterAssignmentServiceServer(grpcServer, server.NewAssignmentServer(svc))

	// Port 0 lets the kernel pick, so parallel tests cannot collide.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)

	endpoint := lis.Addr().String()

	mux := common.GatewayMux()
	if err := assignmentpb.RegisterAssignmentServiceHandlerFromEndpoint(
		ctx, mux, endpoint,
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	); err != nil {
		t.Fatalf("register generated gateway: %v", err)
	}

	if err := httpapi.InstallRoutes(ctx, mux, endpoint); err != nil {
		t.Fatalf("install file routes: %v", err)
	}

	return mux
}

func tokenFor(t *testing.T, userID string) string {
	t.Helper()
	token, err := common.SignJWT(testJWTSecret, userID, userID+"@osbourne.local", "student", time.Hour)
	if err != nil {
		t.Fatalf("sign jwt for %s: %v", userID, err)
	}
	return token
}

// seedAssignment inserts an assignment directly, so the upload route has
// something real to validate against (the server rejects a submission for an
// assignment that does not exist).
func seedAssignment(t *testing.T, mux http.Handler, courseID, title string) string {
	t.Helper()
	rec := postJSON(t, mux, "/api/courses/"+courseID+"/assignments", tokenFor(t, "teacher-1"),
		map[string]string{"title": title, "description": "d"})
	if rec.Code != http.StatusOK {
		t.Fatalf("seed assignment: status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var body struct {
		Assignment struct {
			ID string `json:"id"`
		} `json:"assignment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("seed body %q: %v", rec.Body.String(), err)
	}
	if body.Assignment.ID == "" {
		t.Fatalf("seed returned no assignment id; body %s", rec.Body.String())
	}
	return body.Assignment.ID
}

func postJSON(t *testing.T, mux http.Handler, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// multipartUpload builds the form body the upload route expects, using the
// `submission_file` field name the frontend and the plan both specify.
func multipartUpload(t *testing.T, filename string, content []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("submission_file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

func postUpload(t *testing.T, mux http.Handler, path, token string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func jsonBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	return body
}

func TestUploadRoundTripsThroughTheStream(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	// Bigger than the 64 KiB chunk size, so the handler really loops.
	content := bytes.Repeat([]byte("osbourne"), 40_000)

	body, contentType := multipartUpload(t, "essay.txt", content)
	rec := postUpload(t, mux,
		"/api/assignments/"+assignmentID+"/submissions", tokenFor(t, "student-1"), body, contentType)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	got := jsonBody(t, rec)
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	if got["filename"] != "essay.txt" {
		t.Errorf("filename = %v, want essay.txt", got["filename"])
	}
}

// The stream interceptor is what makes this route authenticated at all. Without
// it, SubmitAssignment had no claims to read the uploader from.
func TestUploadWithoutATokenIs401(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	body, contentType := multipartUpload(t, "essay.txt", []byte("data"))
	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions", "", body, contentType)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
	if got := jsonBody(t, rec); got["success"] != false {
		t.Errorf("success = %v, want false", got["success"])
	}
}

func TestUploadWithoutAFileIs400(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	// A well-formed multipart body with no submission_file field.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("not_the_file", "x"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions",
		tokenFor(t, "student-1"), &buf, mw.FormDataContentType())

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", rec.Code, rec.Body)
	}
}

func TestUploadAboveTheSizeLimitIs413(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	// The route caps at 10 MB. Sending more should be reported as too large
	// rather than as a generic parse failure, which would blame the file.
	body, contentType := multipartUpload(t, "big.bin", bytes.Repeat([]byte("x"), (10<<20)+4096))
	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions",
		tokenFor(t, "student-1"), body, contentType)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body %s", rec.Code, rec.Body)
	}
}

func TestUploadForAnUnknownAssignmentIs404(t *testing.T) {
	mux := newTestGateway(t)

	body, contentType := multipartUpload(t, "essay.txt", []byte("data"))
	rec := postUpload(t, mux, "/api/assignments/does-not-exist/submissions",
		tokenFor(t, "student-1"), body, contentType)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body)
	}
}

// The full round trip: upload a file, then download it and require the bytes to
// come back identical. Anything that mangles the chunking shows up here.
func TestDownloadReturnsWhatWasUploaded(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	content := bytes.Repeat([]byte("abcdefgh"), 20_000)
	body, contentType := multipartUpload(t, "essay.txt", content)
	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions",
		tokenFor(t, "student-1"), body, contentType)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: status = %d; body %s", rec.Code, rec.Body)
	}
	submissionID, _ := jsonBody(t, rec)["id"].(string)
	if submissionID == "" {
		t.Fatalf("upload returned no submission id; body %s", rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/submissions/"+submissionID+"/file", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "student-1"))
	dl := httptest.NewRecorder()
	mux.ServeHTTP(dl, req)

	if dl.Code != http.StatusOK {
		t.Fatalf("download: status = %d, want 200; body %s", dl.Code, dl.Body)
	}

	if got := dl.Body.Bytes(); !bytes.Equal(got, content) {
		t.Errorf("downloaded %d bytes, want the %d uploaded", len(got), len(content))
	}

	// The metadata frame has to reach the headers, or the browser saves the file
	// with no name.
	if cd := dl.Header().Get("Content-Disposition"); cd == "" {
		t.Error("Content-Disposition is missing")
	} else {
		want := `attachment; filename="essay.txt"`
		if cd != want {
			t.Errorf("Content-Disposition = %q, want %q", cd, want)
		}
	}
	if cl := dl.Header().Get("Content-Length"); cl == "" {
		t.Error("Content-Length is missing; the metadata frame carries the size")
	}
}

func TestDownloadWithoutATokenIs401(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	body, contentType := multipartUpload(t, "essay.txt", []byte("data"))
	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions",
		tokenFor(t, "student-1"), body, contentType)
	submissionID, _ := jsonBody(t, rec)["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/submissions/"+submissionID+"/file", nil)
	dl := httptest.NewRecorder()
	mux.ServeHTTP(dl, req)

	if dl.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", dl.Code, dl.Body)
	}
	// The failure has to be JSON, because that is what the frontend branches on.
	if ct := dl.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json on the failure path", ct)
	}
}

func TestDownloadUnknownSubmissionIs404(t *testing.T) {
	mux := newTestGateway(t)

	req := httptest.NewRequest(http.MethodGet, "/api/submissions/does-not-exist/file", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "student-1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body)
	}
}

// A file name arrives from an attacker-controlled multipart header, so it must
// not be able to inject a header of its own.
func TestDownloadSanitizesTheFilename(t *testing.T) {
	mux := newTestGateway(t)
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")

	hostile := "a\"b\r\nX-Injected: yes\r\n\r\n.txt"
	body, contentType := multipartUpload(t, hostile, []byte("data"))
	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions",
		tokenFor(t, "student-1"), body, contentType)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: status = %d; body %s", rec.Code, rec.Body)
	}
	submissionID, _ := jsonBody(t, rec)["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/submissions/"+submissionID+"/file", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "student-1"))
	dl := httptest.NewRecorder()
	mux.ServeHTTP(dl, req)

	if got := dl.Header().Get("X-Injected"); got != "" {
		t.Errorf("X-Injected = %q; the filename escaped into a response header", got)
	}
	if cd := dl.Header().Get("Content-Disposition"); !strings.Contains(cd, "filename=") {
		t.Errorf("Content-Disposition = %q, want a filename", cd)
	}
}

// The generated route for the same collection must not have swallowed the
// custom one: both live under /api/assignments/{id}/submissions.
func TestCustomUploadRouteIsNotShadowedByTheGeneratedOne(t *testing.T) {
	mux := newTestGateway(t)

	// With no assignment seeded, a valid multipart body that reaches the custom
	// handler fails on the assignment lookup (404). If the generated route were
	// serving this path instead, the response would be a JSON decode error or a
	// method-not-allowed, never 404 from the assignment check.
	assignmentID := seedAssignment(t, mux, "c-1", "Essay")
	body, contentType := multipartUpload(t, "essay.txt", []byte("x"))
	rec := postUpload(t, mux, "/api/assignments/"+assignmentID+"/submissions",
		tokenFor(t, "student-1"), body, contentType)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the custom handler; body %s", rec.Code, rec.Body)
	}
	if _, isJSONUpload := jsonBody(t, rec)["filename"]; !isJSONUpload {
		t.Error("response is missing the filename field the custom handler sets")
	}
}

func TestUnroutedAPIPathStillReturnsJSON404(t *testing.T) {
	mux := newTestGateway(t)

	req := httptest.NewRequest(http.MethodGet, "/api/nope", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "student-1"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
