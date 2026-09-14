package domain

import (
	"time"
)

type UserRole string

const (
	RoleStudent UserRole = "Student"
	RoleTeacher UserRole = "Teacher"
)

// UserProfile contains the user's personal master data
type UserProfile struct {
	ID        string    `gorm:"primaryKey" json:"id"` // Same ID as UserAccount.ID
	Name      string    `gorm:"not null" json:"name"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Role      UserRole  `gorm:"type:string;default:'Student';not null" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
