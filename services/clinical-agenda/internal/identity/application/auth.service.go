package application

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

const (
	// DefaultListLimit is the page size used when the caller does not ask
	// for one.
	DefaultListLimit = 50
	// MaxListLimit is the hard ceiling of a listing page (RNF-07): a client
	// cannot exhaust the in-memory repository.
	MaxListLimit = 200
)

var (
	ErrInvalidChallengeToken = errors.New("invalid or expired MFA challenge token")
	ErrForbiddenRole         = errors.New("actor role is not authorized for this identity operation")
)

type defaultClock struct{}

func (defaultClock) Now() time.Time { return time.Now().UTC() }

type AuthService struct {
	repo         UserRepository
	audit        auditapp.Recorder
	clock        Clock
	jwtSecret    []byte
	idleSessions IdleSessionRegistrar
}

// NewAuthService builds the identity service.
//
// `idleSessions` registers the idle-session marker of every new session in the
// shared session store; when it is nil the service keeps working without the
// shared store (local development without Redis).
func NewAuthService(
	repo UserRepository,
	audit auditapp.Recorder,
	clock Clock,
	jwtSecret string,
	idleSessions IdleSessionRegistrar,
) *AuthService {
	if clock == nil {
		clock = defaultClock{}
	}
	if strings.TrimSpace(jwtSecret) == "" {
		jwtSecret = "sigo-dental-default-hs256-secret-key-2026"
	}
	return &AuthService{
		repo:         repo,
		audit:        audit,
		clock:        clock,
		jwtSecret:    []byte(jwtSecret),
		idleSessions: idleSessions,
	}
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !strings.Contains(email, "@") || len(input.Password) < 8 || len(input.Password) > 128 {
		return nil, domain.ErrInvalidCredentials
	}
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, domain.ErrInvalidCredentials
	}
	now := s.clock.Now()
	if !user.Active {
		return nil, domain.ErrAccountInactive
	}
	if user.IsLocked(now) {
		return nil, domain.ErrAccountLocked
	}
	if !domain.VerifyPasswordArgon2id(input.Password, user.PasswordHash) {
		user.RecordFailedAttempt(now)
		_ = s.repo.UpdateUser(ctx, user)
		if user.IsLocked(now) {
			return nil, domain.ErrAccountLocked
		}
		return nil, domain.ErrInvalidCredentials
	}

	token, err := s.signJWT(map[string]any{
		"sub":          user.ID,
		"email":        user.Email,
		"role":         string(user.Role),
		"mfa_verified": false,
		"typ":          "mfa_challenge",
		"exp":          now.Add(domain.ChallengeTTL).Unix(),
	})
	if err != nil {
		return nil, err
	}

	return &LoginOutput{
		MFARequired:    true,
		ChallengeToken: token,
		ExpiresInSec:   int(domain.ChallengeTTL.Seconds()),
	}, nil
}

func (s *AuthService) VerifyMFA(ctx context.Context, input VerifyMFAInput) (*VerifyMFAOutput, error) {
	claims, err := s.verifyJWT(input.ChallengeToken)
	if err != nil {
		return nil, ErrInvalidChallengeToken
	}
	if typ, _ := claims["typ"].(string); typ != "mfa_challenge" {
		return nil, ErrInvalidChallengeToken
	}
	userID, _ := claims["sub"].(string)
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrInvalidChallengeToken
	}

	now := s.clock.Now()
	if user.IsLocked(now) {
		return nil, domain.ErrAccountLocked
	}

	code := strings.TrimSpace(input.TOTPCode)
	if !domain.VerifyTOTP(user.MFASecretEncrypted, code, now) {
		user.RecordFailedAttempt(now)
		_ = s.repo.UpdateUser(ctx, user)
		if user.IsLocked(now) {
			return nil, domain.ErrAccountLocked
		}
		return nil, domain.ErrInvalidMFACode
	}

	// Prevent TOTP replay in the same time window (S2)
	if !s.repo.MarkOTPUsed(ctx, user.ID, code) {
		return nil, domain.ErrInvalidMFACode
	}

	user.ResetFailedAttempts(now)
	_ = s.repo.UpdateUser(ctx, user)

	sessionID := newUUID()
	sess := domain.NewSession(sessionID, user.ID, user.Role, true, input.SourceIP, now)
	if err := s.repo.SaveSession(ctx, sess); err != nil {
		return nil, err
	}

	// The gateway enforces the 15-minute idle timeout with the shared session
	// marker (RNF-05). Without it every authenticated request would be
	// rejected as an expired session.
	if s.idleSessions != nil {
		if err := s.idleSessions.RegisterIdleSession(ctx, sessionID, domain.IdleTimeoutSeconds*time.Second); err != nil {
			return nil, err
		}
	}

	accessToken, err := s.signJWT(map[string]any{
		"sub":          user.ID,
		"sid":          sessionID,
		"email":        user.Email,
		"role":         string(user.Role),
		"mfa_verified": true,
		"exp":          sess.ExpiresAt.Unix(),
	})
	if err != nil {
		return nil, err
	}

	s.recordAudit(ctx, ActorMeta{
		ActorID:     user.ID,
		ActorRole:   string(user.Role),
		MFAVerified: true,
		SourceIP:    input.SourceIP,
	}, auditdom.ActionCreate, "USER_SESSION", sessionID, map[string]string{"mfa_verified": "true"})

	return &VerifyMFAOutput{
		AccessToken: accessToken,
		SessionID:   sessionID,
		User: AuthenticatedUserDTO{
			ID:          user.ID,
			Email:       user.Email,
			FullName:    user.FullName,
			Role:        user.Role,
			MFAVerified: true,
		},
		IdleTimeoutSeconds: domain.IdleTimeoutSeconds,
	}, nil
}

