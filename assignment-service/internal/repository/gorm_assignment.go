package repository

import (
	"context"

	"gorm.io/gorm"
	"osbourne.local/assignment-service/internal/domain"
)

type GORMAssignmentRepository struct {
	db *gorm.DB
}

func NewGORMAssignmentRepository(db *gorm.DB) *GORMAssignmentRepository {
	return &GORMAssignmentRepository{db: db}
}

func (r *GORMAssignmentRepository) Create(ctx context.Context, assignment *domain.Assignment) error {
	return r.db.WithContext(ctx).Create(assignment).Error
}

func (r *GORMAssignmentRepository) GetByID(ctx context.Context, assignmentID string) (*domain.Assignment, error) {
	var assignment domain.Assignment
	if err := r.db.WithContext(ctx).First(&assignment, "id = ?", assignmentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &assignment, nil
}

func (r *GORMAssignmentRepository) ListByCourse(ctx context.Context, courseID string) ([]*domain.Assignment, error) {
	var assignments []*domain.Assignment
	if err := r.db.WithContext(ctx).Where("course_id = ?", courseID).Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}
