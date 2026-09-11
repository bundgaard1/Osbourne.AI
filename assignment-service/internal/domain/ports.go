package domain

import (
	"context"
	"io"
)

type AssignmentRepository interface {
	Create(ctx context.Context, assignment *Assignment) error
	GetByID(ctx context.Context, id string) (*Assignment, error)
	ListByCourse(ctx context.Context, courseID string) ([]*Assignment, error)
}

type SubmissionRepository interface {
	Create(ctx context.Context, submission *Submission) error
	GetByID(ctx context.Context, id string) (*Submission, error)
	Update(ctx context.Context, submission *Submission) error
	ListByAssignment(ctx context.Context, assignmentID string) ([]*Submission, error)
	GetByStudentAndAssignment(ctx context.Context, studentID, assignmentID string) (*Submission, error) // Ofte nødvendig
}

// FileStorage persists the raw bytes of submission files on the local
// filesystem. Paths are relative to the storage root and are built from
// server-side identifiers only.
type FileStorage interface {
	Save(ctx context.Context, relativePath string, src io.Reader) (string, error)
	Get(ctx context.Context, relativePath string) (io.ReadCloser, error)
	Delete(ctx context.Context, relativePath string) error
}
