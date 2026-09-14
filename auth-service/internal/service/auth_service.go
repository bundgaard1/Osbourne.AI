package service

import (
	"context"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	"osbourne.local/auth-common"
	"osbourne.local/auth-service/internal/domain"
)

// ErrInvalidCredentials is returned for unknown emails, inactive accounts, or
// wrong passwords so the caller never leaks which part failed.
var ErrInvalidCredentials = errors.New("invalid credentials")

type Config struct {
	JWTSecret string
	TokenTTL  time.Duration
}

type AuthService struct {
	repo domain.AccountRepository
	cfg  Config
}

func NewAuthService(repo domain.AccountRepository, cfg Config) *AuthService {
	return &AuthService{repo: repo, cfg: cfg}
}

// Login validates the credentials and returns a signed JWT for the account.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *domain.UserAccount, error) {
	account, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("login failed for %s: %v", email, err)
		return "", nil, ErrInvalidCredentials
	}

	if !account.IsActive {
		return "", nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	now := time.Now()
	if err := s.repo.UpdateLastLogin(ctx, account.ID, &now); err != nil {
		log.Printf("failed to update last login for %s: %v", account.ID, err)
	}

	token, err := authcommon.SignJWT(s.cfg.JWTSecret, account.ID, account.Email, string(account.Role), s.cfg.TokenTTL)
	if err != nil {
		return "", nil, err
	}

	return token, account, nil
}

// ValidateToken verifies the JWT signature and expiry and returns its claims.
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*authcommon.Claims, error) {
	return authcommon.ParseJWT(s.cfg.JWTSecret, token)
}