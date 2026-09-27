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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"osbourne.local/common"
	coursecontent "osbourne.local/course-content-service/gen/course-content"
	"osbourne.local/course-content-service/internal/database"
	"osbourne.local/course-content-service/internal/repository"
	"osbourne.local/course-content-service/internal/server"
	"osbourne.local/course-content-service/internal/service"
)

const testJWTSecret = "test-secret"

// newTestGateway serves the content service over a real gRPC connection behind
// the production gateway policy from osbourne.local/common.
//
// CloverDB is a file-backed store, so each test gets its own temp directory
// rather than sharing one.
func newTestGateway(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()

	clover, err := database.NewCloverDB(t.TempDir())
	if err != nil {
		t.Fatalf("open clover db: %v", err)
	}
	t.Cleanup(func() { _ = clover.Close() })

	if err := database.SeedCloverData(clover); err != nil {
		t.Fatalf("seed clover: %v", err)
	}

	repo, err := repository.NewCloverModuleRepository(clover, "modules")
	if err != nil {
		t.Fatalf("new module repository: %v", err)
	}
	moduleSvc := service.NewModuleService(repo)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			common.AuthInterceptor(testJWTSecret),
			common.RequestLoggerInterceptor(),
		),
	)
	coursecontent.RegisterCourseContentServiceServer(grpcServer, server.NewContentServer(moduleSvc))

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
	if err := coursecontent.RegisterCourseContentServiceHandlerFromEndpoint(ctx, mux, bufconnTarget, []grpc.DialOption{
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return bufnet.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}); err != nil {
		t.Fatalf("register gateway: %v", err)
	}

	return mux
}

func tokenFor(t *testing.T, userID string) string {
	t.Helper()
	token, err := common.SignJWT(testJWTSecret, userID, userID+"@osbourne.local", "teacher", time.Hour)
	if err != nil {
		t.Fatalf("sign jwt for %s: %v", userID, err)
	}
	return token
}

func request(t *testing.T, mux http.Handler, method, path, token string, payload any) *httptest.ResponseRecorder {
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

type wireModule struct {
	ID       string `json:"id"`
	CourseID string `json:"course_id"`
	Title    string `json:"title"`
	Text     string `json:"text"`
}

// createModule posts a module and returns the id the server generated, since
// the later routes address it by path.
func createModule(t *testing.T, mux http.Handler, token, courseID, title, text string) string {
	t.Helper()
	rec := request(t, mux, http.MethodPost, "/api/courses/"+courseID+"/modules", token,
		map[string]string{"title": title, "text": text})
	if rec.Code != http.StatusOK {
		t.Fatalf("create module: status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var got struct {
		Module wireModule `json:"module"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("create body %q is not a CreateModuleResponse: %v", rec.Body.String(), err)
	}
	if got.Module.ID == "" {
		t.Fatalf("create returned no module id; body %s", rec.Body.String())
	}
	return got.Module.ID
}

// The whole point of the migration is that these routes are driven by the URL,
// so this walks the full lifecycle over HTTP rather than calling the server
// methods directly.
func TestModuleLifecycleOverREST(t *testing.T) {
	mux := newTestGateway(t)
	token := tokenFor(t, "teacher-1")

	id := createModule(t, mux, token, "c-1", "Week 1", "intro text")

	// Read it back.
	rec := request(t, mux, http.MethodGet, "/api/courses/c-1/modules/"+id, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get module: status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var got struct {
		Module wireModule `json:"module"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("get body %q is not a GetModuleResponse: %v", rec.Body.String(), err)
	}
	if got.Module.Title != "Week 1" {
		t.Errorf("title = %q, want Week 1", got.Module.Title)
	}
	if got.Module.CourseID != "c-1" {
		t.Errorf("course_id = %q, want c-1", got.Module.CourseID)
	}

	// It should appear in the course's module list.
	rec = request(t, mux, http.MethodGet, "/api/courses/c-1/modules", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list modules: status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	var list struct {
		Modules []wireModule `json:"modules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list body %q is not a ListModulesResponse: %v", rec.Body.String(), err)
	}
	if len(list.Modules) != 1 || list.Modules[0].ID != id {
		t.Errorf("modules = %+v, want the one we created (%s)", list.Modules, id)
	}

	// Update it. The route uses PUT, so the body replaces the editable fields.
	rec = request(t, mux, http.MethodPut, "/api/courses/c-1/modules/"+id, token,
		map[string]string{"title": "Week 2", "text": "revised text"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update module: status = %d, want 200; body %s", rec.Code, rec.Body)
	}
	if got := rec.Body.String(); !bytes.Contains([]byte(got), []byte("Week 2")) {
		t.Errorf("update body = %s, want it to reflect the new title", got)
	}

	rec = request(t, mux, http.MethodGet, "/api/courses/c-1/modules/"+id, token, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("re-get body: %v", err)
	}
	if got.Module.Title != "Week 2" {
		t.Errorf("title after update = %q, want Week 2", got.Module.Title)
	}

	// Delete it, and confirm it is gone rather than merely unlisted.
	rec = request(t, mux, http.MethodDelete, "/api/courses/c-1/modules/"+id, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete module: status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	rec = request(t, mux, http.MethodGet, "/api/courses/c-1/modules/"+id, token, nil)
	if rec.Code == http.StatusOK {
		t.Errorf("get after delete = 200; the module is still readable: %s", rec.Body.String())
	}
}

func TestModuleRoutesRequireAToken(t *testing.T) {
	mux := newTestGateway(t)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/courses/c-1/modules"},
		{http.MethodGet, "/api/courses/c-1/modules/m-1"},
		{http.MethodPost, "/api/courses/c-1/modules"},
		{http.MethodPut, "/api/courses/c-1/modules/m-1"},
		{http.MethodDelete, "/api/courses/c-1/modules/m-1"},
	} {
		rec := request(t, mux, tc.method, tc.path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401; body %s", tc.method, tc.path, rec.Code, rec.Body)
		}
	}
}

func TestUnknownModulePathReturnsJSON404(t *testing.T) {
	mux := newTestGateway(t)
	token := tokenFor(t, "teacher-1")

	rec := request(t, mux, http.MethodGet, "/api/courses/c-1/modules/does-not-exist", token, nil)
	if rec.Code == http.StatusOK {
		t.Fatalf("status = 200 for a missing module; body %s", rec.Body.String())
	}

	// Errors have to stay in the shared {code, success, message} shape, because
	// nginx and the frontend read those keys.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body %q is not JSON: %v", rec.Body.String(), err)
	}
	if body["success"] != false {
		t.Errorf("success = %v, want false", body["success"])
	}
}
