package domain

import "context"

// ProfileRepository defines all database operations for profiles.
type ProfileRepository interface {
	GetByID(ctx context.Context, id string) (*UserProfile, error)
	Create(ctx context.Context, profile *UserProfile) error
}