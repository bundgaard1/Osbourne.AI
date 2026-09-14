package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"osbourne.local/auth-service/gen/auth"
	"osbourne.local/auth-service/internal/service"
)

type AuthServer struct {
	auth.UnimplementedAuthServiceServer
	authSvc *service.AuthService
}

func NewAuthServer(authSvc *service.AuthService) *AuthServer {
	return &AuthServer{authSvc: authSvc}
}

func (s *AuthServer) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	token, account, err := s.authSvc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if err == service.ErrInvalidCredentials {
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		}
		return nil, status.Error(codes.Internal, "login failed")
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