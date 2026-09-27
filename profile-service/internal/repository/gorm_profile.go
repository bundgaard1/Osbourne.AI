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
			return nil, fmt.Errorf("profile with id %s was not found", id)
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

// Update saves every column of an already-loaded profile.
//
// Select("*") is explicit because GORM's Save would otherwise skip zero values,
// which would make it impossible to clear a field - a user removing their phone
// number or bio would silently have the write ignored.
func (r *GORMProfileRepository) Update(ctx context.Context, profile *domain.UserProfile) error {
	err := r.db.WithContext(ctx).
		Model(&domain.UserProfile{}).
		Where("id = ?", profile.ID).
		Select("*").
		Omit("id", "created_at").
		Updates(profile).Error
	if err != nil {
		return fmt.Errorf("could not update profile: %w", err)
	}
	return nil
}
