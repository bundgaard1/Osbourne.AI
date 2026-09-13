package domain

import "time"

type Notification struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"index;not null" json:"user_id"` // Who the notification belongs to
	Title     string    `gorm:"not null" json:"title"`         // e.g. "New grade received"
	Message   string    `gorm:"not null" json:"message"`       // e.g. "You received a 12 in INFS-605"
	IsRead    bool      `gorm:"default:false;index" json:"is_read"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `gorm:"index" json:"updated_at"`
}
