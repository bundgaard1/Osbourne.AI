package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// setCookieMetadataKey is the gRPC metadata key an RPC uses to ask the gateway
// to emit a Set-Cookie response header. common.OutgoingHeaderMatcher maps it
// through unprefixed; nothing else is allowed out of a service's response
// metadata, so this is the only way to set a cookie.
const setCookieMetadataKey = "set-cookie"

// sessionCookieName is the HttpOnly cookie the browser holds the JWT in.
//
// The name is load-bearing beyond this file: nginx maps
// `$cookie_osbourne_session` into the Authorization header, so renaming it
// silently breaks every authenticated request once the gateway is in front.
const sessionCookieName = "osbourne_session"

// sendSetCookie serialises c and pushes it out as response metadata.
//
// It deliberately does not use http.SetCookie: that only works on a
// http.ResponseWriter, and this runs inside a gRPC handler where the
// http.ResponseWriter does not exist yet. (*http.Cookie).String() produces the
// same attribute string without the header name, which is exactly the shape the
// gateway's outgoing header matcher forwards.
func sendSetCookie(ctx context.Context, c *http.Cookie) error {
	// Cookie.String() silently returns "" for an invalid cookie (a bad name, or
	// a value containing a byte that is not allowed unquoted), so an empty
	// result means the caller would believe it had set a cookie it did not.
	serialised := c.String()
	if serialised == "" {
		return fmt.Errorf("refusing to set an invalid cookie named %q", c.Name)
	}
	return grpc.SetHeader(ctx, metadata.Pairs(setCookieMetadataKey, serialised))
}

// sessionCookie builds the cookie holding a freshly issued token.
//
// MaxAge tracks the token's own lifetime rather than being left unset: a cookie
// that outlives its token would keep being replayed by the browser after the
// server has already stopped honouring it, and the user would see a login loop
// instead of a clean redirect to /login.
func sessionCookie(token string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredSessionCookie builds the cookie that retires a session.
//
// MaxAge < 0 makes (*http.Cookie).String() emit `Max-Age=0`, which is how a
// browser is told to drop the cookie. Value is emptied as well so a client that
// ignores the expiry attribute still has nothing to send back.
func expiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
