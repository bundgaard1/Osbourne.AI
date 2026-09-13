package domain

import "time"

type Assignment struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	CourseID    string    `gorm:"index;not null" json:"course_id"` // Refers to Course.ID
	Title       string    `gorm:"not null" json:"title"`
	Description string    `json:"description"`
	DueDate     time.Time `gorm:"not null" json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Submission struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	AssignmentID string    `gorm:"index;not null" json:"assignment_id"`
	StudentID    string    `gorm:"index;not null" json:"student_id"`
	FileID       string    `gorm:"not null" json:"file_id"` // Flat UUID file identifier inside the file store
	FileName     string    `json:"file_name"`               // Original filename, kept for metadata
	FileSize     int64     `json:"file_size"`               // Size in bytes
	SubmittedAt  time.Time `json:"submitted_at"`
	Graded       bool      `gorm:"default:false" json:"graded"`
	Score        int       `json:"score,omitempty"` // Nullable, only set if graded
	Feedback     string    `json:"feedback,omitempty"`
}
