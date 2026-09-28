package handler

import (
	"net/http"

	"osbourne.local/common"
	"osbourne.local/frontend/internal/view"
)

// HandleLoginPage renders the sign-in form. There is no POST handler beside it
// any more: the form posts to /api/auth/login, which auth-service answers by
// setting the session cookie itself. The frontend only ever reads that cookie,
// so this handler has no credential handling left in it.
func (h *Handler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	// Already signed in? The cookie is HttpOnly, so this is the only place the
	// frontend can tell, and skipping the form saves a pointless round trip to
	// the API that would just redirect anyway.
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if _, err := common.ParseJWT(h.jwtSecret, cookie.Value); err == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}

	renderPage(w, r, view.LoginPage(view.PageData{}, ""))
}
