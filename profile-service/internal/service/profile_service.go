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

// UpdateProfileInput carries the fields PUT /api/profile replaces.
//
// This is a full replacement, not a merge, and that is deliberate: PUT is
// defined as replacing the resource, and it is also the only option the wire
// format allows. The proto's fields are plain proto3 scalars, which have no
// presence tracking, so an omitted field and one sent as "" are the same
// request on the wire - meaning any merge behaviour would have to be guessed
// from JSON that has already been decoded. A client that wants to change one
// field reads the profile, edits it and PUTs the whole thing back.
type UpdateProfileInput struct {
	Name         string
	Birthday     *time.Time
	Phone        string
	Bio          string
	StudyProgram string
}

// UpdateProfile replaces the stored profile with in and returns the saved row.
func (s *ProfileService) UpdateProfile(ctx context.Context, id string, in UpdateProfileInput) (*domain.UserProfile, error) {
	// Read first so the response reflects what the database actually stored
	// rather than what we believed we wrote, and so an unknown id fails as
	// NotFound instead of silently creating a row with a NULL primary key.
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	existing.Name = in.Name
	existing.Phone = in.Phone
	existing.Bio = in.Bio
	existing.StudyProgram = in.StudyProgram
	// A nil birthday clears the field. Under full-replacement semantics that is
	// correct: no date in the request means no date on the profile.
	existing.Birthday = in.Birthday
	existing.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

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