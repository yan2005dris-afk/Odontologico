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

// wi-15 (S4): the read-contract routes must reach their owning service instead
// of falling into UpstreamUnknown (which answers 404 ROUTE_NOT_FOUND).
func TestResolveUpstream_ReadContractRoutes(t *testing.T) {
	tests := []struct {
		name string
		path string
		want application.UpstreamTarget
	}{
		{"approve a treatment plan", "/api/v1/treatment-plans/plan-1/approve", application.UpstreamClinicalAgenda},
		{"list the treatment plans", "/api/v1/treatment-plans", application.UpstreamClinicalAgenda},
		{"list the patient treatment plans", "/api/v1/patients/pat-1/treatment-plans", application.UpstreamClinicalAgenda},
		{"complete a treatment plan item", "/api/v1/treatment-plans/plan-1/items/item-1/complete", application.UpstreamClinicalAgenda},
		{"list the clinic users", "/api/v1/users", application.UpstreamClinicalAgenda},
		{"list the consents", "/api/v1/consents", application.UpstreamClinicalAgenda},
		{"list the patient consents", "/api/v1/patients/pat-1/consents", application.UpstreamClinicalAgenda},
		{"sign a consent", "/api/v1/patients/pat-1/consents/cons-1/sign", application.UpstreamClinicalAgenda},
		{"list the invoices", "/api/v1/billing/invoices", application.UpstreamBillingStock},
		{"list the cash registers", "/api/v1/billing/cash-registers", application.UpstreamBillingStock},
		{"list the settlements", "/api/v1/settlements", application.UpstreamBillingStock},
		{"list the commission rules", "/api/v1/settlements/rules", application.UpstreamBillingStock},
		{"list the inventory items", "/api/v1/inventory/items", application.UpstreamBillingStock},
		{"list the inventory recipes", "/api/v1/inventory/recipes", application.UpstreamBillingStock},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := application.ResolveUpstream(tc.path)
			if got == application.UpstreamUnknown {
				t.Fatalf("ResolveUpstream(%q) = UpstreamUnknown, want %q", tc.path, tc.want)
			}
			if got != tc.want {
				t.Fatalf("ResolveUpstream(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// wi-15 (S4): the approval of a budget demands MFA (RNF-05) and must reach the
// clinical-agenda upstream instead of dying with 404 ROUTE_NOT_FOUND.
func TestAuthorizeRequest_TreatmentPlanApproval(t *testing.T) {
	now := time.Unix(1700000000, 0)
	secret := "gateway-read-contract-secret"
	verifier := domain.NewHS256Verifier(secret, func() time.Time { return now })
	sessions := newFakeSessionStore()
	limiter := &allowAllLimiter{allow: true}

	svc := application.NewGatewayService(verifier, sessions, limiter, domain.NewDefaultRBACPolicy())

	sessions.active["sess-approver"] = 900 * time.Second
	token, _ := domain.SignHS256Token(domain.Claims{
		Subject:     "dentist-1",
		SessionID:   "sess-approver",
		Role:        domain.RoleDentist,
		MFAVerified: true,
		ExpiresAt:   now.Add(15 * time.Minute).Unix(),
	}, secret)

	decision, err := svc.AuthorizeRequest(context.Background(), application.IngressRequest{
		Method:   "POST",
		Path:     "/api/v1/treatment-plans/plan-1/approve",
		Token:    token,
		SourceIP: "192.168.1.25",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if decision.Upstream != application.UpstreamClinicalAgenda {
		t.Fatalf("expected clinical-agenda upstream, got %q", decision.Upstream)
	}

	sessions.active["sess-approver-nomfa"] = 900 * time.Second
	tokenNoMFA, _ := domain.SignHS256Token(domain.Claims{
		Subject:     "dentist-1",
		SessionID:   "sess-approver-nomfa",
		Role:        domain.RoleDentist,
		MFAVerified: false,
		ExpiresAt:   now.Add(15 * time.Minute).Unix(),
	}, secret)

	_, err = svc.AuthorizeRequest(context.Background(), application.IngressRequest{
		Method:   "POST",
		Path:     "/api/v1/treatment-plans/plan-1/approve",
		Token:    tokenNoMFA,
		SourceIP: "192.168.1.25",
	})
	if err != domain.ErrMFARequired {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
}
