package domain

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

// Role represents one of the 5 canonical SIGO-Dental RBAC roles (SRS §3, RNF-05).
type Role string

const (
	RoleAdmin        Role = "admin"
	RoleReceptionist Role = "receptionist"
	RoleDentist      Role = "dentist"
	RoleAssistant    Role = "assistant"
	RoleAuditor      Role = "auditor"
)

var (
	ErrInvalidClaims = errors.New("invalid or incomplete JWT claims")
	ErrTokenExpired  = errors.New("JWT token has expired")
)

// Claims holds the verified edge security identity extracted from a Bearer JWT (RNF-05).
type Claims struct {
	Subject     string `json:"sub"`
	SessionID   string `json:"sid"`
	Role        Role   `json:"role"`
	MFAVerified bool   `json:"mfa_verified"`
	ExpiresAt   int64  `json:"exp"`
}

// IsValidRole returns true if role matches one of the 5 SRS §3 profiles.
func IsValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleReceptionist, RoleDentist, RoleAssistant, RoleAuditor:
		return true
	default:
		return false
	}
}

// Validate checks mandatory claim fields, role membership, and token expiration.
func (c Claims) Validate(now time.Time) error {
	if strings.TrimSpace(c.Subject) == "" {
		return ErrInvalidClaims
	}
	if strings.TrimSpace(c.SessionID) == "" {
		return ErrInvalidClaims
	}
	if !IsValidRole(c.Role) {
		return ErrInvalidClaims
	}
	if c.ExpiresAt <= 0 || now.Unix() >= c.ExpiresAt {
		return ErrTokenExpired
	}
	return nil
}

// RequiresMFA returns true if the requested path requires mfa_verified=true (S4, RNF-05).
//
// The path-only variant answers for a read of that path; the edge pipeline calls
// RequiresMFAForMethod, which evaluates the concrete HTTP method so that a
// listing does not demand an extra MFA challenge while its writes still do.
func RequiresMFA(path string) bool {
	return RequiresMFAForMethod(http.MethodGet, path)
}

// RequiresMFAForMethod returns true if the method/path pair requires
// mfa_verified=true (S4, RNF-05).
func RequiresMFAForMethod(method, path string) bool {
	clean := strings.TrimRight(strings.TrimSpace(path), "/")
	if clean == "" {
		return false
	}
	m := strings.ToUpper(strings.TrimSpace(method))

	// Clinical and financial namespaces keep their blanket MFA requirement.
	if strings.HasPrefix(clean, "/api/v1/clinical") ||
		strings.HasPrefix(clean, "/api/v1/treatments") ||
		strings.HasPrefix(clean, "/api/v1/billing") ||
		strings.HasPrefix(clean, "/api/v1/settlements") {
		return true
	}

	// Listing-only collections of the read contract (wi-15): the live session is enough.
	switch clean {
	case "/api/v1/treatment-plans", "/api/v1/consents", "/api/v1/users":
		return false
	}

	// Write sub-resources of the read contract (approve, complete, sign, ...).
	if strings.HasPrefix(clean, "/api/v1/treatment-plans/") ||
		strings.HasPrefix(clean, "/api/v1/consents/") {
		return true
	}

	// Patient-scoped sub-resources.
	if sub, ok := patientSubResource(clean); ok {
		switch {
		case isAnamnesisSubResource(sub):
			return true
		case sub == "treatment-plans", sub == "consents":
			// Listing GET needs no extra MFA; creating a plan or a consent does.
			return m != http.MethodGet
		case strings.HasPrefix(sub, "consents/") && strings.HasSuffix(sub, "/sign"):
			// The consent signature always demands a fresh MFA challenge.
			return true
		}
	}
	return false
}

