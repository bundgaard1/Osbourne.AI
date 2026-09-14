package handler

import (
	"context"
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"osbourne.local/auth-common"
	"osbourne.local/frontend/gen/profile"
	grpcclient "osbourne.local/frontend/internal/clients/grpc"
	"osbourne.local/frontend/internal/domain"
)

type contextKey string

const userKey contextKey = "currentUser"

// sessionCookieName is the HttpOnly cookie the frontend stores the JWT in
// after a successful login.
const sessionCookieName = "osbourne_session"

type Handler struct {
	clients   *grpcclient.Clients
	jwtSecret string
}

func New(clients *grpcclient.Clients, jwtSecret string) *Handler {
	return &Handler{
		clients:   clients,
		jwtSecret: jwtSecret,
	}
}

func (h *Handler) Routes(staticFiles fs.FS) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.FileServer(http.FS(staticFiles)))

	r.Get("/login", h.HandleLoginPage)
	r.Post("/login", h.HandleLogin)
	r.Post("/logout", h.HandleLogout)

	r.Group(func(r chi.Router) {
		r.Use(h.Authenticate)
		r.Get("/", h.HandleDashboard)
		r.Get("/profile", h.HandleProfile)
		r.Get("/notifications", h.HandleNotifications)
		r.Get("/course-catalog", h.HandleCourseCatalog)
		r.Get("/courses/{courseID}", h.HandleCoursePage)
		r.Get("/courses/{courseID}/assignments/{assignmentID}", h.HandleAssignmentPage)
		r.Route("/api", func(r chi.Router) {
			r.Post("/courses/enroll", h.HandleEnrollCourse)
			r.Post("/assignments/{assignmentID}/submit", h.HandleSubmitAssignment)
			r.Get("/submissions/{submissionID}/download", h.HandleDownloadSubmission)
			r.Post("/submissions/{submissionID}/grade", h.HandleGradeSubmission)
			r.Post("/notifications/{notificationID}/mark-read", h.HandleMarkNotificationRead)
		})
	})

	return r
}

// Middleware: verifies the session JWT and loads the user into the context.
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			redirectToLogin(w, r)
			return
		}

		claims, err := authcommon.ParseJWT(h.jwtSecret, cookie.Value)
		if err != nil {
			log.Printf("invalid session token: %v", err)
			clearSessionCookie(w)
			redirectToLogin(w, r)
			return
		}

		user := domain.User{
			ID:    claims.UserID,
			Email: claims.Email,
			Role:  claims.Role,
			Token: cookie.Value,
		}

		// Resolve the display name from the user's profile.
		res, err := h.clients.Profile.Client.GetUserProfile(
			authcommon.AttachToken(r.Context(), user.Token),
			&profile.ProfileRequest{UserId: user.ID},
		)
		if err == nil {
			user.Name = res.GetName()
		} else {
			log.Printf("gRPC profile fetch failed: %v", err)
			user.Name = "Unknown"
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authCtx returns a context that attaches the current user's bearer token to
// outgoing gRPC metadata. Every backend service verifies this token.
func (h *Handler) authCtx(ctx context.Context) context.Context {
	if u, ok := ctx.Value(userKey).(domain.User); ok && u.Token != "" {
		return authcommon.AttachToken(ctx, u.Token)
	}
	return ctx
}

func UserFromContext(ctx context.Context) domain.User {
	if u, ok := ctx.Value(userKey).(domain.User); ok {
		return u
	}
	return domain.User{Name: "Guest", Role: "Unknown"}
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusFound)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}