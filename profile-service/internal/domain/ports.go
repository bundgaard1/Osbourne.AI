package domain

import "context"

// ProfileRepository defines all database operations for profiles.
type ProfileRepository interface {
	GetByID(ctx context.Context, id string) (*UserProfile, error)
	Create(ctx context.Context, profile *UserProfile) error
	// Update persists an existing profile in full. It is a replace rather than a
	// merge: the caller has already read the current row, so partial-field
	// semantics would only hide the case where two writers race.
	Update(ctx context.Context, profile *UserProfile) error
}