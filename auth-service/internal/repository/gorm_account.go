package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"osbourne.local/auth-service/internal/domain"
)

type GORMAccountRepository struct {
	db *gorm.DB
}

func NewGORMAccountRepository(db *gorm.DB) *GORMAccountRepository {
	return &GORMAccountRepository{db: db}
}

func (r *GORMAccountRepository) GetByEmail(ctx context.Context, email string) (*domain.UserAccount, error) {
	var account domain.UserAccount

	err := r.db.WithContext(ctx).First(&account, "email = ?", email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("account with email %s not found", email)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &account, nil
}

func (r *GORMAccountRepository) Create(ctx context.Context, account *domain.UserAccount) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *GORMAccountRepository) UpdateLastLogin(ctx context.Context, id string, lastLogin *time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.UserAccount{}).
		Where("id = ?", id).
		Update("last_login_at", lastLogin).Error
}