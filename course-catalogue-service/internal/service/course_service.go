package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"osbourne.local/course-catalogue-service/internal/domain"
)

type CourseService struct {
	repo   domain.CourseCatalogueRepository
	events domain.EventPublisher
}

func NewCourseService(repo domain.CourseCatalogueRepository, events domain.EventPublisher) *CourseService {
	return &CourseService{repo: repo, events: events}
}

func (s *CourseService) GetCourse(ctx context.Context, courseID string) (*domain.Course, error) {
	return s.repo.GetCourse(ctx, courseID)
}

func (s *CourseService) ListCourses(ctx context.Context, page int32, pageSize int32) ([]*domain.Course, int32, error) {
	return s.repo.ListCourses(ctx, page, pageSize)
}

func (s *CourseService) EnrollStudent(ctx context.Context, courseID string, studentID string) error {

	courses, err := s.GetEnrolledCoursesByUserID(ctx, studentID)
	if err != nil {
		return err
	}

	for _, course := range courses {
		if course.ID == courseID {
			return errors.New("student is already enrolled in this course")
		}
	}

	e := &domain.Enrollment{
		ID:         uuid.NewString(),
		CourseID:   courseID,
		UserID:     studentID,
		EnrolledAt: time.Now(),
	}

	err = s.repo.CreateEnrollment(ctx, e)
	if err != nil {
		return err
	}

	// Fire the domain event AFTER the enrollment is safely persisted. A
	// publish failure must not roll back a successful enrollment, so it is
	// logged and swallowed here.
	if s.events != nil {
		if course, gErr := s.repo.GetCourse(ctx, courseID); gErr == nil && course != nil {
			if pubErr := s.events.PublishCourseEnrolled(ctx, studentID, course.ID, course.Code, course.Title); pubErr != nil {
				slog.WarnContext(ctx, "failed to publish course.enrolled event", "student_id", studentID, "course_id", courseID, "err", pubErr)
			}
		}
	}

	return nil
}

func (s *CourseService) GetEnrolledCoursesByUserID(ctx context.Context, userID string) ([]*domain.Course, error) {
	return s.repo.GetEnrolledCoursesByUserID(ctx, userID)
}
