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
	ListUsers(ctx context.Context, filter UserFilter) ([]*domain.User, int, error)
}

type Clock interface {
	Now() time.Time
}

// ActorMeta carries the identity of the caller as resolved by the edge
// gateway (RNF-05). The gateway injects these headers after stripping the
// spoofable ones, so they can be trusted for RBAC and audit purposes.
type ActorMeta struct {
	ActorID     string
	ActorRole   string
	MFAVerified bool
	SourceIP    string
}

// UserFilter narrows a staff listing. Limit and Offset are normalized by the
// service: limit defaults to 50 with a hard ceiling of 200, offset defaults
// to 0.
type UserFilter struct {
	Query  string
	Role   domain.Role
	Limit  int
	Offset int
}

// UserSummaryDTO is the public projection of a clinic staff account. It never
// exposes the password hash nor the TOTP secret.
type UserSummaryDTO struct {
	ID         string      `json:"id"`
	Email      string      `json:"email"`
	FullName   string      `json:"full_name"`
	Role       domain.Role `json:"role"`
	MFAEnabled bool        `json:"mfa_enabled"`
	IsActive   bool        `json:"is_active"`
}

// IdleSessionRegistrar publishes the idle-session marker of a freshly created
// session in the shared session store (Redis, `session:idle:<session_id>`).
//
// The edge gateway enforces the 15-minute idle timeout with that marker
// (RNF-05): it refreshes the TTL on every authenticated request and rejects
// the request when the key is gone. The identity context owns the birth of the
// session, so it is the component that registers the marker.
type IdleSessionRegistrar interface {
	RegisterIdleSession(ctx context.Context, sessionID string, ttl time.Duration) error
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
