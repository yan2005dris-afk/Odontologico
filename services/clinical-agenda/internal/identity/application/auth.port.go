package application

import (
	"context"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	SaveSession(ctx context.Context, session *domain.Session) error
	GetSession(ctx context.Context, sessionID string) (*domain.Session, error)
	MarkOTPUsed(ctx context.Context, userID, code string) bool
}

type Clock interface {
	Now() time.Time
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	SourceIP string `json:"-"`
}

type LoginOutput struct {
	MFARequired    bool   `json:"mfa_required"`
	ChallengeToken string `json:"challenge_token"`
	ExpiresInSec   int    `json:"expires_in_seconds"`
}

type VerifyMFAInput struct {
	ChallengeToken string `json:"challenge_token"`
	TOTPCode       string `json:"totp_code"`
	SourceIP       string `json:"-"`
}

type AuthenticatedUserDTO struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	FullName    string      `json:"full_name"`
	Role        domain.Role `json:"role"`
	MFAVerified bool        `json:"mfa_verified"`
}

type VerifyMFAOutput struct {
	AccessToken        string               `json:"access_token"`
	SessionID          string               `json:"session_id"`
	User               AuthenticatedUserDTO `json:"user"`
	IdleTimeoutSeconds int                  `json:"idle_timeout_seconds"`
}
