package server

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"osbourne.local/auth-service/gen/auth"
	"osbourne.local/auth-service/internal/service"
)

type AuthServer struct {
	auth.UnimplementedAuthServiceServer
	authSvc *service.AuthService
	// tokenTTL is the lifetime the service was configured with. The cookie's
	// Max-Age has to match it, so it is injected rather than re-derived.
	tokenTTL time.Duration
}

func NewAuthServer(authSvc *service.AuthService, tokenTTL time.Duration) *AuthServer {
	return &AuthServer{authSvc: authSvc, tokenTTL: tokenTTL}
}

func (s *AuthServer) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	token, account, err := s.authSvc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if err == service.ErrInvalidCredentials {
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		}
		return nil, status.Error(codes.Internal, "login failed")
	}

	// This is the session boundary: the browser's only handle on the token is
	// this HttpOnly cookie, and the gateway copies it into Authorization on the
	// way back in. Returning an error would leave the caller authenticated but
	// with no cookie, which is a worse outcome than a missing header - the user
	// is simply logged out - so it is logged and the login still succeeds.
	if err := sendSetCookie(ctx, sessionCookie(token, s.tokenTTL)); err != nil {
		slog.ErrorContext(ctx, "login succeeded but the session cookie could not be set", "user_id", account.ID, "err", err)
	}

	return &auth.LoginResponse{
		Token:  token,
		UserId: account.ID,
		Email:  account.Email,
		Role:   string(account.Role),
	}, nil
}

func (s *AuthServer) ValidateToken(ctx context.Context, req *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	claims, err := s.authSvc.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return &auth.ValidateTokenResponse{Valid: false}, nil
	}

	return &auth.ValidateTokenResponse{
		Valid:  true,
		UserId: claims.UserID,
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}

// Logout retires the session by expiring the cookie.
//
// It reads no token and consults no session store, which is deliberate: a user
// whose token has already expired still has the cookie in their browser, and if
// logout required a valid token they could never clear it - they would be stuck
// on a login page that keeps bouncing them straight back in. Expiring the
// cookie is idempotent, so repeating it is harmless.
func (s *AuthServer) Logout(ctx context.Context, _ *auth.LogoutRequest) (*auth.LogoutResponse, error) {
	if err := sendSetCookie(ctx, expiredSessionCookie()); err != nil {
		return nil, status.Error(codes.Internal, "could not clear the session cookie")
	}
	return &auth.LogoutResponse{Success: true}, nil
}
