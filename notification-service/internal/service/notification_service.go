package service

import (
	"context"
	"time"

	"github.com/google/uuid"
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

func (s *NotificationService) MarkNotificationAsRead(ctx context.Context, notificationID string) (*domain.Notification, error) {

	notification, err := s.repo.Get(ctx, notificationID)
	if err != nil {
		return nil, err
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
