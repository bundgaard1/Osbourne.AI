package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"osbourne.local/auth-common"
	"osbourne.local/frontend/gen/auth"
	"osbourne.local/frontend/internal/view"
)

// roleEmails maps the demo roles the login page picks between to the seeded
// account email in the auth-service.
var roleEmails = map[string]string{
	"student": "student@osbourne.local",
	"teacher": "teacher@osbourne.local",
}

func (h *Handler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if _, err := authcommon.ParseJWT(h.jwtSecret, cookie.Value); err == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}

	renderPage(w, r, view.LoginPage(view.PageData{}, ""))
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	role := strings.ToLower(r.FormValue("role"))
	email, ok := roleEmails[role]
	if !ok || r.FormValue("password") == "" {
		renderPage(w, r, view.LoginPage(view.PageData{}, "Choose a role and enter your password."))
		return
	}

	resp, err := h.clients.Auth.Client.Login(h.reqIDCtx(r.Context()), &auth.LoginRequest{
		Email:    email,
		Password: r.FormValue("password"),
	})
	if err != nil {
		slog.WarnContext(r.Context(), "login failed", "email", email, "err", err)
		renderPage(w, r, view.LoginPage(view.PageData{}, "Wrong email or password."))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    resp.GetToken(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}