package service

import (
	"context"
	"log"
	"time"

	"osbourne.local/profile-service/internal/domain"
)

type ProfileService struct {
	repo   domain.ProfileRepository
	events domain.EventPublisher
}

// Injects the repository and the event publisher
func NewProfileService(repo domain.ProfileRepository, events domain.EventPublisher) *ProfileService {
	return &ProfileService{repo: repo, events: events}
}

func (s *ProfileService) GetProfile(ctx context.Context, id string) (*domain.UserProfile, error) {
	// Call the database layer
	return s.repo.GetByID(ctx, id)
}

func (s *ProfileService) CreateProfile(ctx context.Context, profile *domain.UserProfile) error {
	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()

	if err := s.repo.Create(ctx, profile); err != nil {
		return err
	}

	// Fire the domain event AFTER the profile is safely persisted. A publish
	// failure must not roll back a successful creation, so it is logged and
	// swallowed here.
	if s.events != nil {
		event := domain.StudentCreatedEvent{
			StudentID: profile.ID,
			Email:     profile.Email,
			FullName:  profile.Name,
		}
		if pubErr := s.events.PublishStudentCreated(ctx, event); pubErr != nil {
			log.Printf("Failed to publish student.created event: %v", pubErr)
		}
	}

	return nil
}
