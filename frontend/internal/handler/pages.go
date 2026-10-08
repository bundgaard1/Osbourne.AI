package handler

import (
	"log/slog"
	"net/http"
	"strings"

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

	gctx, cancel := h.authCtx(r.Context())
	defer cancel()
	res, err := h.clients.CourseCatalogue.Client.ListEnrolledCourses(
		gctx,
		&coursecatalogue.ListEnrolledCoursesRequest{
			UserId: currentUser.ID,
		},
	)
	if err != nil {
		fetchError(w, r, "Could not fetch the user's enrollments", err)
		return
	}

	renderPage(w, r, view.DashboardPage(view.DashboardPageData{
		PageData: view.PageData{User: currentUser},
		Courses:  toDomainCourses(res.GetEnrolledCourses()),
	}))

}

func (h *Handler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	currentUser := UserFromContext(r.Context())

	gctx, cancel := h.authCtx(r.Context())
	defer cancel()
	res, err := h.clients.Profile.Client.GetUserProfile(
		gctx,
		&profile.ProfileRequest{UserId: currentUser.ID},
	)

	prof := domain.Profile{ID: currentUser.ID, Name: currentUser.Name}
	if err != nil {
		slog.WarnContext(r.Context(), "could not fetch the user's profile", "err", err)
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

	gctx, cancel := h.authCtx(r.Context())
	defer cancel()
	res, err := h.clients.Notification.Client.GetUserNotifications(
		gctx,
		&notification.NotificationsRequest{UserId: user.ID},
	)
	if err != nil {
		fetchError(w, r, "Could not fetch notifications", err)
		return
	}

	renderPage(w, r, view.NotificationsPage(view.NotificationsPageData{
		PageData:      view.PageData{User: user},
		Notifications: toDomainNotifications(res.GetNotifications()),
	}))
}

func (h *Handler) HandleCourseCatalog(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	gctx, cancel := h.authCtx(r.Context())
	defer cancel()
	res, err := h.clients.CourseCatalogue.Client.ListCourses(
		gctx,
		&coursecatalogue.ListCoursesRequest{
			Page:     1,
			PageSize: 10,
		},
	)
	if err != nil {
		fetchError(w, r, "Could not fetch course catalog", err)
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

	gctx, cancel := h.authCtx(r.Context())
	defer cancel()
	res, err := h.clients.CourseCatalogue.Client.GetCourse(
		gctx,
		&coursecatalogue.GetCourseRequest{CourseId: courseID},
	)

	if err != nil {
		fetchError(w, r, "Could not fetch course details", err)
		return
	}

	gctx, cancel = h.authCtx(r.Context())
	defer cancel()
	res2, err := h.clients.CourseContent.Client.ListModulesByCourseID(
		gctx,
		&coursecontent.ListModulesByCourseIDRequest{CourseId: courseID},
	)

	if err != nil {
		fetchError(w, r, "Could not fetch course modules", err)
		return
	}
	modules := toDomainModules(res2.GetModules())

	gctx, cancel = h.authCtx(r.Context())
	defer cancel()
	res3, err := h.clients.Assignment.Client.GetCourseAssignments(
		gctx,
		&assignment.GetCourseAssignmentsRequest{CourseId: courseID},
	)
	if err != nil {
		fetchError(w, r, "Could not fetch course assignments", err)
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

	gctx, cancel := h.authCtx(r.Context())
	defer cancel()
	assignmentsRes, err := h.clients.Assignment.Client.GetAssignment(
		gctx,
		&assignment.GetAssignmentRequest{
			AssignmentId: assignmentID,
		},
	)
	if err != nil {
		fetchError(w, r, "Could not fetch assignment details", err)
		return
	}

	var submissions []domain.Submission
	if strings.EqualFold(user.Role, "student") {
		// Students only ever see their own submissions; the list is scoped by
		// the authenticated identity in the assignment service.
		gctx, cancel := h.authCtx(r.Context())
		defer cancel()
		myRes, err := h.clients.Assignment.Client.ListMySubmissions(
			gctx,
			&assignment.ListMySubmissionsRequest{AssignmentId: assignmentID},
		)
		if err != nil {
			fetchError(w, r, "Could not fetch your submissions", err)
			return
		}
		submissions = toDomainSubmissions(myRes.GetSubmissions())
	} else {
		gctx, cancel := h.authCtx(r.Context())
			defer cancel()
			submissionsRes, err := h.clients.Assignment.Client.ListSubmissions(
				gctx,
				&assignment.ListSubmissionsRequest{AssignmentId: assignmentID},
			)
		if err != nil {
			fetchError(w, r, "Could not fetch assignment submissions", err)
			return
		}
		submissions = toDomainSubmissions(submissionsRes.GetSubmissions())
	}

	renderPage(w, r, view.AssignmentPage(view.AssignmentPageData{
		PageData:    view.PageData{User: user},
		Assignment:  toDomainAssignment(assignmentsRes.GetAssignment()),
		Submissions: submissions,
	}))
}
