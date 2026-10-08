package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"osbourne.local/common"
	coursecatalogue "osbourne.local/course-catalogue-service/gen/course-catalogue"
	"osbourne.local/course-catalogue-service/internal/domain"
	"osbourne.local/course-catalogue-service/internal/repository"
	"osbourne.local/course-catalogue-service/internal/server"
	"osbourne.local/course-catalogue-service/internal/service"
)

const testJWTSecret = "test-secret"

// stubPublisher records published events instead of talking to RabbitMQ, so the
// test needs no broker.
type stubPublisher struct {
	events []string
}

func (s *stubPublisher) PublishCourseEnrolled(_ context.Context, _, courseID, _, _ string) error {
	s.events = append(s.events, courseID)
	return nil
}

// newTestGateway serves the catalogue over a real gRPC connection behind the
// production gateway policy from osbourne.local/common, so the assertions cover
// the header matcher and error mapping the browser will actually hit.
func newTestGateway(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	ctx := context.Background()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&domain.Course{}, &domain.Enrollment{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	repo := repository.NewGORMCourseCatalogueRepository(db)
	courseSvc := service.NewCourseService(repo, &stubPublisher{})

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			common.AuthInterceptor(testJWTSecret),
			common.RequestLoggerInterceptor(),
		),
	)
	coursecatalogue.RegisterCourseCatalogueServiceServer(grpcServer, server.NewCourseServer(courseSvc))

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
	if err := coursecatalogue.RegisterCourseCatalogueServiceHandlerFromEndpoint(ctx, mux, bufconnTarget, []grpc.DialOption{
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return bufnet.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}); err != nil {
		t.Fatalf("register gateway: %v", err)
	}

	return mux, db
}

func tokenFor(t *testing.T, userID string) string {
	t.Helper()
	token, err := common.SignJWT(testJWTSecret, userID, userID+"@osbourne.local", "student", time.Hour)
	if err != nil {
		t.Fatalf("sign jwt for %s: %v", userID, err)
	}
	return token
}

func seedCourse(t *testing.T, db *gorm.DB, id, code, title string) {
	t.Helper()
	err := db.Create(&domain.Course{ID: id, Code: code, Title: title, Credits: 6}).Error
	if err != nil {
		t.Fatalf("seed course %s: %v", id, err)
	}
}

func do(t *testing.T, mux http.Handler, method, path, token string, payload any) *httptest.ResponseRecorder {
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

func enrolledUserIDs(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var rows []domain.Enrollment
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("load enrollments: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.UserID)
	}
	return out
}

// The catalogue is not public in this application: the frontend gates
// /course-catalog and /courses/{id} behind its own Authenticate middleware, so
// an anonymous REST call has no legitimate caller. Asserting 401 here pins that
// the blanket common.AuthInterceptor still covers the browse routes - if it were
// ever relaxed, this is the test that says so.
func TestListCoursesRequiresAToken(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-1", "INFS605", "Cloud")

	rec := do(t, mux, http.MethodGet, "/api/courses", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an anonymous browse; body %s", rec.Code, rec.Body)
	}
}

// A missing course is a client error, not a fault in the catalogue. The
// repository sentinel must translate into gRPC NotFound (-> HTTP 404); passed
// through raw it would surface as a 500, indistinguishable from an outage.
func TestGetCourseMissingReturns404(t *testing.T) {
	mux, _ := newTestGateway(t)

	rec := do(t, mux, http.MethodGet, "/api/courses/nope", tokenFor(t, "student-1"), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body)
	}

	var got struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not the gateway error shape: %v", rec.Body.String(), err)
	}
	if got.Code != http.StatusNotFound || got.Success {
		t.Errorf("got %+v, want code 404 with success=false", got)
	}
	if !strings.Contains(got.Message, "nope") {
		t.Errorf("message %q does not name the missing course", got.Message)
	}
}

