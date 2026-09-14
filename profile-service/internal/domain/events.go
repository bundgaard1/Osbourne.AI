package domain

// StudentCreatedEvent carries the data published for a student.created
// RabbitMQ event once a new student profile has been persisted.
type StudentCreatedEvent struct {
	StudentID string
	Email     string
	FullName  string
}
