package domain

import "time"

type User struct {
	ID   string
	Name string
	Role string
}

type Notification struct {
	ID        string
	Title     string
	Message   string
	Read      bool
	Link      string
	Timestamp time.Time
}

type Module struct {
	ID       string
	CourseID string
	Title    string
	Text     string
}

type Assignment struct {
	ID          string
	CourseID    string
	Title       string
	Description string
	DueDate     time.Time
}

type Submission struct {
	ID           string
	AssignmentID string
	StudentID    string
	Filename     string
	FileSize     int64
	SubmittedAt  time.Time
	Graded       bool
	Score        int
	Feedback     string
}

type Course struct {
	ID          string
	Code        string
	Title       string
	Description string
	Credits     int
}
