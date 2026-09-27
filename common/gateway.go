package common

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/textproto"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// HTTP header names this gateway cares about.
//
// Canonical MIME casing matters: the runtime calls the matchers with keys it
// has already run through textproto.CanonicalMIMEHeaderKey. These matchers are
// exported and are also called directly by the hand-written HandlePath
// handlers, so they canonicalise the key themselves instead of trusting the
// caller to have done it.
const (
	// HeaderAuthorization carries the bearer token. Supplied by the browser
	// directly, or injected by Nginx from the osbourne_session cookie once the
	// gateway is in front.
	HeaderAuthorization = "Authorization"
	// HeaderSetCookie lets a service set a cookie on the response.
	HeaderSetCookie = "Set-Cookie"
	// HeaderRequestID is the client-supplied correlation id.
	HeaderRequestID = "X-Request-Id"
	// HeaderCookie is deliberately never forwarded into gRPC metadata.
	HeaderCookie = "Cookie"
)

// GatewayMarshaler is the JSON codec used for every REST response.
//
// UseProtoNames keeps wire field names identical to the proto ones (user_id,
// isRead, ...). Without it protojson emits lowerCamelCase, which would
// silently break every existing result.data.user_id style read in the
// frontend. DiscardUnknown lets an older client keep working against a
// service that has since gained fields.
//
// EmitUnpopulated is deliberately left false, so a proto3 field holding its
// zero value is omitted from the response entirely: ValidateTokenResponse
// with valid=false serialises to `{}`, not `{"valid": false}`. That is safe
// only because every consumer so far tests truthiness (`if (data.success)`)
// rather than comparing strictly (`data.success === false`), where an absent
// field and an explicit false are indistinguishable. Turning this flag on
// would also start emitting nulls for every unset message and slice.
var GatewayMarshaler = &runtime.JSONPb{
	MarshalOptions: protojson.MarshalOptions{
		UseProtoNames: true,
	},
	UnmarshalOptions: protojson.UnmarshalOptions{
		DiscardUnknown: true,
	},
}

// GatewayMux builds the ServeMux that each service's REST listener is mounted
// on. The six main.go files then differ only in which
// Register<Service>HandlerFromEndpoint calls they make, so all the policy -
// header handling, error shape, JSON codec - lives here.
//
// extra options are applied last, so a service can override the defaults above.
// auth-service uses this to blank the token out of its Login response body.
func GatewayMux(extra ...runtime.ServeMuxOption) *runtime.ServeMux {
	opts := []runtime.ServeMuxOption{
		runtime.WithMarshalerOption(runtime.MIMEWildcard, GatewayMarshaler),
		runtime.WithIncomingHeaderMatcher(IncomingHeaderMatcher),
		runtime.WithOutgoingHeaderMatcher(OutgoingHeaderMatcher),
		runtime.WithErrorHandler(GatewayErrorHandler),
		runtime.WithRoutingErrorHandler(GatewayRoutingErrorHandler),
	}
	return runtime.NewServeMux(append(opts, extra...)...)
}

// IncomingHeaderMatcher decides which HTTP request headers become gRPC
// metadata.
func IncomingHeaderMatcher(key string) (string, bool) {
	switch textproto.CanonicalMIMEHeaderKey(key) {
	case HeaderAuthorization:
		// Mapped explicitly even though it is currently redundant: the runtime's
		// annotateContext already appends `authorization` for this header before
		// consulting any matcher, for backwards compatibility. The value is
		// identical either way and AuthInterceptor reads values[0], so the
		// duplicate is inert - but it means the auth path survives a runtime
		// release that drops that carve-out, instead of failing closed.
		return MetadataKey, true
	case HeaderRequestID:
		// No special case exists for this one anywhere in the runtime. Without
		// the mapping it would arrive as grpcgateway-X-Request-Id, and
		// AuthInterceptor would never see the id the client sent.
		return RequestIDMetadataKey, true
	case HeaderCookie:
		// Deny rather than fall through. The default matcher would forward this
		// as grpcgateway-Cookie, putting the raw session cookie into gRPC
		// metadata where it can be logged or traced. Nginx lifts the session
		// into the Authorization header, so the cookie is redundant from here on.
		return "", false
	}
	return runtime.DefaultHeaderMatcher(key)
}

// OutgoingHeaderMatcher decides which gRPC response metadata become HTTP
// response headers. It is an allowlist of exactly one header.
//
// That is deliberately stricter than the runtime default, which prefixes
// everything with Grpc-Metadata- and forwards it all. Internal metadata has no
// business reaching the browser, so anything not named here is dropped.
func OutgoingHeaderMatcher(key string) (string, bool) {
	if textproto.CanonicalMIMEHeaderKey(key) == HeaderSetCookie {
		// Emitted unprefixed, so a service can opt in from inside an RPC with:
		//
		//	grpc.SetHeader(ctx, metadata.Pairs("set-cookie", "osbourne_session=...; HttpOnly"))
		//
		// metadata.Pairs lowercases its keys, so the matcher has to accept
		// "set-cookie" and hand back the canonical "Set-Cookie" spelling.
		return HeaderSetCookie, true
	}
	return "", false
}

