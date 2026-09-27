package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"osbourne.local/auth-service/gen/auth"
	"osbourne.local/auth-service/internal/database"
	"osbourne.local/auth-service/internal/repository"
	"osbourne.local/auth-service/internal/server"
	"osbourne.local/auth-service/internal/service"
	"osbourne.local/common"
)

// The seeded demo accounts, mirrored from internal/database/seed.go. They are
// duplicated rather than exported because the seed table is deliberately
// unexported; a rename there should fail these tests loudly rather than silently
// start testing the wrong account.
const (
	studentEmail = "student@osbourne.local"
	studentPass  = "student123"
)

// newTestGateway serves the auth service over a real gRPC connection and
// returns a runtime.ServeMux with the production gateway policy from
// osbourne.local/common.
//
// It dials rather than registering the server implementation in-process,
// because the thing under test is a header surviving the trip: a Set-Cookie
// only exists if the response metadata made it back through a real RPC.
func newTestGateway(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()

	db, err := database.NewGORMDB(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	database.SeedData(db)

	authSvc := service.NewAuthService(repository.NewGORMAccountRepository(db), service.Config{
		JWTSecret: "test-secret",
		TokenTTL:  time.Hour,
	})

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(common.RequestLoggerInterceptor()),
	)
	auth.RegisterAuthServiceServer(grpcServer, server.NewAuthServer(authSvc, time.Hour))

	bufnet := bufconn.Listen(1024 * 1024)
	go func() { _ = grpcServer.Serve(bufnet) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		bufnet.Close()
	})

	// The real common.GatewayMux, not a local copy: the cookie assertions below
	// are only meaningful against the production header matcher.
	//
	// The target needs an explicit scheme. RegisterXHandlerFromEndpoint builds
	// its connection with grpc.NewClient, whose default resolver is DNS, so a
	// bare "bufnet" would try to resolve it as a hostname.
	const bufconnTarget = "passthrough:///bufnet"

	mux := common.GatewayMux(runtime.WithForwardResponseOption(redactLoginToken))
	if err := auth.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, bufconnTarget, []grpc.DialOption{
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return bufnet.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}); err != nil {
		t.Fatalf("register gateway: %v", err)
	}

	return mux
}

// redactLoginToken mirrors the option auth-service's main.go installs, so the
// "token is not in the body" assertion exercises the real behaviour.
func redactLoginToken(_ context.Context, _ http.ResponseWriter, msg proto.Message) error {
	if resp, ok := msg.(*auth.LoginResponse); ok {
		resp.Token = ""
	}
	return nil
}

func postJSON(t *testing.T, path string, payload any) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "osbourne_session" {
			return c
		}
	}
	t.Fatalf("no osbourne_session cookie set; got %v", rec.Result().Cookies())
	return nil
}

func TestLoginSetsHttpOnlySessionCookie(t *testing.T) {
	mux := newTestGateway(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, postJSON(t, "/api/auth/login", map[string]string{
		"email": studentEmail, "password": studentPass,
	}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	cookie := sessionCookie(t, rec)

	// Each attribute is load-bearing, so assert them one by one rather than
	// comparing the serialised string.
	if cookie.Value == "" {
		t.Error("cookie value is empty")
	}
	if !cookie.HttpOnly {
		t.Error("cookie is not HttpOnly, so any XSS can read the token")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q, want /", cookie.Path)
	}
	// Without Max-Age the cookie would outlive its own token and the user would
	// see a login loop rather than a clean redirect to /login.
	if cookie.MaxAge <= 0 {
		t.Errorf("Max-Age = %d, want a positive lifetime", cookie.MaxAge)
	}
}

func TestLoginKeepsTokenOutOfTheResponseBody(t *testing.T) {
	mux := newTestGateway(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, postJSON(t, "/api/auth/login", map[string]string{
		"email": studentEmail, "password": studentPass,
	}))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}

	// The body is readable by script, so a token here would make HttpOnly
	// meaningless: the whole reason for the cookie is that JS cannot reach it.
	if tok, ok := body["token"]; ok && tok != "" {
		t.Errorf("body carries the token %q; it should arrive only as an HttpOnly cookie", tok)
	}

	// The rest of the response is still what the client needs.
	if body["user_id"] == nil || body["user_id"] == "" {
		t.Error("body has no user_id")
	}
	if body["email"] != studentEmail {
		t.Errorf("email = %v, want %q", body["email"], studentEmail)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	mux := newTestGateway(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, postJSON(t, "/api/auth/login", map[string]string{
		"email": studentEmail, "password": "wrong-password",
	}))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}

	// A rejected login must not leave anything usable behind.
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("failed login set cookies: %v", cookies)
	}

	// The error body has to match the shape the frontend reads.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body["success"] != false {
		t.Errorf("success = %v, want false", body["success"])
	}
	if code, ok := body["code"].(float64); !ok || int(code) != http.StatusUnauthorized {
		t.Errorf("code = %v, want the HTTP status 401", body["code"])
	}
}

func TestLogoutExpiresTheSessionCookieWithoutAToken(t *testing.T) {
	mux := newTestGateway(t)
	rec := httptest.NewRecorder()
	// No Authorization header: a user whose token has already expired still has
	// the cookie in their browser and must be able to drop it.
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	cookie := sessionCookie(t, rec)

	// A negative MaxAge makes (*http.Cookie).String() emit `Max-Age=0`, which is
	// how a browser is told to discard the cookie.
	if cookie.MaxAge >= 0 {
		t.Errorf("Max-Age = %d, want a negative value so the browser discards the cookie", cookie.MaxAge)
	}
	if cookie.Value != "" {
		t.Errorf("value = %q, want empty", cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Error("expiring cookie should stay HttpOnly")
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q, want / so the clear matches the set", cookie.Path)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body["success"] != true {
		t.Errorf("success = %v, want true", body["success"])
	}
}

func TestLoginLogoutRoundTripIsIdempotent(t *testing.T) {
	mux := newTestGateway(t)

	// Logging out twice has to be harmless: a double-clicked button, or a retry
	// after a flaky response, should not produce an error page.
	for i := range 2 {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("logout %d: status = %d, want 200; body %s", i+1, rec.Code, rec.Body)
		}
	}
}

func TestValidateTokenOverREST(t *testing.T) {
	mux := newTestGateway(t)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, postJSON(t, "/api/auth/validate", map[string]string{
		"token": "not-a-real-jwt",
	}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}

	// A bad token is a valid answer, not an error: the endpoint introspects, so
	// the client is asking "is this good?", not "give me a session". A 4xx here
	// would conflate the two.
	if body["valid"] == true {
		t.Error("valid = true for a bogus token")
	}

	// Pin the omission behaviour rather than leaving it to chance. The gateway
	// marshaler leaves EmitUnpopulated off, so valid=false is dropped from the
	// body rather than serialised. That is only safe because callers test
	// truthiness; see common.GatewayMarshaler. If this ever asserts equality,
	// the config needs revisiting.
	if _, present := body["valid"]; present {
		t.Errorf("valid was serialised as %v; expected it to be omitted, "+
			"so a change here means the marshaler config moved", body["valid"])
	}
}

func TestUnknownAuthPathReturnsJSON404(t *testing.T) {
	mux := newTestGateway(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body["success"] != false {
		t.Errorf("success = %v, want false", body["success"])
	}
	if code, ok := body["code"].(float64); !ok || int(code) != http.StatusNotFound {
		t.Errorf("code = %v, want the HTTP status 404, not a gRPC code", body["code"])
	}
}
