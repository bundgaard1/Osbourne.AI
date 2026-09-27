package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
)

func TestIncomingHeaderMatcher(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantKey string
		wantOK  bool
	}{
		// The three headers the gateway makes explicit decisions about.
		{"authorization is forwarded unprefixed", "Authorization", MetadataKey, true},
		{"authorization is matched case-insensitively", "authorization", MetadataKey, true},
		{"request id gets the metadata key the interceptor reads", "X-Request-Id", RequestIDMetadataKey, true},
		{"request id is matched case-insensitively", "x-request-id", RequestIDMetadataKey, true},
		{"cookie is denied", "Cookie", "", false},

		// Anything else falls through to the runtime default, which prefixes
		// permanent HTTP headers and unwraps Grpc-Metadata- ones. The point of
		// these cases is only that we do not swallow them.
		{"other permanent headers still fall through", "Content-Type", "grpcgateway-Content-Type", true},
		{"Grpc-Metadata- headers still fall through, unwrapped", "Grpc-Metadata-Foo", "Foo", true},
		{"unrelated headers are still dropped", "X-Whatever", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotOK := IncomingHeaderMatcher(tc.key)
			if gotOK != tc.wantOK || gotKey != tc.wantKey {
				t.Errorf("IncomingHeaderMatcher(%q) = (%q, %v), want (%q, %v)",
					tc.key, gotKey, gotOK, tc.wantKey, tc.wantOK)
			}
		})
	}
}

// TestIncomingHeaderMatcherXRequestIDIsLoadBearing documents why the
// X-Request-Id case cannot be dropped as redundant.
//
// It is not one of the runtime's permanent IANA headers, so the default matcher
// discards it entirely. Without the explicit mapping the correlation id a client
// sends would never reach AuthInterceptor, and WithRequestID would never fire.
func TestIncomingHeaderMatcherXRequestIDIsLoadBearing(t *testing.T) {
	if _, ok := runtime.DefaultHeaderMatcher(HeaderRequestID); ok {
		t.Fatalf("the runtime default now forwards %q; the explicit mapping in "+
			"IncomingHeaderMatcher may be removable", HeaderRequestID)
	}

	gotKey, ok := IncomingHeaderMatcher(HeaderRequestID)
	if !ok || gotKey != RequestIDMetadataKey {
		t.Errorf("IncomingHeaderMatcher(%q) = (%q, %v), want (%q, true)",
			HeaderRequestID, gotKey, ok, RequestIDMetadataKey)
	}
}

// TestIncomingHeaderMatcherDoesNotLeakCookie is the security-relevant one: the
// session cookie must never become gRPC metadata, where it would be visible to
// interceptors, loggers and any tracing exporter.
func TestIncomingHeaderMatcherDoesNotLeakCookie(t *testing.T) {
	for _, key := range []string{"Cookie", "cookie", "COOKIE"} {
		if gotKey, ok := IncomingHeaderMatcher(key); ok {
			t.Errorf("IncomingHeaderMatcher(%q) forwarded the cookie as %q", key, gotKey)
		}
	}
}

func TestOutgoingHeaderMatcher(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantKey string
		wantOK  bool
	}{
		// metadata.Pairs lowercases keys, so this lower-case form is the one a
		// service will actually use.
		{"set-cookie from metadata.Pairs is emitted canonically", "set-cookie", HeaderSetCookie, true},
		{"canonical spelling is accepted too", "Set-Cookie", HeaderSetCookie, true},
		{"internal metadata is not leaked to the browser", "x-internal", "", false},
		{"unrelated metadata is not leaked", "some-debug-key", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotOK := OutgoingHeaderMatcher(tc.key)
			if gotOK != tc.wantOK || gotKey != tc.wantKey {
				t.Errorf("OutgoingHeaderMatcher(%q) = (%q, %v), want (%q, %v)",
					tc.key, gotKey, gotOK, tc.wantKey, tc.wantOK)
			}
		})
	}
}

func TestGatewayErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"not found maps to 404", grpcstatus.Error(codes.NotFound, "course not found"), http.StatusNotFound, "course not found"},
		{"invalid argument maps to 400", grpcstatus.Error(codes.InvalidArgument, "email is required"), http.StatusBadRequest, "email is required"},
		{"unauthenticated maps to 401", RequiresAuthentication(), http.StatusUnauthorized, "missing or invalid bearer token"},
		{"permission denied maps to 403", grpcstatus.Error(codes.PermissionDenied, "not your submission"), http.StatusForbidden, "not your submission"},
		{"internal maps to 500", grpcstatus.Error(codes.Internal, "boom"), http.StatusInternalServerError, "boom"},
		{"unavailable maps to 503", grpcstatus.Error(codes.Unavailable, "no upstream"), http.StatusServiceUnavailable, "no upstream"},
		{"a plain error maps to 500", errString("some failure"), http.StatusInternalServerError, "some failure"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)

			GatewayErrorHandler(context.Background(), GatewayMux(), GatewayMarshaler, rec, req, tc.err)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}

			var body gatewayErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
			}

			// The frontend branches on success and code, so both are load-bearing.
			if body.Success {
				t.Error("success = true on an error response")
			}
			// code must be the HTTP status, not the gRPC code.
			if body.Code != tc.wantStatus {
				t.Errorf("body code = %d, want the HTTP status %d", body.Code, tc.wantStatus)
			}
			if body.Message != tc.wantMsg {
				t.Errorf("body message = %q, want %q", body.Message, tc.wantMsg)
			}
		})
	}
}

