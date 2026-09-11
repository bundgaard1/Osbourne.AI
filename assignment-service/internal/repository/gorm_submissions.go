package repository

import (
	"context"

	"gorm.io/gorm"
	"osbourne.local/assignment-service/internal/domain"
)

type GORMSubmissionRepository struct {
	db *gorm.DB
}

func NewGORMSubmissionRepository(db *gorm.DB) *GORMSubmissionRepository {
	return &GORMSubmissionRepository{db: db}
}

func (r *GORMSubmissionRepository) Create(ctx context.Context, submission *domain.Submission) error {
	return r.db.WithContext(ctx).Create(submission).Error
}

func (r *GORMSubmissionRepository) GetByID(ctx context.Context, submissionID string) (*domain.Submission, error) {
	var submission domain.Submission
	if err := r.db.WithContext(ctx).
		First(&submission, "id = ?", submissionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &submission, nil
}

func (r *GORMSubmissionRepository) Update(ctx context.Context, submission *domain.Submission) error {
	return r.db.WithContext(ctx).Save(submission).Error
}

func (r *GORMSubmissionRepository) ListByCourse(ctx context.Context, assignmentID string) ([]*domain.Submission, error) {
	var submissions []*domain.Submission
	if err := r.db.WithContext(ctx).
		Where("assignment_id = ?", assignmentID).
		Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}

func (r *GORMSubmissionRepository) ListByAssignment(ctx context.Context, assignmentID string) ([]*domain.Submission, error) {
	var submissions []*domain.Submission
	if err := r.db.WithContext(ctx).
		Where("assignment_id = ?", assignmentID).
		Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}

func (r *GORMSubmissionRepository) GetByStudentAndAssignment(ctx context.Context, studentID, assignmentID string) (*domain.Submission, error) {
	var submission domain.Submission
	if err := r.db.WithContext(ctx).
		Where("student_id = ? AND assignment_id = ?", studentID, assignmentID).
		First(&submission).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &submission, nil
}
