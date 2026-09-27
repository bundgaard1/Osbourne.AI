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

	"github.com/glebarez/sqlite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"osbourne.local/common"
	"osbourne.local/profile-service/gen/profile"
	"osbourne.local/profile-service/internal/domain"
	"osbourne.local/profile-service/internal/repository"
	"osbourne.local/profile-service/internal/server"
	"osbourne.local/profile-service/internal/service"
)

const testJWTSecret = "test-secret"

// newTestGateway serves the profile service over a real gRPC connection behind
// the production gateway policy from osbourne.local/common.
//
// It dials instead of registering the server implementation in-process on
// purpose: the assertion that matters here is that a JWT arrives in the gRPC
// metadata and turns into claims, which only happens if the request really
// crosses an RPC boundary and the real common.AuthInterceptor runs.
//
// The database is returned alongside the handler so tests can assert on rows
// the server wrote, not just on what came back over the wire.
func newTestGateway(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	ctx := context.Background()

	db := setupTestDB(t)
	repo := repository.NewGORMProfileRepository(db)
	profileSvc := service.NewProfileService(repo)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			common.AuthInterceptor(testJWTSecret),
			common.RequestLoggerInterceptor(),
		),
	)
	profile.RegisterProfileServiceServer(grpcServer, server.NewProfileServer(profileSvc))

	bufnet := bufconn.Listen(1024 * 1024)
	go func() { _ = grpcServer.Serve(bufnet) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		bufnet.Close()
	})

	// grpc.NewClient defaults to the DNS resolver, so the target needs an
	// explicit scheme or "bufnet" would be treated as a hostname.
	const bufconnTarget = "passthrough:///bufnet"

	mux := common.GatewayMux()
	if err := profile.RegisterProfileServiceHandlerFromEndpoint(ctx, mux, bufconnTarget, []grpc.DialOption{
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return bufnet.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}); err != nil {
		t.Fatalf("register gateway: %v", err)
	}

	return mux, db
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&domain.UserProfile{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}
	return db
}

func seedProfile(t *testing.T, db *gorm.DB, id, name string) {
	t.Helper()
	if err := db.Create(&domain.UserProfile{ID: id, Name: name, StudyProgram: "CS"}).Error; err != nil {
		t.Fatalf("seed profile %s: %v", id, err)
	}
}

// tokenFor mints a real JWT so the test exercises the same verification path
// production uses, rather than hand-rolling a context value.
func tokenFor(t *testing.T, userID string) string {
	t.Helper()
	token, err := common.SignJWT(testJWTSecret, userID, userID+"@osbourne.local", "student", time.Hour)
	if err != nil {
		t.Fatalf("generate jwt for %s: %v", userID, err)
	}
	return token
}

func doJSON(t *testing.T, mux http.Handler, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeProfile(t *testing.T, rec *httptest.ResponseRecorder) *profile.ProfileResponse {
	t.Helper()
	var got profile.ProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not a ProfileResponse: %v", rec.Body.String(), err)
	}
	return &got
}

// decodeUpdate unwraps UpdateUserProfileResponse, whose profile is nested
// rather than returned bare like GetUserProfile's.
func decodeUpdate(t *testing.T, rec *httptest.ResponseRecorder) *profile.ProfileResponse {
	t.Helper()
	var wrapper profile.UpdateUserProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &wrapper); err != nil {
		t.Fatalf("body %q is not an UpdateUserProfileResponse: %v", rec.Body.String(), err)
	}
	if wrapper.Profile == nil {
		t.Fatalf("body %q has no profile", rec.Body.String())
	}
	return wrapper.Profile
}

func TestGetProfileUsesTheTokenSubject(t *testing.T) {
	mux, db := newTestGateway(t)
	seedProfile(t, db, "student-1", "Ada")

	rec := doJSON(t, mux, http.MethodGet, "/api/profile", tokenFor(t, "student-1"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	got := decodeProfile(t, rec)
	if got.Id != "student-1" {
		t.Errorf("id = %q, want student-1", got.Id)
	}
	if got.Name != "Ada" {
		t.Errorf("name = %q, want Ada", got.Name)
	}
}

func TestGetProfileWithoutTokenIs401(t *testing.T) {
	mux, _ := newTestGateway(t)

	rec := doJSON(t, mux, http.MethodGet, "/api/profile", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}

	// The error body has to match the shared gateway error shape, because
	// nginx and the frontend read .success and .message.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body["success"] != false {
		t.Errorf("success = %v, want false", body["success"])
	}
	if body["code"] != float64(http.StatusUnauthorized) {
		t.Errorf("code = %v, want 401", body["code"])
	}
}

func TestUpdateProfileWritesTheCallersOwnRow(t *testing.T) {
	mux, db := newTestGateway(t)
	seedProfile(t, db, "student-1", "Ada")
	seedProfile(t, db, "student-2", "Grace")

	rec := doJSON(t, mux, http.MethodPut, "/api/profile", tokenFor(t, "student-1"), map[string]string{
		"name":          "Ada Lovelace",
		"bio":           "new bio",
		"study_program": "Math",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	got := decodeUpdate(t, rec)
	if got.Id != "student-1" {
		t.Errorf("id = %q, want student-1", got.Id)
	}
	if got.Name != "Ada Lovelace" {
		t.Errorf("name = %q, want Ada Lovelace", got.Name)
	}

	// The other student's row must be untouched. A full-replacement write is
	// the most likely way to clobber a neighbour by accident.
	var other domain.UserProfile
	if err := db.First(&other, "id = ?", "student-2").Error; err != nil {
		t.Fatalf("reload student-2: %v", err)
	}
	if other.Name != "Grace" {
		t.Errorf("student-2 name = %q, want Grace", other.Name)
	}
}

// The field UpdateUserProfileRequest does have is a user_id, so this is the
// regression test for the reason the handler ignores it.
func TestUpdateProfileIgnoresAUserIDInTheBody(t *testing.T) {
	mux, db := newTestGateway(t)
	seedProfile(t, db, "student-1", "Ada")
	seedProfile(t, db, "victim-2", "Grace")

	// A signed-in attacker puts someone else's id in the request.
	rec := doJSON(t, mux, http.MethodPut, "/api/profile", tokenFor(t, "student-1"), map[string]any{
		"user_id": "victim-2",
		"name":    "pwned",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	if got := decodeUpdate(t, rec); got.Id != "student-1" {
		t.Errorf("response id = %q, want student-1 (the token subject)", got.Id)
	}

	var victim domain.UserProfile
	if err := db.First(&victim, "id = ?", "victim-2").Error; err != nil {
		t.Fatalf("reload victim-2: %v", err)
	}
	if victim.Name != "Grace" {
		t.Errorf("victim name = %q, want Grace: a user_id in the body was honoured", victim.Name)
	}
}

func TestUpdateProfileWithoutTokenIs401(t *testing.T) {
	mux, db := newTestGateway(t)
	seedProfile(t, db, "student-1", "Ada")

	rec := doJSON(t, mux, http.MethodPut, "/api/profile", "", map[string]string{"name": "pwned"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}

	var unchanged domain.UserProfile
	if err := db.First(&unchanged, "id = ?", "student-1").Error; err != nil {
		t.Fatalf("reload student-1: %v", err)
	}
	if unchanged.Name != "Ada" {
		t.Errorf("name = %q, want Ada: an unauthenticated write landed", unchanged.Name)
	}
}
