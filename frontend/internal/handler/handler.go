package handler

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"osbourne.local/common"
	"osbourne.local/frontend/gen/profile"
	grpcclient "osbourne.local/frontend/internal/clients/grpc"
	"osbourne.local/frontend/internal/domain"
)

type contextKey string

const userKey contextKey = "currentUser"

// sessionCookieName is the HttpOnly cookie the frontend stores the JWT in
// after a successful login.
const sessionCookieName = "osbourne_session"

// grpcCallTimeout bounds every outgoing gRPC call the frontend makes. Without
// it a service that has hung would hold the page render forever; with it the
// call fails DeadlineExceeded, which grpcToHTTPStatus maps to a 503 the user can
// act on instead of a browser tab that spins.
const grpcCallTimeout = 10 * time.Second

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
	r.Use(logRequest)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.FileServer(http.FS(staticFiles)))

	// Only the page routes live here now. Everything under /api/* is served by
	// the services themselves through the nginx api-gateway; the frontend serves
	// HTML and nothing else, so a /api path that reaches it is a misroute rather
	// than a missing handler.
	r.Get("/login", h.HandleLoginPage)

	r.Group(func(r chi.Router) {
		r.Use(h.Authenticate)
		r.Get("/", h.HandleDashboard)
		r.Get("/profile", h.HandleProfile)
		r.Get("/notifications", h.HandleNotifications)
		r.Get("/course-catalog", h.HandleCourseCatalog)
		r.Get("/courses/{courseID}", h.HandleCoursePage)
		r.Get("/courses/{courseID}/assignments/{assignmentID}", h.HandleAssignmentPage)
	})

	return r
}

// logRequest emits a structured, JSON access log line per HTTP request, tagged
// with the request id set by middleware.RequestID.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// chi's middleware.RequestID always mints a fresh id and ignores the
		// inbound one, so a page load and the API calls it triggers would carry
		// unrelated ids even though nginx ties them together. Adopting nginx's
		// id is what makes one browser action traceable end to end.
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = middleware.GetReqID(r.Context())
		}

		ctx := common.WithRequestID(r.Context(), id)
		r = r.WithContext(ctx)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		slog.InfoContext(r.Context(), "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// Middleware: verifies the session JWT and loads the user into the context.
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			redirectToLogin(w, r)
			return
		}

		claims, err := common.ParseJWT(h.jwtSecret, cookie.Value)
		if err != nil {
			slog.WarnContext(r.Context(), "invalid session token", "err", err)
			clearSessionCookie(w)
			redirectToLogin(w, r)
			return
		}

		ctx := common.WithClaims(r.Context(), claims)
		user := domain.User{
			ID:    claims.UserID,
			Email: claims.Email,
			Role:  claims.Role,
			Token: cookie.Value,
		}

		// Resolve the display name from the user's profile. The request id goes
		// along explicitly rather than through authCtx, because the user is not in
		// the context yet — that is what is being resolved. Without it this call
		// is the one gRPC hop of every page render that cannot be correlated with
		// the page request that caused it.
		gctx, cancel := context.WithTimeout(h.reqIDCtx(ctx), grpcCallTimeout)
		defer cancel()
		res, err := h.clients.Profile.Client.GetUserProfile(
			common.AttachToken(gctx, user.Token),
			&profile.ProfileRequest{UserId: user.ID},
		)
		if err == nil {
			user.Name = res.GetName()
		} else {
			slog.WarnContext(ctx, "gRPC profile fetch failed", "err", err)
			user.Name = "Unknown"
		}

		ctx = context.WithValue(ctx, userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authCtx returns a context that attaches the current user's bearer token and
// the request id to outgoing gRPC metadata, bounded by a deadline so a hung
// service cannot hang the page render forever. Every backend service verifies
// this token and can correlate on the request id.
func (h *Handler) authCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx = h.reqIDCtx(ctx)
	ctx, cancel := context.WithTimeout(ctx, grpcCallTimeout)
	if u, ok := ctx.Value(userKey).(domain.User); ok && u.Token != "" {
		ctx = common.AttachToken(ctx, u.Token)
	}
	return ctx, cancel
}

// reqIDCtx appends the current request id to outgoing gRPC metadata so backend
// services can correlate a single request across the whole stack. It reads the
// id from common rather than from chi, because logRequest may have adopted the
// id nginx sent and chi still holds the one it minted.
func (h *Handler) reqIDCtx(ctx context.Context) context.Context {
	if id := common.RequestIDFromContext(ctx); id != "" {
		return common.AttachRequestID(ctx, id)
	}
	if id := middleware.GetReqID(ctx); id != "" {
		return common.AttachRequestID(ctx, id)
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