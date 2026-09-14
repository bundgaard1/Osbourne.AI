package domain

// GradePublishedEvent carries the data published for a grade.published
// RabbitMQ event once a submission has been graded successfully.
type GradePublishedEvent struct {
	SubmissionID   string
	AssignmentID   string
	AssignmentName string
	StudentID      string
	CourseID       string
	Score          int
}