// gatewayErrorBody is the single error shape every REST endpoint returns.
type gatewayErrorBody struct {
	// Code is the HTTP status, repeated in the body so callers that only parse
	// the JSON still know what happened. It is the HTTP status and not the gRPC
	// code on purpose: the frontend's existing handlers branch on this field,
	// and the stock gateway handler emits the gRPC code - 13 for every internal
	// error - which a client cannot act on.
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GatewayErrorHandler replaces runtime.DefaultHTTPErrorHandler so every error
// crossing the REST boundary has the same body shape.
//
// It also logs. The gRPC server logs the same failure from its own
// interceptor, but without the HTTP status or the path, which makes a 500 on
// /api/profile hard to trace back to a request.
func GatewayErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	// A handler may wrap an error in *runtime.HTTPStatusError to force a
	// specific status. Unwrap before deciding anything.
	var httpErr *runtime.HTTPStatusError
	if errors.As(err, &httpErr) {
		err = httpErr.Err
	}

	st := grpcstatus.Convert(err)
	httpStatus := runtime.HTTPStatusFromCode(st.Code())
	if httpErr != nil && httpErr.HTTPStatus != 0 {
		httpStatus = httpErr.HTTPStatus
	}

	writeGatewayError(ctx, w, r, gatewayErrorDetail{
		status:   httpStatus,
		message:  st.Message(),
		grpcCode: st.Code().String(),
		cause:    err.Error(),
	})
}

// GatewayRoutingErrorHandler covers requests that match no route at all.
//
// It is a separate hook from GatewayErrorHandler, and leaving it at the runtime
// default would mean a mistyped URL answered 404 with the stock
// `{"code": 5, "message": "Not Found"}` - a different body shape, and a gRPC
// code rather than an HTTP status, for what the client sees as exactly the
// same class of failure.
func GatewayRoutingErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, httpStatus int) {
	writeGatewayError(ctx, w, r, gatewayErrorDetail{
		status:   httpStatus,
		message:  http.StatusText(httpStatus),
		grpcCode: codes.Unknown.String(),
		cause:    "no route matched",
	})
}

// gatewayErrorDetail is the input to writeGatewayError, carrying the two
// possible sources of an error status: a gRPC status, or a routing decision.
type gatewayErrorDetail struct {
	status   int
	message  string
	grpcCode string
	cause    string
}

func writeGatewayError(ctx context.Context, w http.ResponseWriter, r *http.Request, d gatewayErrorDetail) {
	attrs := []any{
		"grpc_code", d.grpcCode,
		"http_status", d.status,
		"method", r.Method,
		"path", r.URL.Path,
		"error", d.cause,
	}
	// The gRPC-side request id is not in ctx yet at this point - the
	// interceptor that populates it runs on the server, downstream of here - so
	// read it straight off the request.
	if rid := r.Header.Get(HeaderRequestID); rid != "" {
		attrs = append(attrs, "request_id", rid)
	}

	// A 4xx is usually the client's fault, a 5xx is ours. Splitting the level
	// keeps alerting on 5xx from drowning in 401s from stale cookies.
	if d.status >= http.StatusInternalServerError {
		slog.ErrorContext(ctx, "rest request failed", attrs...)
	} else {
		slog.WarnContext(ctx, "rest request rejected", attrs...)
	}

	// Mirror the runtime's own trailer hygiene: a Trailer announced before the
	// status was known would otherwise be emitted alongside a body we are not
	// sending.
	w.Header().Del("Trailer")
	w.Header().Del("Transfer-Encoding")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(d.status)

	body := gatewayErrorBody{Code: d.status, Success: false, Message: d.message}
	if body.Message == "" {
		body.Message = http.StatusText(d.status)
	}
	if encErr := json.NewEncoder(w).Encode(body); encErr != nil {
		slog.ErrorContext(ctx, "failed to write gateway error body", "error", encErr)
	}
}

// UserIDFromContextOrRequest resolves whose identity an RPC is acting on.
//
// `requested` is the user_id carried in the request message. Trusted internal
// gRPC callers - notably the frontend's SSR handlers, which already know the
// session's user - pass it explicitly. REST callers do not: they send no
// user_id and the answer comes from the verified JWT instead. A request that
// supplies neither is rejected rather than silently resolved to "".
//
// A requested id is NOT checked against the token. That is the point of the
// trusted internal path, and it is exactly why every RPC reachable from a
// public endpoint has to ignore the field and call UserIDFromContext on its
// own instead - see the comments on EnrollUser, ListMySubmissions and
// UpdateUserProfile in the protos.
func UserIDFromContextOrRequest(ctx context.Context, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}

	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return "", RequiresAuthentication()
	}
	if claims.UserID != "" {
		return claims.UserID, nil
	}
	// Tokens minted before the UserID claim was added carry only the subject.
	if claims.Subject == "" {
		return "", RequiresAuthentication()
	}
	return claims.Subject, nil
}

// AuthStreamInterceptor is the streaming counterpart of AuthInterceptor.
//
// AuthInterceptor is a grpc.UnaryServerInterceptor, so it is never consulted
// for SubmitAssignment (client streaming) or DownloadSubmission (server
// streaming). Without this, those two would be the only RPCs in the system
// callable without a token, and both touch submitted student work.
//
// The error is returned straight from the stream handler, so the runtime's
// stream error handler renders it with the same shape as GatewayErrorHandler.
func AuthStreamInterceptor(secret string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return RequiresAuthentication()
		}

		values := md.Get(MetadataKey)
		if len(values) == 0 {
			return RequiresAuthentication()
		}

		claims, err := ParseJWT(secret, trimBearer(values[0]))
		if err != nil {
			return RequiresAuthentication()
		}

		ctx := WithClaims(ss.Context(), claims)
		if rids := md.Get(RequestIDMetadataKey); len(rids) > 0 {
			ctx = WithRequestID(ctx, rids[0])
		}

		return handler(srv, &contextServerStream{ServerStream: ss, ctx: ctx})
	}
}

// contextServerStream overrides Context() so the handler sees the enriched
// context. gRPC offers no way to replace a stream's context in place, which is
// what this wrapper is for.
type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context { return s.ctx }
