package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"osbourne.local/notification-service/internal/domain"
)

type NotificationService struct {
	repo domain.NotificationRepository
}

func NewNotificationService(repo domain.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) GetUserNotifications(ctx context.Context, id string) (*[]domain.Notification, error) {
	return s.repo.ListByUser(ctx, id)
}

// MarkNotificationAsRead flags one of userID's notifications as read.
//
// The owner is checked here rather than in the RPC handler so the check cannot
// be skipped by another caller. Without it, POST /api/notifications/{id}/read
// takes the id from the URL, which the browser controls, so any signed-in user
// could mark anyone else's notifications read by guessing ids.
//
// A notification belonging to somebody else reports NotFound rather than
// PermissionDenied: distinguishing the two would turn the endpoint into an
// oracle for discovering which notification ids exist.
func (s *NotificationService) MarkNotificationAsRead(ctx context.Context, userID, notificationID string) (*domain.Notification, error) {
	notification, err := s.repo.Get(ctx, notificationID)
	if err != nil {
		return nil, err
	}

	if notification.UserID != userID {
		return nil, status.Errorf(codes.NotFound, "notification %s was not found", notificationID)
	}

	notification.IsRead = true
	notification.UpdatedAt = time.Now()

	return s.repo.Update(ctx, notification)
}

func (s *NotificationService) CreateNotification(ctx context.Context, userID string, title string, message string) error {
	n := &domain.Notification{
		ID:        uuid.NewString(),
		UserID:    userID,
		Title:     title,
		Message:   message,
		IsRead:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return s.repo.Create(ctx, n)
}
