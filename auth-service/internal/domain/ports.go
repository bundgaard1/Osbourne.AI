package domain

import (
	"context"
	"time"
)

// AccountRepository defines all database operations for user accounts.
type AccountRepository interface {
	GetByEmail(ctx context.Context, email string) (*UserAccount, error)
	Create(ctx context.Context, account *UserAccount) error
	UpdateLastLogin(ctx context.Context, id string, lastLogin *time.Time) error
}

// EventPublisher is implemented by the RabbitMQ publisher and fires
// account.created events once an account has been persisted.
type EventPublisher interface {
	PublishAccountCreated(ctx context.Context, event AccountCreatedEvent) error
}