func (s *AuthService) ValidateAndTouchSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	sess, err := s.repo.GetSession(ctx, strings.TrimSpace(sessionID))
	if err != nil || sess == nil {
		return nil, domain.ErrSessionRevoked
	}
	err = sess.ValidateAndTouch(s.clock.Now())
	_ = s.repo.SaveSession(ctx, sess)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	sess, err := s.repo.GetSession(ctx, strings.TrimSpace(sessionID))
	if err != nil || sess == nil {
		return nil
	}
	sess.Revoke()
	return s.repo.SaveSession(ctx, sess)
}

// ListUsers returns the clinic staff accounts (S1). Only the `admin` role may
// enumerate them: the edge gateway enforces the same rule, this check is the
// defense in depth of the service (SRS §3).
//
// The listing leaves an immutable audit trail with action EXPORT (RNF-06): a
// bulk read of personal data is an export, not a point read.
func (s *AuthService) ListUsers(ctx context.Context, actor ActorMeta, filter UserFilter) ([]UserSummaryDTO, int, error) {
	if err := requireAdminRole(actor); err != nil {
		return nil, 0, err
	}

	filter = normalizeUserFilter(filter)
	users, total, err := s.repo.ListUsers(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	items := make([]UserSummaryDTO, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		items = append(items, UserSummaryDTO{
			ID:         u.ID,
			Email:      u.Email,
			FullName:   u.FullName,
			Role:       u.Role,
			MFAEnabled: strings.TrimSpace(u.MFASecretEncrypted) != "",
			IsActive:   u.Active,
		})
	}

	s.recordAudit(ctx, actor, auditdom.ActionExport, "USER", "users", map[string]string{
		"total": strconv.Itoa(total),
	})
	return items, total, nil
}

// normalizeUserFilter applies the pagination contract of the listing API:
// limit defaults to 50 with a hard ceiling of 200 and offset defaults to 0.
func normalizeUserFilter(filter UserFilter) UserFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Role = domain.Role(strings.ToLower(strings.TrimSpace(string(filter.Role))))
	if filter.Limit <= 0 {
		filter.Limit = DefaultListLimit
	} else if filter.Limit > MaxListLimit {
		filter.Limit = MaxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func requireAdminRole(actor ActorMeta) error {
	if strings.ToLower(strings.TrimSpace(actor.ActorRole)) != string(domain.RoleAdmin) {
		return ErrForbiddenRole
	}
	return nil
}

// recordAudit appends the immutable audit trail of the operation. A failing
// recorder must never invalidate the business response, but the failure is
// logged instead of being silently discarded (RNF-06).
func (s *AuthService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	if _, err := s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	}); err != nil {
		log.Printf("audit trail could not be recorded (action=%s resource=%s/%s): %v", action, resType, resID, err)
	}
}

func (s *AuthService) signJWT(claims map[string]any) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, s.jwtSecret)
	_, _ = mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

func (s *AuthService) verifyJWT(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidChallengeToken
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, s.jwtSecret)
	_, _ = mac.Write([]byte(unsigned))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, ErrInvalidChallengeToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidChallengeToken
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidChallengeToken
	}
	if expFloat, ok := claims["exp"].(float64); ok {
		if s.clock.Now().Unix() > int64(expFloat) {
			return nil, ErrInvalidChallengeToken
		}
	}
	return claims, nil
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
