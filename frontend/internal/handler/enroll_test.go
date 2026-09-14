package handler

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"osbourne.local/auth-common"
	"osbourne.local/frontend/gen/assignment"
	coursecatalogue "osbourne.local/frontend/gen/course-catalogue"
	coursecontent "osbourne.local/frontend/gen/course-content"
	grpcclient "osbourne.local/frontend/internal/clients/grpc"
	"osbourne.local/frontend/ui"
)

type fakeCatalogueService struct {
	coursecatalogue.UnimplementedCourseCatalogueServiceServer
}

func (f *fakeCatalogueService) GetCourse(_ context.Context, req *coursecatalogue.GetCourseRequest) (*coursecatalogue.GetCourseResponse, error) {
	if req.GetCourseId() == "nope" {
		return nil, status.Error(codes.NotFound, "course not found")
	}
	return &coursecatalogue.GetCourseResponse{Course: &coursecatalogue.Course{
		Id:          req.GetCourseId(),
		Code:        "CS101",
		Title:       "Intro to CS",
		Description: "Basics",
		Credits:     10,
	}}, nil
}

func (f *fakeCatalogueService) EnrollUser(_ context.Context, _ *coursecatalogue.EnrollUserRequest) (*coursecatalogue.EnrollUserResponse, error) {
	return &coursecatalogue.EnrollUserResponse{Success: true}, nil
}

func (f *fakeCatalogueService) ListCourses(context.Context, *coursecatalogue.ListCoursesRequest) (*coursecatalogue.ListCoursesResponse, error) {
	return &coursecatalogue.ListCoursesResponse{Courses: []*coursecatalogue.Course{
		{
			Id:          "c1",
			Code:        "CS101",
			Title:       "Intro to CS",
			Description: "Basics",
			Credits:     10,
		},
		{
			Id:          "c2",
			Code:        "CS102",
			Title:       "Data Structures",
			Description: "More basics",
			Credits:     10,
		},
	}}, nil
}

func (f *fakeCatalogueService) ListEnrolledCourses(context.Context, *coursecatalogue.ListEnrolledCoursesRequest) (*coursecatalogue.ListEnrolledCoursesResponse, error) {
	return &coursecatalogue.ListEnrolledCoursesResponse{}, nil
}

type fakeContentService struct {
	coursecontent.UnimplementedCourseContentServiceServer
}

func (f *fakeContentService) ListModulesByCourseID(_ context.Context, req *coursecontent.ListModulesByCourseIDRequest) (*coursecontent.ListModulesByCourseIDResponse, error) {
	if req.GetCourseId() == "nope" {
		return &coursecontent.ListModulesByCourseIDResponse{}, nil
	}
	return &coursecontent.ListModulesByCourseIDResponse{Modules: []*coursecontent.Module{
		{Id: "m1", CourseId: req.GetCourseId(), Title: "Module 1", Text: "First module content"},
		{Id: "m2", CourseId: req.GetCourseId(), Title: "Module 2", Text: "Second module content"},
	}}, nil
}

type fakeAssignmentService struct {
	assignment.UnimplementedAssignmentServiceServer
}

func (f *fakeAssignmentService) GetCourseAssignments(_ context.Context, _ *assignment.GetCourseAssignmentsRequest) (*assignment.GetCourseAssignmentsResponse, error) {
	return &assignment.GetCourseAssignmentsResponse{}, nil
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	const jwtSecret = "dev-secret-change-me"

	token, err := authcommon.SignJWT(jwtSecret, "12345", "student@osbourne.local", "student", time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	coursecatalogue.RegisterCourseCatalogueServiceServer(srv, &fakeCatalogueService{})
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	contentLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("content listen: %v", err)
	}
	contentSrv := grpc.NewServer()
	coursecontent.RegisterCourseContentServiceServer(contentSrv, &fakeContentService{})
	go contentSrv.Serve(contentLis)
	t.Cleanup(contentSrv.Stop)

	assignmentLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("assignment listen: %v", err)
	}
	assignmentSrv := grpc.NewServer()
	assignment.RegisterAssignmentServiceServer(assignmentSrv, &fakeAssignmentService{})
	go assignmentSrv.Serve(assignmentLis)
	t.Cleanup(assignmentSrv.Stop)

	clients, err := grpcclient.Dial("unused", "unused", "unused", lis.Addr().String(), contentLis.Addr().String(), assignmentLis.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(clients.Close)

	h := New(clients, jwtSecret)
	router := h.Routes(ui.Files)

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	jar.SetCookies(u, []*http.Cookie{{
		Name:  sessionCookieName,
		Value: token,
	}})
	ts.Client().Jar = jar

	return ts
}

func TestCoursePageNotFound(t *testing.T) {
	ts := newTestServer(t)

	resp, err := ts.Client().Get(ts.URL + "/courses/nope")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGRPCToHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", status.Error(codes.NotFound, "x"), http.StatusNotFound},
		{"permission denied", status.Error(codes.PermissionDenied, "x"), http.StatusForbidden},
		{"unauthenticated", status.Error(codes.Unauthenticated, "x"), http.StatusForbidden},
		{"unavailable", status.Error(codes.Unavailable, "x"), http.StatusServiceUnavailable},
		{"deadline exceeded", status.Error(codes.DeadlineExceeded, "x"), http.StatusServiceUnavailable},
		{"unknown default", status.Error(codes.Internal, "x"), http.StatusBadGateway},
		{"nil error", nil, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := grpcToHTTPStatus(tc.err); got != tc.want {
				t.Errorf("grpcToHTTPStatus(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestCoursePageParamRoute(t *testing.T) {
	ts := newTestServer(t)

	resp, err := ts.Client().Get(ts.URL + "/courses/c1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "Intro to CS") {
		t.Errorf("course page missing course data (param not routed): %s", body)
	}

	for _, want := range []string{"Module 1", "First module content", "Module 2", "Second module content"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("course page missing content module %q: %s", want, body)
		}
	}
}

func TestEnrollScriptsInCatalogPage(t *testing.T) {
	ts := newTestServer(t)

	resp, err := ts.Client().Get(ts.URL + "/course-catalog")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)

	if strings.Contains(html, "/static/js/app.js") {
		t.Errorf("catalog page still references app.js")
	}
	if strings.Contains(html, "hx-") {
		t.Errorf("catalog page contains htmx attributes")
	}

	if got := strings.Count(html, "class=\"course-card\""); got != 2 {
		t.Errorf("expected 2 course cards, got %d", got)
	}
	if got := strings.Count(html, "function showToast(message, kind)"); got != 1 {
		t.Errorf("showToast script should render exactly once, got %d", got)
	}
	if got := strings.Count(html, "addEventListener('submit'"); got != 1 {
		t.Errorf("enroll submit listener should render exactly once, got %d", got)
	}
	if got := strings.Count(html, `/api/courses/enroll`); got != 1 {
		t.Errorf("enroll endpoint reference should render exactly once, got %d", got)
	}
}

func TestHandleEnrollCourseRoutes(t *testing.T) {
	ts := newTestServer(t)

	form := url.Values{"course_id": {"c1"}}
	resp, err := ts.Client().Post(ts.URL+"/api/courses/enroll", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	t.Logf("response body: %s", body)

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got %q", resp.Header.Get("Content-Type"))
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	if payload["success"] != true {
		t.Errorf("expected success=true, got %v", payload["success"])
	}
	msg, _ := payload["message"].(string)
	if !strings.Contains(msg, "Enrolled in") {
		t.Errorf("expected success message, got %q", msg)
	}
}
