package domain

import (
	"time"
)

// UserProfile contains the user's personal master data. Identity attributes
// (email, role) live on the account in auth-service; this record only carries
// the user's profile details and is created in reaction to account.created
// events.
type UserProfile struct {
	ID          string     `gorm:"primaryKey" json:"id"` // Same ID as the account ID
	Name        string     `gorm:"not null" json:"name"`
	Birthday    *time.Time `json:"birthday,omitempty"`
	Phone       string     `json:"phone"`
	Bio         string     `json:"bio"`
	StudyProgram string    `json:"study_program"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}