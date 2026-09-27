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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"osbourne.local/common"
	"osbourne.local/notification-service/gen/notification"
	"osbourne.local/notification-service/internal/domain"
	"osbourne.local/notification-service/internal/repository"
	"osbourne.local/notification-service/internal/server"
	"osbourne.local/notification-service/internal/service"
)

const testJWTSecret = "test-secret"

// newTestGateway serves the notification service over a real gRPC connection
// behind the production gateway policy from osbourne.local/common.
//
// It dials rather than registering the implementation in-process, because the
// behaviour under test is claims arriving in context from a verified JWT - that
// only happens if the request crosses an RPC boundary and the real
// common.AuthInterceptor runs.
func newTestGateway(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	ctx := context.Background()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&domain.Notification{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	notificationSvc := service.NewNotificationService(repository.NewGORMNotificationRepository(db))

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			common.AuthInterceptor(testJWTSecret),
			common.RequestLoggerInterceptor(),
		),
	)
	notification.RegisterNotificationServiceServer(grpcServer, server.NewNotificationServer(notificationSvc))

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
	if err := notification.RegisterNotificationServiceHandlerFromEndpoint(ctx, mux, bufconnTarget, []grpc.DialOption{
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

func do(t *testing.T, mux http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(nil))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func seedNotification(t *testing.T, db *gorm.DB, id, userID, title string, isRead bool) {
	t.Helper()
	err := db.Create(&domain.Notification{
		ID: id, UserID: userID, Title: title, Message: "body", IsRead: isRead,
	}).Error
	if err != nil {
		t.Fatalf("seed notification %s: %v", id, err)
	}
}

// wireNotification mirrors the JSON the gateway emits, rather than decoding
// into the generated proto type. Going through the proto would let a rename of
// the Go field silently pass while the browser's result.data.msg broke: it is
// the JSON keys that are the contract.
//
// It also forces the field names to be spelled out, which is what pins
// common.GatewayMarshaler's UseProtoNames setting (user_id, is_read, msg).
type wireNotification struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Msg       string `json:"msg"`
	IsRead    bool   `json:"is_read"`
	Timestamp string `json:"timestamp"`
}

type wireNotifications struct {
	Notifications []wireNotification `json:"notifications"`
}

func TestGetNotificationsIsScopedToTheTokenSubject(t *testing.T) {
	mux, db := newTestGateway(t)
	seedNotification(t, db, "n-mine", "student-1", "For me", false)
	seedNotification(t, db, "n-theirs", "student-2", "Not for me", false)

	rec := do(t, mux, http.MethodGet, "/api/notifications", tokenFor(t, "student-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var got wireNotifications
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not a notification list: %v", rec.Body.String(), err)
	}

	// The whole point of the JWT fallback: the route carries no user_id, so
	// another user's notifications must not appear.
	if len(got.Notifications) != 1 {
		t.Fatalf("got %d notifications, want exactly the caller's 1: %s",
			len(got.Notifications), rec.Body.String())
	}
	if got.Notifications[0].UserID != "student-1" {
		t.Errorf("user_id = %q, want student-1", got.Notifications[0].UserID)
	}
	if got.Notifications[0].IsRead {
		t.Error("is_read = true, want false for a freshly seeded notification")
	}
}

func TestGetNotificationsWithoutTokenIs401(t *testing.T) {
	mux, _ := newTestGateway(t)

	rec := do(t, mux, http.MethodGet, "/api/notifications", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
}

func TestMarkOwnNotificationAsRead(t *testing.T) {
	mux, db := newTestGateway(t)
	seedNotification(t, db, "n-1", "student-1", "For me", false)

	rec := do(t, mux, http.MethodPost, "/api/notifications/n-1/read", tokenFor(t, "student-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	var reloaded domain.Notification
	if err := db.First(&reloaded, "id = ?", "n-1").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.IsRead {
		t.Error("notification was not marked read")
	}
}

// The notification id lives in the URL, so it is whatever the browser sends.
// This is the regression test for the ownership check.
func TestMarkSomeoneElsesNotificationAsReadIsRejected(t *testing.T) {
	mux, db := newTestGateway(t)
	seedNotification(t, db, "n-victim", "student-2", "Victim's", false)

	rec := do(t, mux, http.MethodPost, "/api/notifications/n-victim/read", tokenFor(t, "student-1"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body %s", rec.Code, rec.Body)
	}

	// NotFound rather than PermissionDenied: telling the two apart would let a
	// caller map out which notification ids exist.
	if rec.Code == http.StatusForbidden {
		t.Error("returned 403; that leaks that the notification exists")
	}

	var reloaded domain.Notification
	if err := db.First(&reloaded, "id = ?", "n-victim").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.IsRead {
		t.Error("another user's notification was marked read")
	}
}

func TestMarkNotificationWithoutTokenIs401(t *testing.T) {
	mux, db := newTestGateway(t)
	seedNotification(t, db, "n-1", "student-1", "For me", false)

	rec := do(t, mux, http.MethodPost, "/api/notifications/n-1/read", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}

	var reloaded domain.Notification
	if err := db.First(&reloaded, "id = ?", "n-1").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.IsRead {
		t.Error("an unauthenticated request marked a notification read")
	}
}

// The service is the layer that owns the check, so assert the gRPC code the
// gateway will translate - not just the HTTP shape.
func TestOwnershipViolationIsANotFoundStatus(t *testing.T) {
	_, db := newTestGateway(t)
	seedNotification(t, db, "n-victim", "student-2", "Victim's", false)

	svc := service.NewNotificationService(repository.NewGORMNotificationRepository(db))
	_, err := svc.MarkNotificationAsRead(context.Background(), "student-1", "n-victim")
	if err == nil {
		t.Fatal("expected an error marking another user's notification")
	}
	if got := status.Code(err); got != codes.NotFound {
		t.Errorf("code = %v, want NotFound", got)
	}
}
