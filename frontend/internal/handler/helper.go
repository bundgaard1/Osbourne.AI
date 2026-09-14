package handler

import (
	coursecatalogue "osbourne.local/frontend/gen/course-catalogue"
	coursecontent "osbourne.local/frontend/gen/course-content"

	"osbourne.local/frontend/gen/assignment"
	"osbourne.local/frontend/gen/notification"
	"osbourne.local/frontend/gen/profile"
	"osbourne.local/frontend/internal/domain"
)

func toDomainProfile(p *profile.ProfileResponse) domain.Profile {
	if p == nil {
		return domain.Profile{}
	}

	prof := domain.Profile{
		ID:           p.GetId(),
		Name:         p.GetName(),
		Phone:        p.GetPhone(),
		Bio:          p.GetBio(),
		StudyProgram: p.GetStudyProgram(),
	}
	if p.GetBirthday() != nil {
		birthday := p.GetBirthday().AsTime()
		prof.Birthday = &birthday
	}
	return prof
}

func toDomainCourse(c *coursecatalogue.Course) domain.Course {
	if c == nil {
		return domain.Course{}
	}

	return domain.Course{
		ID:          c.GetId(),
		Code:        c.GetCode(),
		Title:       c.GetTitle(),
		Description: c.GetDescription(),
		Credits:     int(c.GetCredits()),
	}
}

func toDomainNotification(n *notification.Notification) domain.Notification {
	if n == nil {
		return domain.Notification{}
	}
	return domain.Notification{
		ID:        n.GetId(),
		Title:     n.GetTitle(),
		Message:   n.GetMsg(),
		IsRead:    n.GetIsRead(),
		Timestamp: n.GetTimestamp().AsTime(),
	}
}

func toDomainCourses(courses []*coursecatalogue.Course) []domain.Course {
	result := make([]domain.Course, 0, len(courses))
	for _, c := range courses {
		result = append(result, toDomainCourse(c))
	}
	return result
}

func toDomainNotifications(notifications []*notification.Notification) []domain.Notification {
	result := make([]domain.Notification, 0, len(notifications))
	for _, n := range notifications {
		result = append(result, toDomainNotification(n))
	}
	return result
}

func toDomainModules(modules []*coursecontent.Module) []domain.Module {
	result := make([]domain.Module, 0, len(modules))
	for _, m := range modules {
		result = append(result, toDomainModule(m))
	}
	return result
}

func toDomainModule(m *coursecontent.Module) domain.Module {
	if m == nil {
		return domain.Module{}
	}

	return domain.Module{
		ID:       m.GetId(),
		CourseID: m.GetCourseId(),
		Title:    m.GetTitle(),
		Text:     m.GetText(),
	}
}

func toDomainAssignments(assignments []*assignment.Assignment) []domain.Assignment {
	result := make([]domain.Assignment, 0, len(assignments))
	for _, a := range assignments {
		result = append(result, toDomainAssignment(a))
	}
	return result
}

func toDomainAssignment(a *assignment.Assignment) domain.Assignment {
	if a == nil {
		return domain.Assignment{}
	}

	return domain.Assignment{
		ID:          a.GetId(),
		CourseID:    a.GetCourseId(),
		Title:       a.GetTitle(),
		Description: a.GetDescription(),
		DueDate:     a.GetDueDate().AsTime(),
	}
}

func toDomainSubmissions(submissions []*assignment.Submission) []domain.Submission {
	result := make([]domain.Submission, 0, len(submissions))
	for _, s := range submissions {
		result = append(result, toDomainSubmission(s))
	}
	return result
}

func toDomainSubmission(s *assignment.Submission) domain.Submission {
	if s == nil {
		return domain.Submission{}
	}

	return domain.Submission{
		ID:           s.GetId(),
		AssignmentID: s.GetAssignmentId(),
		StudentID:    s.GetStudentId(),
		Filename:     s.GetFilename(),
		FileSize:     s.GetSize(),
		SubmittedAt:  s.GetSubmittedAt().AsTime(),
		Graded:       s.GetGraded(),
		Score:        int(s.GetScore()),
		Feedback:     s.GetFeedback(),
	}
}
