package testutils

import (
	"context"
	"errors"
	"io"

	"osbourne.local/assignment-service/internal/domain"
)

type FakeAssignemntsRepository struct {
	Assignments map[string]*domain.Assignment
}

func NewFakeAssignmentsRepository() *FakeAssignemntsRepository {
	return &FakeAssignemntsRepository{
		Assignments: map[string]*domain.Assignment{}}
}

func (f *FakeAssignemntsRepository) Create(ctx context.Context, assignment *domain.Assignment) error {
	f.Assignments[assignment.ID] = assignment
	return nil
}

func (f *FakeAssignemntsRepository) GetByID(ctx context.Context, assignmentID string) (*domain.Assignment, error) {
	return f.Assignments[assignmentID], nil
}

func (f *FakeAssignemntsRepository) ListByCourse(ctx context.Context, courseID string) ([]*domain.Assignment, error) {
	var out []*domain.Assignment
	for _, a := range f.Assignments {
		if a.CourseID == courseID {
			out = append(out, a)
		}
	}
	return out, nil
}

type FakeSubmissionRepository struct {
	Submissions map[string]*domain.Submission
	FailCreate  bool
}

func NewFakeSubmissionRepository() *FakeSubmissionRepository {
	return &FakeSubmissionRepository{Submissions: map[string]*domain.Submission{}}
}

func (f *FakeSubmissionRepository) Create(ctx context.Context, submission *domain.Submission) error {
	if f.FailCreate {
		return errors.New("submission already exists")
	}
	f.Submissions[submission.ID] = submission
	return nil
}

func (f *FakeSubmissionRepository) GetByID(ctx context.Context, submissionID string) (*domain.Submission, error) {
	return f.Submissions[submissionID], nil
}

func (f *FakeSubmissionRepository) Update(ctx context.Context, submission *domain.Submission) error {
	f.Submissions[submission.ID] = submission
	return nil
}

func (f *FakeSubmissionRepository) ListByAssignment(ctx context.Context, assignmentID string) ([]*domain.Submission, error) {
	var out []*domain.Submission
	for _, s := range f.Submissions {
		if s.AssignmentID == assignmentID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *FakeSubmissionRepository) ListByStudentAndAssignment(ctx context.Context, studentID, assignmentID string) ([]*domain.Submission, error) {
	var out []*domain.Submission
	for _, s := range f.Submissions {
		if s.StudentID == studentID && s.AssignmentID == assignmentID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *FakeSubmissionRepository) GetByStudentAndAssignment(ctx context.Context, studentID, assignmentID string) (*domain.Submission, error) {
	for _, s := range f.Submissions {
		if s.StudentID == studentID && s.AssignmentID == assignmentID {
			return s, nil
		}
	}
	return nil, nil
}

type FakeStorage struct {
	Files   map[string][]byte
	Deleted []string
}

func NewFakeFileStorage() *FakeStorage {
	return &FakeStorage{Files: map[string][]byte{}}
}

func (s *FakeStorage) Save(ctx context.Context, relativePath string, src io.Reader) (int64, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return 0, err
	}
	s.Files[relativePath] = data
	return int64(len(data)), nil
}

func (s *FakeStorage) Get(ctx context.Context, relativePath string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *FakeStorage) Delete(ctx context.Context, relativePath string) error {
	delete(s.Files, relativePath)
	s.Deleted = append(s.Deleted, relativePath)
	return nil
}
