package domain

import (
	"errors"
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
func RequiresMFA(path string) bool {
	clean := strings.TrimRight(path, "/")
	if clean == "" {
		return false
	}
	if strings.HasPrefix(clean, "/api/v1/clinical") ||
		strings.HasPrefix(clean, "/api/v1/treatments") ||
		strings.HasPrefix(clean, "/api/v1/billing") ||
		strings.HasPrefix(clean, "/api/v1/settlements") {
		return true
	}
	if strings.HasPrefix(clean, "/api/v1/patients/") && strings.HasSuffix(clean, "/anamnesis") {
		return true
	}
	return false
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

	// Auth routes (e.g., /api/v1/auth/mfa/verify, /api/v1/auth/logout) are accessible to all authenticated roles.
	if strings.HasPrefix(clean, "/api/v1/auth") {
		return true
	}

	// Admin has full administrative and system route access.
	if role == RoleAdmin {
		return isKnownAPIRoute(clean)
	}

	switch {
	// 1. Clinical workspace (/api/v1/clinical/*)
	case strings.HasPrefix(clean, "/api/v1/clinical"):
		// Dentist and Assistant can access clinical workspace; Receptionist and Auditor cannot.
		return role == RoleDentist || role == RoleAssistant

	// 2. Patient Anamnesis (/api/v1/patients/*/anamnesis) — clinical history restricted from receptionist/auditor
	case strings.HasPrefix(clean, "/api/v1/patients/") && strings.HasSuffix(clean, "/anamnesis"):
		return role == RoleDentist || role == RoleAssistant

	// 3. Unified Patient Demographic Record (/api/v1/patients*)
	case strings.HasPrefix(clean, "/api/v1/patients"):
		if role == RoleReceptionist || role == RoleDentist {
			return true
		}
		if role == RoleAssistant {
			return m == "GET"
		}
		return false

	// 4. Treatment Plans & Budgets (/api/v1/treatments*)
	case strings.HasPrefix(clean, "/api/v1/treatments"):
		if role == RoleDentist {
			return true
		}
		if role == RoleReceptionist || role == RoleAssistant {
			return m == "GET"
		}
		return false

	// 5. Scheduling & Multi-Chair Calendar (/api/v1/scheduling*)
	case strings.HasPrefix(clean, "/api/v1/scheduling"):
		if role == RoleReceptionist || role == RoleDentist {
			return true
		}
		if role == RoleAssistant {
			return m == "GET" || m == "PATCH"
		}
		return false

	// 6. Billing, POS & Blind Cash Close (/api/v1/billing*)
	case strings.HasPrefix(clean, "/api/v1/billing"):
		if role == RoleReceptionist {
			return true
		}
		if role == RoleAuditor {
			return m == "GET"
		}
		return false

	// 7. Specialist Settlements & Commissions (/api/v1/settlements*)
	case strings.HasPrefix(clean, "/api/v1/settlements"):
		if role == RoleAuditor || role == RoleDentist {
			return m == "GET"
		}
		return false

	// 8. Clinical Inventory & Supplies (/api/v1/inventory*)
	case strings.HasPrefix(clean, "/api/v1/inventory"):
		if role == RoleAssistant {
			return true
		}
		if role == RoleDentist || role == RoleAuditor {
			return m == "GET"
		}
		return false

	// 9. Immutable Audit Trail (/api/v1/audit*)
	case strings.HasPrefix(clean, "/api/v1/audit"):
		if role == RoleAuditor {
			return m == "GET"
		}
		return false

	default:
		return false
	}
}

func isKnownAPIRoute(clean string) bool {
	prefixes := []string{
		"/api/v1/auth",
		"/api/v1/patients",
		"/api/v1/clinical",
		"/api/v1/treatments",
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
