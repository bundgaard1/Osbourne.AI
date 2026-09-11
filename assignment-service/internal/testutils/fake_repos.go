package testutils

import (
	"context"
	"errors"
	"io"

	"osbourne.local/assignment-service/internal/domain"
)

type FakeAssignemntsRepository struct {
	domain.AssignmentRepository
	assignments map[string]*domain.Assignment
}

func NewFakeAssignmentsRepository() *FakeAssignemntsRepository {
	return &FakeAssignemntsRepository{
		assignments: map[string]*domain.Assignment{}}
}

func (f *FakeAssignemntsRepository) Create(ctx context.Context, assignment *domain.Assignment) error {
	f.assignments[assignment.ID] = assignment
	return nil
}

func (f *FakeAssignemntsRepository) GetByID(ctx context.Context, assignmentID string) (*domain.Assignment, error) {
	return f.assignments[assignmentID], nil
}

func (f *FakeAssignemntsRepository) ListByCourse(ctx context.Context, courseID string) ([]*domain.Assignment, error) {
	var out []*domain.Assignment
	for _, a := range f.assignments {
		if a.CourseID == courseID {
			out = append(out, a)
		}
	}
	return out, nil
}

type FakeSubmissionRepository struct {
	submissions map[string]*domain.Submission
	failCreate  bool
}

func NewFakeSubmissionRepository() *FakeSubmissionRepository {
	return &FakeSubmissionRepository{submissions: map[string]*domain.Submission{}}
}

func (f *FakeSubmissionRepository) Create(ctx context.Context, submission *domain.Submission) error {
	if f.failCreate {
		return errors.New("submission already exists")
	}
	f.submissions[submission.ID] = submission
	return nil
}

func (f *FakeSubmissionRepository) GetByID(ctx context.Context, submissionID string) (*domain.Submission, error) {
	return f.submissions[submissionID], nil
}

func (f *FakeSubmissionRepository) Update(ctx context.Context, submission *domain.Submission) error {
	f.submissions[submission.ID] = submission
	return nil
}

func (f *FakeSubmissionRepository) ListByAssignment(ctx context.Context, assignmentID string) ([]*domain.Submission, error) {
	var out []*domain.Submission
	for _, s := range f.submissions {
		if s.AssignmentID == assignmentID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *FakeSubmissionRepository) GetByStudentAndAssignment(ctx context.Context, studentID, assignmentID string) (*domain.Submission, error) {
	for _, s := range f.submissions {
		if s.StudentID == studentID && s.AssignmentID == assignmentID {
			return s, nil
		}
	}
	return nil, nil
}

type FakeStorage struct {
	files   map[string][]byte
	deleted []string
}

func NewFakeStorage() *FakeStorage {
	return &FakeStorage{files: map[string][]byte{}}
}

func (s *FakeStorage) Save(ctx context.Context, relativePath string, src io.Reader) (string, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return "", err
	}
	s.files[relativePath] = data
	return relativePath, nil
}

func (s *FakeStorage) Get(ctx context.Context, relativePath string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *FakeStorage) Delete(ctx context.Context, relativePath string) error {
	delete(s.files, relativePath)
	s.deleted = append(s.deleted, relativePath)
	return nil
}
