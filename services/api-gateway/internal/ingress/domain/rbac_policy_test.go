package domain_test

import (
	"testing"
	"time"

	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

func TestClaimsValidation(t *testing.T) {
	now := time.Unix(1700000000, 0)

	tests := []struct {
		name    string
		claims  domain.Claims
		wantErr error
	}{
		{
			name: "valid claims for dentist with MFA",
			claims: domain.Claims{
				Subject:     "user-uuid-001",
				SessionID:   "sess-uuid-001",
				Role:        domain.RoleDentist,
				MFAVerified: true,
				ExpiresAt:   now.Add(15 * time.Minute).Unix(),
			},
			wantErr: nil,
		},
		{
			name: "missing subject",
			claims: domain.Claims{
				SessionID:   "sess-uuid-001",
				Role:        domain.RoleDentist,
				MFAVerified: true,
				ExpiresAt:   now.Add(15 * time.Minute).Unix(),
			},
			wantErr: domain.ErrInvalidClaims,
		},
		{
			name: "missing session id",
			claims: domain.Claims{
				Subject:     "user-uuid-001",
				Role:        domain.RoleDentist,
				MFAVerified: true,
				ExpiresAt:   now.Add(15 * time.Minute).Unix(),
			},
			wantErr: domain.ErrInvalidClaims,
		},
		{
			name: "unknown role",
			claims: domain.Claims{
				Subject:     "user-uuid-001",
				SessionID:   "sess-uuid-001",
				Role:        domain.Role("hacker"),
				MFAVerified: true,
				ExpiresAt:   now.Add(15 * time.Minute).Unix(),
			},
			wantErr: domain.ErrInvalidClaims,
		},
		{
			name: "expired token",
			claims: domain.Claims{
				Subject:     "user-uuid-001",
				SessionID:   "sess-uuid-001",
				Role:        domain.RoleDentist,
				MFAVerified: true,
				ExpiresAt:   now.Add(-1 * time.Second).Unix(),
			},
			wantErr: domain.ErrTokenExpired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.claims.Validate(now)
			if err != tc.wantErr {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRequiresMFA(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/clinical/odontograms", true},
		{"/api/v1/clinical/periodontograms", true},
		{"/api/v1/clinical/soep-notes", true},
		{"/api/v1/patients/123e4567/anamnesis", true},
		{"/api/v1/treatments/plans", true},
		{"/api/v1/billing/invoices", true},
		{"/api/v1/billing/cash-close", true},
		{"/api/v1/settlements/dentists", true},
		{"/api/v1/patients", false},
		{"/api/v1/patients/123e4567", false},
		{"/api/v1/scheduling/appointments", false},
		{"/api/v1/inventory/supplies", false},
		{"/api/v1/auth/mfa/verify", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := domain.RequiresMFA(tc.path)
			if got != tc.want {
				t.Errorf("RequiresMFA(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// wi-15 (S4): the read-contract routes added under /api/v1/users,
// /api/v1/treatment-plans and /api/v1/consents.
func TestRequiresMFA_ReadContractRoutes(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/treatment-plans", false},
		{"/api/v1/treatment-plans/plan-1/approve", true},
		{"/api/v1/treatment-plans/plan-1/items/item-1/complete", true},
		{"/api/v1/patients/pat-1/treatment-plans", false},
		{"/api/v1/consents", false},
		{"/api/v1/users", false},
		{"/api/v1/patients/pat-1/consents", false},
		{"/api/v1/patients/pat-1/consents/cons-1/sign", true},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := domain.RequiresMFA(tc.path)
			if got != tc.want {
				t.Errorf("RequiresMFA(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// wi-15 (S4): the edge evaluates the concrete HTTP method, so a write on a
// patient-scoped collection still demands a fresh MFA challenge while its
// listing does not.
func TestRequiresMFAForMethod_ReadContractRoutes(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{"GET", "/api/v1/treatment-plans", false},
		{"POST", "/api/v1/treatment-plans/plan-1/approve", true},
		{"DELETE", "/api/v1/treatment-plans/plan-1/items/item-1", true},
		{"GET", "/api/v1/consents", false},
		{"GET", "/api/v1/users", false},
		{"GET", "/api/v1/patients/pat-1/consents", false},
		{"POST", "/api/v1/patients/pat-1/consents", true},
		{"POST", "/api/v1/patients/pat-1/consents/cons-1/sign", true},
		{"GET", "/api/v1/patients/pat-1/treatment-plans", false},
		{"POST", "/api/v1/patients/pat-1/treatment-plans", true},
		{"GET", "/api/v1/patients/pat-1/anamnesis", true},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			got := domain.RequiresMFAForMethod(tc.method, tc.path)
			if got != tc.want {
				t.Errorf("RequiresMFAForMethod(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestRBACPolicy_ReadContractRoutes(t *testing.T) {
	policy := domain.NewDefaultRBACPolicy()

	tests := []struct {
		name   string
		role   domain.Role
		method string
		path   string
		want   bool
	}{
		// /api/v1/users is administrative only (SRS §3).
		{name: "admin lists the clinic users", role: domain.RoleAdmin, method: "GET", path: "/api/v1/users", want: true},
		{name: "dentist cannot list the clinic users", role: domain.RoleDentist, method: "GET", path: "/api/v1/users", want: false},
		{name: "receptionist cannot list the clinic users", role: domain.RoleReceptionist, method: "GET", path: "/api/v1/users", want: false},
		{name: "assistant cannot list the clinic users", role: domain.RoleAssistant, method: "GET", path: "/api/v1/users", want: false},
		{name: "auditor cannot list the clinic users", role: domain.RoleAuditor, method: "GET", path: "/api/v1/users", want: false},

		// /api/v1/treatment-plans: parity with /api/v1/treatments.
		{name: "dentist reads the treatment plan list", role: domain.RoleDentist, method: "GET", path: "/api/v1/treatment-plans", want: true},
		{name: "dentist approves a treatment plan", role: domain.RoleDentist, method: "POST", path: "/api/v1/treatment-plans/plan-1/approve", want: true},
		{name: "receptionist reads the treatment plan list", role: domain.RoleReceptionist, method: "GET", path: "/api/v1/treatment-plans", want: true},
		{name: "receptionist cannot approve a treatment plan", role: domain.RoleReceptionist, method: "POST", path: "/api/v1/treatment-plans/plan-1/approve", want: false},
		{name: "assistant reads the treatment plan list", role: domain.RoleAssistant, method: "GET", path: "/api/v1/treatment-plans", want: true},
		{name: "assistant cannot approve a treatment plan", role: domain.RoleAssistant, method: "POST", path: "/api/v1/treatment-plans/plan-1/approve", want: false},
		{name: "auditor cannot read the treatment plan list", role: domain.RoleAuditor, method: "GET", path: "/api/v1/treatment-plans", want: false},

		// /api/v1/consents: clinical data, so the auditor has no access at all.
		{name: "dentist reads the global consent list", role: domain.RoleDentist, method: "GET", path: "/api/v1/consents", want: true},
		{name: "receptionist reads the global consent list", role: domain.RoleReceptionist, method: "GET", path: "/api/v1/consents", want: true},
		{name: "receptionist signs a consent", role: domain.RoleReceptionist, method: "POST", path: "/api/v1/patients/pat-1/consents/cons-1/sign", want: true},
		{name: "assistant reads the consent list", role: domain.RoleAssistant, method: "GET", path: "/api/v1/consents", want: true},
		{name: "assistant cannot sign a consent", role: domain.RoleAssistant, method: "POST", path: "/api/v1/patients/pat-1/consents/cons-1/sign", want: false},
		{name: "auditor cannot read the consent list", role: domain.RoleAuditor, method: "GET", path: "/api/v1/consents", want: false},
		{name: "auditor cannot read the patient consents", role: domain.RoleAuditor, method: "GET", path: "/api/v1/patients/pat-1/consents", want: false},

		// The auditor is a financial/inventory reconciler, never a clinical reader.
		{name: "auditor cannot read the patient record", role: domain.RoleAuditor, method: "GET", path: "/api/v1/patients", want: false},

		// Patient-scoped sub-resources must not fall back to the patient-record policy.
		{name: "receptionist reads the patient treatment plans", role: domain.RoleReceptionist, method: "GET", path: "/api/v1/patients/pat-1/treatment-plans", want: true},
		{name: "receptionist cannot create a patient treatment plan", role: domain.RoleReceptionist, method: "POST", path: "/api/v1/patients/pat-1/treatment-plans", want: false},
		{name: "dentist creates a patient treatment plan", role: domain.RoleDentist, method: "POST", path: "/api/v1/patients/pat-1/treatment-plans", want: true},
		{name: "receptionist reads the patient consents", role: domain.RoleReceptionist, method: "GET", path: "/api/v1/patients/pat-1/consents", want: true},
		{name: "receptionist creates a patient consent", role: domain.RoleReceptionist, method: "POST", path: "/api/v1/patients/pat-1/consents", want: true},
		{name: "assistant reads the patient consents", role: domain.RoleAssistant, method: "GET", path: "/api/v1/patients/pat-1/consents", want: true},
		{name: "assistant cannot create a patient consent", role: domain.RoleAssistant, method: "POST", path: "/api/v1/patients/pat-1/consents", want: false},

		// The admin is not blocked by the known-route registry.
		{name: "admin reads the treatment plan list", role: domain.RoleAdmin, method: "GET", path: "/api/v1/treatment-plans", want: true},
		{name: "admin reads the consent list", role: domain.RoleAdmin, method: "GET", path: "/api/v1/consents", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.IsAllowed(tc.role, tc.method, tc.path)
			if got != tc.want {
				t.Errorf("IsAllowed(role=%q, method=%q, path=%q) = %v, want %v",
					tc.role, tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestRBACPolicy_IsAllowed(t *testing.T) {
	policy := domain.NewDefaultRBACPolicy()

	tests := []struct {
		name   string
		role   domain.Role
		method string
		path   string
		want   bool
	}{
		// S5 explicit cases from plan.md:
		{
			name:   "receptionist cannot modify clinical odontograms",
			role:   domain.RoleReceptionist,
			method: "POST",
			path:   "/api/v1/clinical/odontograms",
			want:   false,
		},
		{
			name:   "receptionist cannot read clinical odontograms",
			role:   domain.RoleReceptionist,
			method: "GET",
			path:   "/api/v1/clinical/odontograms",
			want:   false,
		},
		{
			name:   "assistant cannot invoke billing invoices",
			role:   domain.RoleAssistant,
			method: "POST",
			path:   "/api/v1/billing/invoices",
			want:   false,
		},
		{
			name:   "assistant cannot read billing invoices",
			role:   domain.RoleAssistant,
			method: "GET",
			path:   "/api/v1/billing/invoices",
			want:   false,
		},
		// Positive & matrix boundary cases across the 5 roles:
		{
			name:   "dentist can modify clinical odontograms",
			role:   domain.RoleDentist,
			method: "POST",
			path:   "/api/v1/clinical/odontograms",
			want:   true,
		},
		{
			name:   "dentist can create treatment plans",
			role:   domain.RoleDentist,
			method: "POST",
			path:   "/api/v1/treatments/plans",
			want:   true,
		},
		{
			name:   "dentist cannot issue billing invoices",
			role:   domain.RoleDentist,
			method: "POST",
			path:   "/api/v1/billing/invoices",
			want:   false,
		},
		{
			name:   "dentist can read own settlements",
			role:   domain.RoleDentist,
			method: "GET",
			path:   "/api/v1/settlements",
			want:   true,
		},
		{
			name:   "receptionist can create patient",
			role:   domain.RoleReceptionist,
			method: "POST",
			path:   "/api/v1/patients",
			want:   true,
		},
		{
			name:   "receptionist cannot view patient clinical anamnesis",
			role:   domain.RoleReceptionist,
			method: "GET",
			path:   "/api/v1/patients/pat-1/anamnesis",
			want:   false,
		},
		{
			name:   "receptionist can schedule appointments",
			role:   domain.RoleReceptionist,
			method: "POST",
			path:   "/api/v1/scheduling/appointments",
			want:   true,
		},
		{
			name:   "receptionist can issue billing invoices",
			role:   domain.RoleReceptionist,
			method: "POST",
			path:   "/api/v1/billing/invoices",
			want:   true,
		},
		{
			name:   "assistant can record clinical notes and vitals",
			role:   domain.RoleAssistant,
			method: "POST",
			path:   "/api/v1/clinical/soep-notes",
			want:   true,
		},
		{
			name:   "assistant can manage clinical inventory supplies",
			role:   domain.RoleAssistant,
			method: "POST",
			path:   "/api/v1/inventory/supplies",
			want:   true,
		},
		{
			name:   "auditor can read billing and settlements and audit logs",
			role:   domain.RoleAuditor,
			method: "GET",
			path:   "/api/v1/billing/invoices",
			want:   true,
		},
		{
			name:   "auditor can read settlements",
			role:   domain.RoleAuditor,
			method: "GET",
			path:   "/api/v1/settlements/commissions",
			want:   true,
		},
		{
			name:   "auditor can read audit logs",
			role:   domain.RoleAuditor,
			method: "GET",
			path:   "/api/v1/audit/logs",
			want:   true,
		},
		{
			name:   "auditor cannot mutate billing invoices",
			role:   domain.RoleAuditor,
			method: "POST",
			path:   "/api/v1/billing/invoices",
			want:   false,
		},
		{
			name:   "auditor cannot access clinical odontograms",
			role:   domain.RoleAuditor,
			method: "GET",
			path:   "/api/v1/clinical/odontograms",
			want:   false,
		},
		{
			name:   "admin can access audit logs and all system routes",
			role:   domain.RoleAdmin,
			method: "GET",
			path:   "/api/v1/audit/logs",
			want:   true,
		},
		{
			name:   "all authenticated roles can call auth mfa verify",
			role:   domain.RoleReceptionist,
			method: "POST",
			path:   "/api/v1/auth/mfa/verify",
			want:   true,
		},
		{
			name:   "unknown route is denied by default",
			role:   domain.RoleDentist,
			method: "GET",
			path:   "/api/v1/unknown/secret",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.IsAllowed(tc.role, tc.method, tc.path)
			if got != tc.want {
				t.Errorf("IsAllowed(role=%q, method=%q, path=%q) = %v, want %v",
					tc.role, tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestHS256Verifier_VerifyToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	secret := "test-gateway-secret-key-256-bits"
	verifier := domain.NewHS256Verifier(secret, func() time.Time { return now })

	validClaims := domain.Claims{
		Subject:     "user-100",
		SessionID:   "sess-100",
		Role:        domain.RoleDentist,
		MFAVerified: true,
		ExpiresAt:   now.Add(15 * time.Minute).Unix(),
	}

	token, err := domain.SignHS256Token(validClaims, secret)
	if err != nil {
		t.Fatalf("SignHS256Token failed: %v", err)
	}

	got, err := verifier.VerifyToken(token)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if got.Subject != validClaims.Subject || got.Role != validClaims.Role || !got.MFAVerified {
		t.Fatalf("claims mismatch: got %+v, want %+v", got, validClaims)
	}

	// Reject wrong signature
	badSigToken, _ := domain.SignHS256Token(validClaims, "wrong-secret")
	if _, err := verifier.VerifyToken(badSigToken); err != domain.ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}

	// Reject alg=none attack ("eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1MSJ9.")
	noneToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1MSJ9.sig"
	if _, err := verifier.VerifyToken(noneToken); err != domain.ErrUnsupportedAlg {
		t.Fatalf("expected ErrUnsupportedAlg, got %v", err)
	}

	// Reject malformed token
	if _, err := verifier.VerifyToken("not-a-jwt"); err != domain.ErrMalformedToken {
		t.Fatalf("expected ErrMalformedToken, got %v", err)
	}
	if _, err := verifier.VerifyToken("!!!.payload.sig"); err != domain.ErrMalformedToken {
		t.Fatalf("expected ErrMalformedToken on bad header b64, got %v", err)
	}
	if _, err := verifier.VerifyToken("bm90LWpzb24.payload.sig"); err != domain.ErrMalformedToken {
		t.Fatalf("expected ErrMalformedToken on non-json header, got %v", err)
	}

	// Additional RBAC matrix branch coverage
	policy := domain.NewDefaultRBACPolicy()
	if policy.IsAllowed(domain.Role("invalid"), "GET", "/api/v1/patients") {
		t.Fatalf("expected false for invalid role")
	}
	if !policy.IsAllowed(domain.RoleAssistant, "GET", "/api/v1/patients") {
		t.Fatalf("expected assistant GET /api/v1/patients to be allowed")
	}
	if policy.IsAllowed(domain.RoleAssistant, "POST", "/api/v1/patients") {
		t.Fatalf("expected assistant POST /api/v1/patients to be denied")
	}
	if !policy.IsAllowed(domain.RoleReceptionist, "GET", "/api/v1/treatments/plans") {
		t.Fatalf("expected receptionist GET /api/v1/treatments/plans to be allowed")
	}
	if policy.IsAllowed(domain.RoleReceptionist, "POST", "/api/v1/treatments/plans") {
		t.Fatalf("expected receptionist POST /api/v1/treatments/plans to be denied")
	}
	if !policy.IsAllowed(domain.RoleAssistant, "PATCH", "/api/v1/scheduling/appointments") {
		t.Fatalf("expected assistant PATCH /api/v1/scheduling/appointments to be allowed")
	}
	if policy.IsAllowed(domain.RoleAuditor, "GET", "/api/v1/scheduling/appointments") {
		t.Fatalf("expected auditor GET /api/v1/scheduling/appointments to be denied")
	}
	if !policy.IsAllowed(domain.RoleDentist, "GET", "/api/v1/inventory/supplies") {
		t.Fatalf("expected dentist GET /api/v1/inventory/supplies to be allowed")
	}
	if policy.IsAllowed(domain.RoleReceptionist, "GET", "/api/v1/inventory/supplies") {
		t.Fatalf("expected receptionist GET /api/v1/inventory/supplies to be denied")
	}
	if policy.IsAllowed(domain.RoleDentist, "GET", "/api/v1/audit/logs") {
		t.Fatalf("expected dentist GET /api/v1/audit/logs to be denied")
	}
	if !policy.IsAllowed(domain.RoleDentist, "GET", "/api/v1/patients/p1/anamnesis") {
		t.Fatalf("expected dentist GET anamnesis to be allowed")
	}
	if domain.RequiresMFA("/") {
		t.Fatalf("expected root path not to require MFA")
	}
}
