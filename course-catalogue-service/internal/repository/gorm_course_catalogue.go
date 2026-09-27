package repository

import (
	"context"

	"gorm.io/gorm"
	"osbourne.local/course-catalogue-service/internal/domain"
)

type GORMCourseCatalogueRepository struct {
	db *gorm.DB
}

func NewGORMCourseCatalogueRepository(db *gorm.DB) *GORMCourseCatalogueRepository {
	return &GORMCourseCatalogueRepository{db: db}
}

func (r *GORMCourseCatalogueRepository) GetCourse(ctx context.Context, courseID string) (*domain.Course, error) {
	var course domain.Course
	if err := r.db.WithContext(ctx).First(&course, "id = ?", courseID).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

// pagination bounds applied when a request omits or misuses the paging
// parameters. The REST route makes these optional, so the bare case
// `GET /api/courses` has to mean something sensible.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// ListCourses returns one page of courses and the unpaginated total.
//
// The bounds are normalised here rather than in the RPC handler, because this
// is where the LIMIT/OFFSET arithmetic lives and because the gRPC path reaches
// it too. Un-normalised, `GET /api/courses` with no query parameters computes
// offset = (0-1)*0 = 0 with LIMIT 0, which SQL reads as "return no rows" - the
// response then says total_count: 1 with an empty list, which reads like a bug
// in the browser rather than a missing query parameter.
func (r *GORMCourseCatalogueRepository) ListCourses(ctx context.Context, page int32, pageSize int32) ([]*domain.Course, int32, error) {
	var courses []*domain.Course
	var total int64

	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = defaultPageSize
	case pageSize > maxPageSize:
		pageSize = maxPageSize
	}

	offset := (page - 1) * pageSize

	if err := r.db.WithContext(ctx).Model(&domain.Course{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).Limit(int(pageSize)).Offset(int(offset)).Find(&courses).Error; err != nil {
		return nil, 0, err
	}

	return courses, int32(total), nil
}

func (r *GORMCourseCatalogueRepository) CreateEnrollment(ctx context.Context, enrollment *domain.Enrollment) error {
	return r.db.WithContext(ctx).Create(enrollment).Error
}

func (r *GORMCourseCatalogueRepository) GetEnrolledCoursesByUserID(ctx context.Context, userID string) ([]*domain.Course, error) {
	var courses []*domain.Course

	err := r.db.WithContext(ctx).
		Table("courses").
		Joins("JOIN enrollments ON enrollments.course_id = courses.id").
		Where("enrollments.user_id = ?", userID).
		Find(&courses).Error

	if err != nil {
		return nil, err
	}

	return courses, nil
}
