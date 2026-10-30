package domain

import (
	"errors"
	"time"
)

const (
	IdleTimeoutDuration = 15 * time.Minute // 900 seconds per RNF-05
	IdleTimeoutSeconds  = 900
	ChallengeTTL        = 5 * time.Minute
)

var (
	ErrSessionIdleTimeout = errors.New("session expired after 15 minutes of inactivity (RNF-05)")
	ErrSessionRevoked     = errors.New("session has been revoked or expired")
	ErrMFARequired        = errors.New("MFA verification is required for this session")
)

type Session struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Role           Role      `json:"role"`
	MFAVerified    bool      `json:"mfa_verified"`
	LastActivityAt time.Time `json:"last_activity_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Revoked        bool      `json:"revoked"`
	SourceIP       string    `json:"source_ip"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewSession(id, userID string, role Role, mfaVerified bool, sourceIP string, now time.Time) *Session {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if sourceIP == "" {
		sourceIP = "0.0.0.0"
	}
	return &Session{
		ID:             id,
		UserID:         userID,
		Role:           role,
		MFAVerified:    mfaVerified,
		LastActivityAt: now,
		ExpiresAt:      now.Add(8 * time.Hour),
		Revoked:        false,
		SourceIP:       sourceIP,
		CreatedAt:      now,
	}
}

func (s *Session) ValidateAndTouch(now time.Time) error {
	now = now.UTC()
	if s.Revoked || now.After(s.ExpiresAt) {
		s.Revoked = true
		return ErrSessionRevoked
	}
	if now.Sub(s.LastActivityAt) > IdleTimeoutDuration {
		s.Revoked = true
		return ErrSessionIdleTimeout
	}
	s.LastActivityAt = now
	return nil
}

func (s *Session) Revoke() {
	s.Revoked = true
}
