package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"osbourne.local/profile-service/internal/domain"
)

type GORMProfileRepository struct {
	db *gorm.DB
}

func NewGORMProfileRepository(db *gorm.DB) *GORMProfileRepository {
	return &GORMProfileRepository{db: db}
}

func (r *GORMProfileRepository) GetByID(ctx context.Context, id string) (*domain.UserProfile, error) {
	var profileEntity domain.UserProfile

	err := r.db.WithContext(ctx).
		First(&profileEntity, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("student with id %s was not found", id)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &profileEntity, nil
}

func (r *GORMProfileRepository) Create(ctx context.Context, profile *domain.UserProfile) error {
	err := r.db.WithContext(ctx).Create(profile).Error
	if err != nil {
		return fmt.Errorf("could not create profile: %w", err)
	}
	return nil
}