// TestGatewayErrorHandlerForcedStatus covers a handler wrapping its error to
// override the gRPC-code-to-status mapping.
func TestGatewayErrorHandlerForcedStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/courses", nil)

	wrapped := &runtime.HTTPStatusError{HTTPStatus: http.StatusServiceUnavailable, Err: grpcstatus.Error(codes.Unavailable, "catalog down")}
	GatewayErrorHandler(context.Background(), GatewayMux(), GatewayMarshaler, rec, req, wrapped)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want the forced %d", rec.Code, http.StatusServiceUnavailable)
	}
}

// TestGatewayErrorHandlerEmptyMessage checks we never emit an empty message,
// which the frontend would render as a blank toast.
func TestGatewayErrorHandlerEmptyMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)

	GatewayErrorHandler(context.Background(), GatewayMux(), GatewayMarshaler, rec, req, grpcstatus.Error(codes.NotFound, ""))

	var body gatewayErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body.Message == "" {
		t.Error("message is empty, want the http.StatusText fallback")
	}
	if body.Message != http.StatusText(http.StatusNotFound) {
		t.Errorf("message = %q, want %q", body.Message, http.StatusText(http.StatusNotFound))
	}
}

func TestUserIDFromContextOrRequest(t *testing.T) {
	const secret = "test-secret"
	token, err := SignJWT(secret, "user-42", "student@osbourne.test", "student", time.Hour)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}
	claims, err := ParseJWT(secret, token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	authed := WithClaims(context.Background(), claims)

	t.Run("falls back to the JWT subject when nothing is requested", func(t *testing.T) {
		got, err := UserIDFromContextOrRequest(authed, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "user-42" {
			t.Errorf("got %q, want user-42", got)
		}
	})

	t.Run("an explicit request wins, for trusted internal callers", func(t *testing.T) {
		got, err := UserIDFromContextOrRequest(authed, "internal-user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "internal-user" {
			t.Errorf("got %q, want internal-user", got)
		}
	})

	t.Run("an explicit request works without claims at all", func(t *testing.T) {
		got, err := UserIDFromContextOrRequest(context.Background(), "internal-user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "internal-user" {
			t.Errorf("got %q, want internal-user", got)
		}
	})

	// Resolving to "" here would let a caller act on a missing identity, so it
	// has to be an error.
	t.Run("neither requested nor authenticated is rejected", func(t *testing.T) {
		got, err := UserIDFromContextOrRequest(context.Background(), "")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if got != "" {
			t.Errorf("got %q, want an empty id alongside the error", got)
		}
		if grpcstatus.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", grpcstatus.Code(err))
		}
	})

	t.Run("claims without a UserID fall back to the subject", func(t *testing.T) {
		subjectOnly := &Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "legacy-user"}}
		got, err := UserIDFromContextOrRequest(WithClaims(context.Background(), subjectOnly), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "legacy-user" {
			t.Errorf("got %q, want legacy-user", got)
		}
	})

	t.Run("claims with neither id nor subject are rejected", func(t *testing.T) {
		got, err := UserIDFromContextOrRequest(WithClaims(context.Background(), &Claims{}), "")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if got != "" {
			t.Errorf("got %q, want an empty id alongside the error", got)
		}
	})
}

