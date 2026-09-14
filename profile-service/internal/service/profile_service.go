package service

import (
	"context"
	"time"

	"osbourne.local/profile-service/internal/domain"
)

type ProfileService struct {
	repo domain.ProfileRepository
}

func NewProfileService(repo domain.ProfileRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

func (s *ProfileService) GetProfile(ctx context.Context, id string) (*domain.UserProfile, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateProfileFromEvent creates the profile row that belongs to a newly
// created account (see internal/consumer). It is idempotent: a redelivered
// account.created event must not fail or duplicate.
func (s *ProfileService) CreateProfileFromEvent(ctx context.Context, id, name string) error {
	if _, err := s.repo.GetByID(ctx, id); err == nil {
		return nil
	}

	return s.repo.Create(ctx, &domain.UserProfile{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
}