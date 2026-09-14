package domain

import "context"

// ProfileRepository definerer alle database-operationer for profiler
type ProfileRepository interface {
	GetByID(ctx context.Context, id string) (*UserProfile, error)
	Create(ctx context.Context, profile *UserProfile) error
}

// EventPublisher is implemented by the RabbitMQ publisher, injected into the
// service layer so the service is not coupled to the transport.
type EventPublisher interface {
	PublishStudentCreated(ctx context.Context, event StudentCreatedEvent) error
}
