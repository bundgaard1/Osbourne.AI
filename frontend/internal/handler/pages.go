package handler

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"osbourne.local/frontend/gen/assignment"
	coursecatalogue "osbourne.local/frontend/gen/course-catalogue"
	coursecontent "osbourne.local/frontend/gen/course-content"
	"osbourne.local/frontend/gen/notification"
	"osbourne.local/frontend/gen/profile"
	"osbourne.local/frontend/internal/domain"
	"osbourne.local/frontend/internal/view"
)

func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	currentUser := UserFromContext(r.Context())

	res, err := h.clients.CourseCatalogue.Client.ListEnrolledCourses(
		h.authCtx(r.Context()),
		&coursecatalogue.ListEnrolledCoursesRequest{
			UserId: currentUser.ID,
		},
	)
	if err != nil {
		fetchError(w, "Could not fetch the user's enrollments", err)
		return
	}

	renderPage(w, r, view.DashboardPage(view.DashboardPageData{
		PageData: view.PageData{User: currentUser},
		Courses:  toDomainCourses(res.GetEnrolledCourses()),
	}))

}

func (h *Handler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	currentUser := UserFromContext(r.Context())

	res, err := h.clients.Profile.Client.GetUserProfile(
		h.authCtx(r.Context()),
		&profile.ProfileRequest{UserId: currentUser.ID},
	)

	prof := domain.Profile{ID: currentUser.ID, Name: currentUser.Name}
	if err != nil {
		log.Printf("Could not fetch the user's profile: %v", err)
	} else {
		prof = toDomainProfile(res)
	}

	renderPage(w, r, view.ProfilePage(view.ProfilePageData{
		PageData: view.PageData{User: currentUser},
		Profile:  prof,
	}))
}

func (h *Handler) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	res, err := h.clients.Notification.Client.GetUserNotifications(
		h.authCtx(r.Context()),
		&notification.NotificationsRequest{UserId: user.ID},
	)
	if err != nil {
		fetchError(w, "Could not fetch notifications", err)
		return
	}

	renderPage(w, r, view.NotificationsPage(view.NotificationsPageData{
		PageData:      view.PageData{User: user},
		Notifications: toDomainNotifications(res.GetNotifications()),
	}))
}

func (h *Handler) HandleCourseCatalog(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	res, err := h.clients.CourseCatalogue.Client.ListCourses(
		h.authCtx(r.Context()),
		&coursecatalogue.ListCoursesRequest{
			Page:     1,
			PageSize: 10,
		},
	)
	if err != nil {
		fetchError(w, "Could not fetch course catalog", err)
		return
	}

	renderPage(w, r, view.CatalogPage(view.CatalogPageData{
		PageData: view.PageData{User: user},
		Courses:  toDomainCourses(res.GetCourses()),
	}))
}

func (h *Handler) HandleCoursePage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	courseID := chi.URLParam(r, "courseID")

	res, err := h.clients.CourseCatalogue.Client.GetCourse(
		h.authCtx(r.Context()),
		&coursecatalogue.GetCourseRequest{CourseId: courseID},
	)

	if err != nil {
		fetchError(w, "Could not fetch course details", err)
		return
	}

	res2, err := h.clients.CourseContent.Client.ListModulesByCourseID(
		h.authCtx(r.Context()),
		&coursecontent.ListModulesByCourseIDRequest{CourseId: courseID},
	)

	if err != nil {
		fetchError(w, "Could not fetch course modules", err)
		return
	}
	modules := toDomainModules(res2.GetModules())

	res3, err := h.clients.Assignment.Client.GetCourseAssignments(
		h.authCtx(r.Context()),
		&assignment.GetCourseAssignmentsRequest{CourseId: courseID},
	)
	if err != nil {
		fetchError(w, "Could not fetch course assignments", err)
		return
	}
	assignments := toDomainAssignments(res3.GetAssignments())

	coursePageData := view.CoursePageData{
		PageData:    view.PageData{User: user},
		Course:      toDomainCourse(res.GetCourse()),
		Modules:     modules,
		Assignemnts: assignments,
	}

	renderPage(w, r, view.CoursePage(coursePageData))
}

func (h *Handler) HandleAssignmentPage(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	assignmentID := chi.URLParam(r, "assignmentID")

	assignmentsRes, err := h.clients.Assignment.Client.GetAssignment(
		h.authCtx(r.Context()),
		&assignment.GetAssignmentRequest{
			AssignmentId: assignmentID,
		},
	)
	if err != nil {
		fetchError(w, "Could not fetch assignment details", err)
		return
	}

	submissionsRes, err := h.clients.Assignment.Client.ListSubmissions(
		h.authCtx(r.Context()),
		&assignment.ListSubmissionsRequest{
			AssignmentId: assignmentID,
		},
	)
	if err != nil {
		fetchError(w, "Could not fetch assignment submissions", err)
		return
	}

	renderPage(w, r, view.AssignmentPage(view.AssignmentPageData{
		PageData:    view.PageData{User: user},
		Assignment:  toDomainAssignment(assignmentsRes.GetAssignment()),
		Submissions: toDomainSubmissions(submissionsRes.GetSubmissions()),
	}))
}
