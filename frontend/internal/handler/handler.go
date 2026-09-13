package handler

import (
	"context"
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"osbourne.local/frontend/gen/profile"
	grpcclient "osbourne.local/frontend/internal/clients/grpc"
	"osbourne.local/frontend/internal/domain"
)

type contextKey string

const userKey contextKey = "currentUser"

type Handler struct {
	clients *grpcclient.Clients
}

func New(clients *grpcclient.Clients) *Handler {
	return &Handler{
		clients: clients,
	}
}

func (h *Handler) Routes(staticFiles fs.FS) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.FileServer(http.FS(staticFiles)))

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
		})
	})

	return r
}

// Middleware: Fetches the user once into the context
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("id")
		if userID == "" {
			userID = "12345"
		}

		res, err := h.clients.Profile.Client.GetUserProfile(
			r.Context(),
			&profile.ProfileRequest{UserId: userID},
		)

		user := domain.User{Name: "Guest", Role: "Unknown"}
		if err == nil {
			user = domain.User{
				ID:   res.GetId(),
				Name: res.GetName(),
				Role: res.GetRole(),
			}
		} else {
			log.Printf("gRPC user fetch failed: %v", err)
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromContext(ctx context.Context) domain.User {
	if u, ok := ctx.Value(userKey).(domain.User); ok {
		return u
	}
	return domain.User{Name: "Guest", Role: "Unknown"}
}
