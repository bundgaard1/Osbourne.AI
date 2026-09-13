package repository

import (
	"context"

	"gorm.io/gorm"
	"osbourne.local/notification-service/internal/domain"
)

type GORMNotificationRepository struct {
	db *gorm.DB
}

func NewGORMNotificationRepository(db *gorm.DB) *GORMNotificationRepository {
	return &GORMNotificationRepository{db: db}
}

func (r *GORMNotificationRepository) ListByUser(ctx context.Context, id string) (*[]domain.Notification, error) {

	var notifications []domain.Notification

	err := r.db.WithContext(ctx).
		Where("user_id = ?", id).
		Find(&notifications).Error

	if err != nil {
		return nil, err
	}

	return &notifications, nil
}

func (r *GORMNotificationRepository) Get(ctx context.Context, id string) (*domain.Notification, error) {

	var notification domain.Notification

	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&notification).Error

	if err != nil {
		return nil, err
	}

	return &notification, nil
}

func (r *GORMNotificationRepository) Update(ctx context.Context, notification *domain.Notification) (*domain.Notification, error) {
	err := r.db.WithContext(ctx).Save(notification).Error
	if err != nil {
		return nil, err
	}
	return notification, nil
}

func (r *GORMNotificationRepository) Create(ctx context.Context, notification *domain.Notification) error {

	err := r.db.WithContext(ctx).Create(&notification).Error
	if err != nil {
		return err
	}

	return nil
}
