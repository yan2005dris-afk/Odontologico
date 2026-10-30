package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/sigo-dental/api-gateway/internal/ingress/application"
	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

type fakeSessionStore struct {
	active  map[string]time.Duration
	lastTTL time.Duration
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{active: make(map[string]time.Duration)}
}

func (f *fakeSessionStore) TouchSession(_ context.Context, sessionID string, ttl time.Duration) (bool, error) {
	if _, exists := f.active[sessionID]; !exists {
		return false, nil
	}
	f.active[sessionID] = ttl
	f.lastTTL = ttl
	return true, nil
}

type allowAllLimiter struct {
	allow bool
}

func (l *allowAllLimiter) Allow(_ string) bool {
	return l.allow
}

func TestGatewayService_AuthorizeRequest(t *testing.T) {
	now := time.Unix(1700000000, 0)
	secret := "gateway-unit-test-secret-key"
	verifier := domain.NewHS256Verifier(secret, func() time.Time { return now })
	sessions := newFakeSessionStore()
	limiter := &allowAllLimiter{allow: true}

	svc := application.NewGatewayService(
		verifier,
		sessions,
		limiter,
		domain.NewDefaultRBACPolicy(),
	)

	t.Run("S3: active session refreshes 900s sliding TTL", func(t *testing.T) {
		sessions.active["sess-active-1"] = 300 * time.Second
		tok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "dentist-1",
			SessionID:   "sess-active-1",
			Role:        domain.RoleDentist,
			MFAVerified: true,
			ExpiresAt:   now.Add(15 * time.Minute).Unix(),
		}, secret)

		decision, err := svc.AuthorizeRequest(context.Background(), application.IngressRequest{
			Method:    "GET",
			Path:      "/api/v1/clinical/odontograms",
			Token:     tok,
			SourceIP:  "192.168.1.25",
			RequestID: "req-1",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if decision.Claims.Subject != "dentist-1" {
			t.Fatalf("unexpected subject: %s", decision.Claims.Subject)
		}
		if sessions.lastTTL != 900*time.Second {
			t.Fatalf("expected sliding TTL 900s, got %v", sessions.lastTTL)
		}
	})

	t.Run("S3: expired or missing idle session returns ErrIdleSessionExpired", func(t *testing.T) {
		tok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "dentist-1",
			SessionID:   "sess-expired-999",
			Role:        domain.RoleDentist,
			MFAVerified: true,
			ExpiresAt:   now.Add(15 * time.Minute).Unix(),
		}, secret)

		_, err := svc.AuthorizeRequest(context.Background(), application.IngressRequest{
			Method:   "GET",
			Path:     "/api/v1/patients",
			Token:    tok,
			SourceIP: "192.168.1.25",
		})
		if err != domain.ErrIdleSessionExpired {
			t.Fatalf("expected ErrIdleSessionExpired, got %v", err)
		}
	})

	t.Run("S4: clinical route without mfa_verified=true returns ErrMFARequired", func(t *testing.T) {
		sessions.active["sess-nomfa"] = 900 * time.Second
		tok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "dentist-1",
			SessionID:   "sess-nomfa",
			Role:        domain.RoleDentist,
			MFAVerified: false,
			ExpiresAt:   now.Add(15 * time.Minute).Unix(),
		}, secret)

		_, err := svc.AuthorizeRequest(context.Background(), application.IngressRequest{
			Method:   "POST",
			Path:     "/api/v1/clinical/odontograms",
			Token:    tok,
			SourceIP: "192.168.1.25",
		})
		if err != domain.ErrMFARequired {
			t.Fatalf("expected ErrMFARequired, got %v", err)
		}
	})

	t.Run("S5: receptionist modifying clinical odontogram returns ErrForbiddenRole", func(t *testing.T) {
		sessions.active["sess-recep"] = 900 * time.Second
		tok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "recep-1",
			SessionID:   "sess-recep",
			Role:        domain.RoleReceptionist,
			MFAVerified: true,
			ExpiresAt:   now.Add(15 * time.Minute).Unix(),
		}, secret)

		_, err := svc.AuthorizeRequest(context.Background(), application.IngressRequest{
			Method:   "POST",
			Path:     "/api/v1/clinical/odontograms",
			Token:    tok,
			SourceIP: "192.168.1.25",
		})
		if err != domain.ErrForbiddenRole {
			t.Fatalf("expected ErrForbiddenRole, got %v", err)
		}
	})

	t.Run("Rate limit exceeded returns ErrRateLimitExceeded", func(t *testing.T) {
		limiter.allow = false
		defer func() { limiter.allow = true }()

		_, err := svc.AuthorizeRequest(context.Background(), application.IngressRequest{
			Method:   "GET",
			Path:     "/api/v1/patients",
			Token:    "any",
			SourceIP: "10.0.0.9",
		})
		if err != domain.ErrRateLimitExceeded {
			t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
		}
	})
}

func TestResolveUpstreamContext(t *testing.T) {
	tests := []struct {
		path string
		want application.UpstreamTarget
	}{
		{"/api/v1/auth/login", application.UpstreamClinicalAgenda},
		{"/api/v1/patients/123", application.UpstreamClinicalAgenda},
		{"/api/v1/clinical/odontograms", application.UpstreamClinicalAgenda},
		{"/api/v1/treatments/plans", application.UpstreamClinicalAgenda},
		{"/api/v1/scheduling/appointments", application.UpstreamClinicalAgenda},
		{"/api/v1/audit/logs", application.UpstreamClinicalAgenda},
		{"/api/v1/billing/invoices", application.UpstreamBillingStock},
		{"/api/v1/settlements/commissions", application.UpstreamBillingStock},
		{"/api/v1/inventory/supplies", application.UpstreamBillingStock},
		{"/api/v1/unknown", application.UpstreamUnknown},
	}

	for _, tc := range tests {
		got := application.ResolveUpstream(tc.path)
		if got != tc.want {
			t.Errorf("ResolveUpstream(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
