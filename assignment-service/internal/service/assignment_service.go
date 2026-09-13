package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"osbourne.local/assignment-service/internal/domain"
)

var ErrAssignmentNotFound = errors.New("assignment not found")

type AssignmentService struct {
	assignmentRepo domain.AssignmentRepository
	submissionRepo domain.SubmissionRepository
	fileStore      domain.FileStorage
}

func NewAssignmentService(assignmentRepo domain.AssignmentRepository, submissionRepo domain.SubmissionRepository, fileStore domain.FileStorage) *AssignmentService {
	return &AssignmentService{
		assignmentRepo: assignmentRepo,
		submissionRepo: submissionRepo,
		fileStore:      fileStore,
	}
}

func (s *AssignmentService) CreateAssignment(ctx context.Context, assignment *domain.Assignment) error {
	if assignment.ID == "" {
		assignment.ID = uuid.NewString()
	}
	assignment.CreatedAt = time.Now()
	assignment.UpdatedAt = time.Now()

	return s.assignmentRepo.Create(ctx, assignment)
}

func (s *AssignmentService) GetCourseAssignments(ctx context.Context, courseID string) ([]*domain.Assignment, error) {
	// For simplicity, we assume that the repository has a method to list assignments by course ID.
	// This method should be implemented in the repository layer.
	assignments, err := s.assignmentRepo.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assignments for course %s: %w", courseID, err)
	}
	return assignments, nil
}

func (s *AssignmentService) GetAssignment(ctx context.Context, assignmentID string) (*domain.Assignment, error) {
	return s.assignmentRepo.GetByID(ctx, assignmentID)
}

type SubmitAssignmentInput struct {
	AssignmentID string
	StudentID    string
	FileName     string
	Size         int64
}

func (s *AssignmentService) SubmitAssignment(ctx context.Context, in SubmitAssignmentInput, src io.Reader) (*domain.Submission, error) {
	// 1. Validate that the assignment exists
	assignment, err := s.assignmentRepo.GetByID(ctx, in.AssignmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assignment: %w", err)
	}
	if assignment == nil {
		return nil, ErrAssignmentNotFound
	}

	// 2. Build a flat storage path from server-side identifiers only: the
	//    file is stored under a fresh UUID, so the path is safe against
	//    path traversal and never leaks the original filename.
	submissionID := uuid.NewString()
	fileID := uuid.NewString()
	filePath := fileID + sanitizedExtension(in.FileName)

	// 3. Save the physical file
	sizeOut, err := s.fileStore.Save(ctx, filePath, src)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// 4. Persist the submission metadata
	submission := &domain.Submission{
		ID:           submissionID,
		AssignmentID: in.AssignmentID,
		StudentID:    in.StudentID,
		FileID:      filePath,
		FileName:     in.FileName,
		FileSize:     sizeOut,
		SubmittedAt:  time.Now(),
	}
	if err := s.submissionRepo.Create(ctx, submission); err != nil {
		// ROLLBACK: if the database write fails, remove the saved file again
		_ = s.fileStore.Delete(ctx, filePath)
		return nil, fmt.Errorf("failed to persist submission: %w", err)
	}

	return submission, nil
}

func (s *AssignmentService) GetSubmission(ctx context.Context, submissionID string) (*domain.Submission, error) {
	return s.submissionRepo.GetByID(ctx, submissionID)
}

func (s *AssignmentService) ListSubmissionsByAssignment(ctx context.Context, assignmentID string) ([]*domain.Submission, error) {
	return s.submissionRepo.ListByAssignment(ctx, assignmentID)
}

func (s *AssignmentService) GradeSubmission(ctx context.Context, submissionID string, score int, feedback string) error {
	submission, err := s.submissionRepo.GetByID(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("failed to fetch submission: %w", err)
	}
	if submission == nil {
		return fmt.Errorf("cant find submission with ID: %s", submissionID)
	}

	submission.Graded = true
	submission.Score = score
	submission.Feedback = feedback

	return s.submissionRepo.Update(ctx, submission)
}

func (s *AssignmentService) FileStorage() domain.FileStorage {
	return s.fileStore
}

// sanitizedExtension returns the lowercase file extension from filename,
// allowing only [a-z0-9] characters after the leading dot. It returns an
// empty string when there is no safe extension to keep.
func sanitizedExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	if len(ext) < 2 {
		return ""
	}

	for _, r := range ext {
		if r == '.' {
			continue
		}
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}

	if len(ext) > 16 {
		ext = ext[:16]
	}
	return ext
}