func TestAuthStreamInterceptor(t *testing.T) {
	const secret = "test-secret"
	token, err := SignJWT(secret, "user-42", "student@osbourne.test", "student", time.Hour)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}

	t.Run("injects claims and the request id into the handler context", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			MetadataKey, BearerPrefix+token,
			RequestIDMetadataKey, "req-1",
		))

		var sawClaims bool
		var sawRequestID string
		handler := func(srv interface{}, stream grpc.ServerStream) error {
			claims, ok := ClaimsFromContext(stream.Context())
			sawClaims = ok && claims.UserID == "user-42"
			sawRequestID = RequestIDFromContext(stream.Context())
			return nil
		}

		if err := AuthStreamInterceptor(secret)(nil, &fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{}, handler); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sawClaims {
			t.Error("handler did not see the authenticated claims")
		}
		if sawRequestID != "req-1" {
			t.Errorf("request id = %q, want req-1", sawRequestID)
		}
	})

	// Without this interceptor the two streaming RPCs are the only unauthenticated
	// entry points in the system.
	t.Run("rejects a stream with no metadata", func(t *testing.T) {
		called := false
		handler := func(srv interface{}, stream grpc.ServerStream) error {
			called = true
			return nil
		}

		err := AuthStreamInterceptor(secret)(nil, &fakeServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{}, handler)
		if grpcstatus.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", grpcstatus.Code(err))
		}
		if called {
			t.Error("handler ran despite missing credentials")
		}
	})

	t.Run("rejects a stream with no authorization header", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("some-other-header", "value"))
		err := AuthStreamInterceptor(secret)(nil, &fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{},
			func(srv interface{}, stream grpc.ServerStream) error { return nil })
		if grpcstatus.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", grpcstatus.Code(err))
		}
	})

	t.Run("rejects a stream with a bogus token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, BearerPrefix+"not-a-jwt"))
		err := AuthStreamInterceptor(secret)(nil, &fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{},
			func(srv interface{}, stream grpc.ServerStream) error { return nil })
		if grpcstatus.Code(err) != codes.Unauthenticated {
			t.Errorf("code = %v, want Unauthenticated", grpcstatus.Code(err))
		}
	})
}

// errString is a minimal non-status error, standing in for the plain errors a
// handler can return.
type errString string

func (e errString) Error() string { return string(e) }

// fakeServerStream is a grpc.ServerStream that only carries a context, which is
// all AuthStreamInterceptor touches.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *fakeServerStream) Context() context.Context { return s.ctx }

// TestGatewayMarshalerUsesProtoNames pins UseProtoNames.
//
// errdetails.BadRequest.field_violations is a good probe: protojson's default
// emits "fieldViolations", while UseProtoNames emits "field_violations". Every
// REST body has to keep the proto spelling, because that is what the
// frontend's result.data reads already use - flipping this flag silently
// renames every multi-word field in every response.
func TestGatewayMarshalerUsesProtoNames(t *testing.T) {
	msg := &errdetails.BadRequest{
		FieldViolations: []*errdetails.BadRequest_FieldViolation{
			{Field: "email", Description: "required"},
		},
	}

	got, err := GatewayMarshaler.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("output %s is not JSON: %v", got, err)
	}

	if _, ok := decoded["field_violations"]; !ok {
		t.Errorf("output %s has no field_violations key; UseProtoNames looks disabled", got)
	}
	if _, ok := decoded["fieldViolations"]; ok {
		t.Errorf("output %s used lowerCamelCase; UseProtoNames looks disabled", got)
	}
}

// TestGatewayMarshalerDiscardsUnknownFields keeps an older client working
// against a service that has gained fields.
func TestGatewayMarshalerDiscardsUnknownFields(t *testing.T) {
	var msg errdetails.BadRequest
	if err := GatewayMarshaler.Unmarshal([]byte(`{"field_violations":[],"something_new":true}`), &msg); err != nil {
		t.Fatalf("Unmarshal rejected an unknown field: %v", err)
	}
}

// TestGatewayMuxRoutingErrorShape is the end-to-end guard on the mux wiring.
//
// ServeMux does not route an unmatched request to the error handler; it goes to
// the *routing* error handler, a separate hook. If GatewayMux forgets to set
// it, a mistyped URL silently reverts to the runtime's stock body and every
// other error in the system keeps the new shape. Drive a real ServeMux and
// check the body, which is the only way to catch that.
func TestGatewayMuxRoutingErrorShape(t *testing.T) {
	mux := GatewayMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/no-such-thing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	var body gatewayErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body.Success {
		t.Error("success = true on a 404")
	}
	// This is the assertion that matters: the stock routing handler would emit
	// the gRPC code 5 here, not 404.
	if body.Code != http.StatusNotFound {
		t.Errorf("body code = %d, want the HTTP status 404", body.Code)
	}
	if body.Message == "" {
		t.Error("message is empty, want the http.StatusText fallback")
	}
}

// TestGatewayMuxMethodMismatch covers the other routing rejection: a real path
// reached with the wrong verb.
func TestGatewayMuxMethodMismatch(t *testing.T) {
	rec := httptest.NewRecorder()
	mux := GatewayMux()
	if err := mux.HandlePath(http.MethodGet, "/api/profile", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {}); err != nil {
		t.Fatalf("HandlePath: %v", err)
	}
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/profile", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	var body gatewayErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body.Code != http.StatusMethodNotAllowed {
		t.Errorf("body code = %d, want 405", body.Code)
	}
}
