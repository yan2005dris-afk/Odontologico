package domain_test

import (
	"testing"
	"time"

	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

func TestClaimsValidation(t *testing.T) {
	now := time.Unix(1700000000, 0)

	tests := []struct {
		name      string
		claims    domain.Claims
		wantErr   error
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
