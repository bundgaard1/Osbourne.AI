package server

import (
	"context"
	"log/slog"

	coursecatalogue "osbourne.local/course-catalogue-service/gen/course-catalogue"
	"osbourne.local/course-catalogue-service/internal/domain"
	"osbourne.local/course-catalogue-service/internal/service"
)

type CourseServer struct {
	coursecatalogue.UnimplementedCourseCatalogueServiceServer
	courseSvc *service.CourseService
}

func NewCourseServer(courseSvc *service.CourseService) *CourseServer {
	return &CourseServer{
		courseSvc: courseSvc,
	}
}

func (s *CourseServer) GetCourse(ctx context.Context, req *coursecatalogue.GetCourseRequest) (*coursecatalogue.GetCourseResponse, error) {
	slog.InfoContext(ctx, "received get_course request", "course_id", req.GetCourseId())

	course, err := s.courseSvc.GetCourse(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	courseProto := toProtoCourse(course)

	return &coursecatalogue.GetCourseResponse{
		Course: courseProto,
	}, nil
}

func (s *CourseServer) ListCourses(ctx context.Context, req *coursecatalogue.ListCoursesRequest) (*coursecatalogue.ListCoursesResponse, error) {
	slog.InfoContext(ctx, "received list_courses request", "page", req.GetPage(), "page_size", req.GetPageSize())

	courses, totalCount, err := s.courseSvc.ListCourses(ctx, req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}

	courseProtos := make([]*coursecatalogue.Course, 0, len(courses))
	for _, course := range courses {
		courseProtos = append(courseProtos, toProtoCourse(course))
	}

	return &coursecatalogue.ListCoursesResponse{
		Courses:    courseProtos,
		TotalCount: totalCount,
	}, nil
}

func (s *CourseServer) EnrollUser(ctx context.Context, req *coursecatalogue.EnrollUserRequest) (*coursecatalogue.EnrollUserResponse, error) {
	slog.InfoContext(ctx, "received enroll_user request", "course_id", req.GetCourseId())

	err := s.courseSvc.EnrollStudent(ctx, req.GetCourseId(), req.GetUserId())
	if err != nil {
		return nil, err
	}

	return &coursecatalogue.EnrollUserResponse{
		Success: true,
	}, nil
}

func (s *CourseServer) ListEnrolledCourses(ctx context.Context, req *coursecatalogue.ListEnrolledCoursesRequest) (*coursecatalogue.ListEnrolledCoursesResponse, error) {
	slog.InfoContext(ctx, "received list_enrolled_courses request")

	enrolledCourses, err := s.courseSvc.GetEnrolledCoursesByUserID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	courseProtos := make([]*coursecatalogue.Course, 0, len(enrolledCourses))
	for _, course := range enrolledCourses {
		courseProtos = append(courseProtos, toProtoCourse(course))
	}

	return &coursecatalogue.ListEnrolledCoursesResponse{
		EnrolledCourses: courseProtos,
	}, nil
}

func toProtoCourse(course *domain.Course) *coursecatalogue.Course {
	if course == nil {
		return nil
	}

	return &coursecatalogue.Course{
		Id:          course.ID,
		Code:        course.Code,
		Title:       course.Title,
		Description: course.Description,
		Credits:     int32(course.Credits),
	}
}