// patientSubResource returns the sub-resource of a patient-scoped route, e.g.
// "consents/{consentId}/sign" for "/api/v1/patients/{id}/consents/{consentId}/sign".
func patientSubResource(clean string) (string, bool) {
	const prefix = "/api/v1/patients/"
	if !strings.HasPrefix(clean, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(clean, prefix)
	idx := strings.Index(rest, "/")
	if idx < 0 {
		return "", false
	}
	return rest[idx+1:], true
}

// isAnamnesisSubResource reports whether a patient-scoped sub-resource is the
// anamnesis, the clinical history restricted from reception and audit.
func isAnamnesisSubResource(sub string) bool {
	return sub == "anamnesis" || strings.HasSuffix(sub, "/anamnesis")
}

// RBACPolicy evaluates edge authorization rules for the 5 canonical roles (S5, SRS §3, RNF-05).
type RBACPolicy struct{}

// NewDefaultRBACPolicy constructs the default SIGO-Dental edge RBAC policy.
func NewDefaultRBACPolicy() *RBACPolicy {
	return &RBACPolicy{}
}

// IsAllowed checks whether a given role may invoke method on path.
func (p *RBACPolicy) IsAllowed(role Role, method, path string) bool {
	if !IsValidRole(role) {
		return false
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	clean := strings.TrimRight(path, "/")
	sub, hasSub := patientSubResource(clean)

	// Auth routes (e.g., /api/v1/auth/mfa/verify, /api/v1/auth/logout) are accessible to all authenticated roles.
	if strings.HasPrefix(clean, "/api/v1/auth") {
		return true
	}

	// Admin has full administrative and system route access.
	if role == RoleAdmin {
		return isKnownAPIRoute(clean)
	}

	switch {
	// 1. Clinic user directory (/api/v1/users*) — administrative only (SRS §3).
	case strings.HasPrefix(clean, "/api/v1/users"):
		return role == RoleAdmin

	// 2. Clinical workspace (/api/v1/clinical/*)
	case strings.HasPrefix(clean, "/api/v1/clinical"):
		// Dentist and Assistant can access clinical workspace; Receptionist and Auditor cannot.
		return role == RoleDentist || role == RoleAssistant

	// 3. Patient Anamnesis (/api/v1/patients/*/anamnesis) — clinical history restricted from receptionist/auditor
	case hasSub && isAnamnesisSubResource(sub):
		return role == RoleDentist || role == RoleAssistant

	// 4. Treatment Plans & Budgets (/api/v1/treatment-plans*, /api/v1/patients/*/treatment-plans)
	//    Parity with the legacy /api/v1/treatments namespace: the dentist owns the plan,
	//    reception and assistance only read it.
	case strings.HasPrefix(clean, "/api/v1/treatment-plans") || (hasSub && sub == "treatment-plans"):
		if role == RoleDentist {
			return true
		}
		if role == RoleReceptionist || role == RoleAssistant {
			return m == http.MethodGet
		}
		return false

	// 5. Digital Informed Consent (/api/v1/consents*, /api/v1/patients/*/consents*)
	//    Clinical data: the auditor has no access at all (SRS §3).
	case strings.HasPrefix(clean, "/api/v1/consents") ||
		(hasSub && (sub == "consents" || strings.HasPrefix(sub, "consents/"))):
		if role == RoleDentist || role == RoleReceptionist {
			return true
		}
		if role == RoleAssistant {
			return m == http.MethodGet
		}
		return false

	// 6. Unified Patient Demographic Record (/api/v1/patients*)
	case strings.HasPrefix(clean, "/api/v1/patients"):
		if role == RoleReceptionist || role == RoleDentist {
			return true
		}
		if role == RoleAssistant {
			return m == http.MethodGet
		}
		return false

	// 7. Treatment Plans & Budgets, legacy namespace (/api/v1/treatments*)
	case strings.HasPrefix(clean, "/api/v1/treatments"):
		if role == RoleDentist {
			return true
		}
		if role == RoleReceptionist || role == RoleAssistant {
			return m == http.MethodGet
		}
		return false

	// 8. Scheduling & Multi-Chair Calendar (/api/v1/scheduling*)
	case strings.HasPrefix(clean, "/api/v1/scheduling"):
		if role == RoleReceptionist || role == RoleDentist {
			return true
		}
		if role == RoleAssistant {
			return m == http.MethodGet || m == http.MethodPatch
		}
		return false

	// 9. Billing, POS & Blind Cash Close (/api/v1/billing*)
	case strings.HasPrefix(clean, "/api/v1/billing"):
		if role == RoleReceptionist {
			return true
		}
		if role == RoleAuditor {
			return m == http.MethodGet
		}
		return false

	// 10. Specialist Settlements & Commissions (/api/v1/settlements*)
	case strings.HasPrefix(clean, "/api/v1/settlements"):
		if role == RoleAuditor || role == RoleDentist {
			return m == http.MethodGet
		}
		return false

	// 11. Clinical Inventory & Supplies (/api/v1/inventory*)
	case strings.HasPrefix(clean, "/api/v1/inventory"):
		if role == RoleAssistant {
			return true
		}
		if role == RoleDentist || role == RoleAuditor {
			return m == http.MethodGet
		}
		return false

	// 12. Immutable Audit Trail (/api/v1/audit*)
	case strings.HasPrefix(clean, "/api/v1/audit"):
		if role == RoleAuditor {
			return m == http.MethodGet
		}
		return false

	default:
		return false
	}
}

func isKnownAPIRoute(clean string) bool {
	prefixes := []string{
		"/api/v1/auth",
		"/api/v1/users",
		"/api/v1/patients",
		"/api/v1/clinical",
		"/api/v1/treatments",
		"/api/v1/treatment-plans",
		"/api/v1/consents",
		"/api/v1/scheduling",
		"/api/v1/billing",
		"/api/v1/settlements",
		"/api/v1/inventory",
		"/api/v1/audit",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(clean, p) {
			return true
		}
	}
	return false
}