func TestListCoursesForASignedInStudent(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-1", "INFS605", "Cloud")

	rec := do(t, mux, http.MethodGet, "/api/courses", tokenFor(t, "student-1"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	// Decode into a local struct rather than the generated proto, so the JSON
	// keys are spelled out and a rename of the Go field cannot pass silently
	// while the browser's result.data.courses breaks.
	var got struct {
		Courses []struct {
			ID    string `json:"id"`
			Code  string `json:"code"`
			Title string `json:"title"`
		} `json:"courses"`
		TotalCount int32 `json:"total_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not a course list: %v", rec.Body.String(), err)
	}
	if len(got.Courses) != 1 || got.Courses[0].Code != "INFS605" {
		t.Errorf("courses = %+v, want the single INFS605 course", got.Courses)
	}
	if got.TotalCount != 1 {
		t.Errorf("total_count = %d, want 1", got.TotalCount)
	}
}

func TestEnrollsTheTokenSubject(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-1", "INFS605", "Cloud")

	rec := do(t, mux, http.MethodPost, "/api/enrollments", tokenFor(t, "student-1"),
		map[string]string{"course_id": "c-1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	if got := enrolledUserIDs(t, db); len(got) != 1 || got[0] != "student-1" {
		t.Errorf("enrolled %v, want exactly [student-1]", got)
	}
}

// The enrollment body is `*`, so user_id arrives from the browser. This is the
// regression test for the handler ignoring it.
func TestEnrollIgnoresAUserIDInTheBody(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-1", "INFS605", "Cloud")

	rec := do(t, mux, http.MethodPost, "/api/enrollments", tokenFor(t, "student-1"),
		map[string]string{"course_id": "c-1", "user_id": "victim-2"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	if got := enrolledUserIDs(t, db); len(got) != 1 || got[0] != "student-1" {
		t.Errorf("enrolled %v, want [student-1]: a user_id from the body was honoured", got)
	}
}

func TestEnrollWithoutTokenIs401(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-1", "INFS605", "Cloud")

	rec := do(t, mux, http.MethodPost, "/api/enrollments", "", map[string]string{"course_id": "c-1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
	if got := enrolledUserIDs(t, db); len(got) != 0 {
		t.Errorf("enrolled %v, want nobody: an unauthenticated call created an enrollment", got)
	}
}

// The route is /api/enrollments/me, so the response must reflect the token and
// nothing the caller supplied.
func TestListEnrolledCoursesIsScopedToTheTokenSubject(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-mine", "INFS605", "Mine")
	seedCourse(t, db, "c-theirs", "INFS404", "Theirs")

	for _, e := range []domain.Enrollment{
		{ID: "e-1", CourseID: "c-mine", UserID: "student-1"},
		{ID: "e-2", CourseID: "c-theirs", UserID: "student-2"},
	} {
		if err := db.Create(&e).Error; err != nil {
			t.Fatalf("seed enrollment: %v", err)
		}
	}

	rec := do(t, mux, http.MethodGet, "/api/enrollments/me", tokenFor(t, "student-1"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var got struct {
		EnrolledCourses []struct {
			Code string `json:"code"`
		} `json:"enrolled_courses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not an enrollment list: %v", rec.Body.String(), err)
	}
	if len(got.EnrolledCourses) != 1 || got.EnrolledCourses[0].Code != "INFS605" {
		t.Errorf("enrolled_courses = %+v, want only INFS605", got.EnrolledCourses)
	}
}

func TestListEnrolledCoursesWithoutTokenIs401(t *testing.T) {
	mux, _ := newTestGateway(t)

	rec := do(t, mux, http.MethodGet, "/api/enrollments/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
}

// A bare GET /api/courses has no query parameters, which used to mean
// LIMIT 0 and an empty list alongside a non-zero total_count.
func TestListCoursesWithoutPaginationParamsStillReturnsCourses(t *testing.T) {
	mux, db := newTestGateway(t)
	seedCourse(t, db, "c-1", "INFS605", "Cloud")

	rec := do(t, mux, http.MethodGet, "/api/courses", tokenFor(t, "student-1"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var got struct {
		Courses []struct {
			Code string `json:"code"`
		} `json:"courses"`
		TotalCount int32 `json:"total_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not a course list: %v", rec.Body.String(), err)
	}
	if len(got.Courses) != 1 {
		t.Errorf("courses = %+v, want 1; total_count said %d", got.Courses, got.TotalCount)
	}
}

func TestListCoursesHonoursPagination(t *testing.T) {
	mux, db := newTestGateway(t)
	for i, code := range []string{"AA100", "BB200", "CC300"} {
		seedCourse(t, db, string(rune('c'+i)), code, "Course "+code)
	}

	rec := do(t, mux, http.MethodGet, "/api/courses?page=2&page_size=1", tokenFor(t, "student-1"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var got struct {
		Courses []struct {
			Code string `json:"code"`
		} `json:"courses"`
		TotalCount int32 `json:"total_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not a course list: %v", rec.Body.String(), err)
	}
	if len(got.Courses) != 1 {
		t.Fatalf("got %d courses, want 1 with page_size=1", len(got.Courses))
	}
	// total_count stays the unpaginated total, so the browser can render
	// "page 2 of 3".
	if got.TotalCount != 3 {
		t.Errorf("total_count = %d, want 3", got.TotalCount)
	}
}
