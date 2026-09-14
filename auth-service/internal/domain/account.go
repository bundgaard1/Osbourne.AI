package domain

import "time"

type UserRole string

const (
	RoleStudent UserRole = "student"
	RoleTeacher UserRole = "teacher"
	RoleAdmin   UserRole = "admin"
)

// UserAccount is the identity record owned by the auth-service. It carries
// login credentials plus the identity attributes that define who the user is
// (email and role); the user's master-data profile lives in profile-service.
type UserAccount struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	Email        string     `gorm:"uniqueIndex;not null" json:"email"`
	Role         UserRole   `gorm:"type:string;not null;default:'student'" json:"role"`
	PasswordHash string     `gorm:"not null" json:"-"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}