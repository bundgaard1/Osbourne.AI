package domain

import "context"

type CourseCatalogueRepository interface {
	GetCourse(ctx context.Context, courseID string) (*Course, error)
	ListCourses(ctx context.Context, page int32, pageSize int32) ([]*Course, int32, error)
	CreateEnrollment(ctx context.Context, enrollment *Enrollment) error
	GetEnrolledCoursesByUserID(ctx context.Context, userID string) ([]*Course, error)
}

type EventPublisher interface {
	PublishCourseEnrolled(ctx context.Context, studentID, courseID, courseCode, courseName string) error
}
