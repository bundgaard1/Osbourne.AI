package database

import (
	"log/slog"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"osbourne.local/auth-service/internal/domain"
)

// demoAccounts are local example accounts. Account creation is intentionally
// not exposed anywhere; these seeds are the only way accounts come into being,
// and each seeded account produces an account.created event so profile-service
// and notification-service react to it the same way they would for any new
// account.
var demoAccounts = []struct {
	ID       string
	Email    string
	Role     domain.UserRole
	FullName string
	Password string
}{
	{ID: "12345", Email: "student@osbourne.local", Role: domain.RoleStudent, FullName: "Andy Osborne", Password: "student123"},
	{ID: "99999", Email: "teacher@osbourne.local", Role: domain.RoleTeacher, FullName: "Dr. Jane Teacher", Password: "teacher123"},
}

// SeedData inserts the demo accounts when the table is empty. It returns the
// list of account.created events for the newly created accounts so the caller
// can publish them once the RabbitMQ publisher is ready.
func SeedData(db *gorm.DB) []domain.AccountCreatedEvent {
	var count int64
	db.Model(&domain.UserAccount{}).Count(&count)
	if count > 0 {
		return nil
	}

	events := make([]domain.AccountCreatedEvent, 0, len(demoAccounts))
	for _, seed := range demoAccounts {
		hash, err := bcrypt.GenerateFromPassword([]byte(seed.Password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("failed to hash password", "email", seed.Email, "err", err)
			continue
		}

		account := domain.UserAccount{
			ID:           seed.ID,
			Email:        seed.Email,
			Role:         seed.Role,
			PasswordHash: string(hash),
			IsActive:     true,
		}

		if err := db.Create(&account).Error; err != nil {
			slog.Error("failed to seed account", "email", seed.Email, "err", err)
			continue
		}

		events = append(events, domain.AccountCreatedEvent{
			AccountID: seed.ID,
			Email:     seed.Email,
			Role:      string(seed.Role),
			FullName:  seed.FullName,
		})
	}

	return events
}