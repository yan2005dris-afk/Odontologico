package application

import (
	"context"
	"strings"

	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

// UpstreamTarget identifies the downstream bounded-context microservice (S2, SRS §7).
type UpstreamTarget string

const (
	UpstreamUnknown        UpstreamTarget = ""
	UpstreamClinicalAgenda UpstreamTarget = "clinical-agenda"
	UpstreamBillingStock   UpstreamTarget = "billing-stock"
)

// IngressRequest encapsulates an incoming edge HTTP request for security evaluation.
type IngressRequest struct {
	Method    string
	Path      string
	Token     string
	SourceIP  string
	RequestID string
}

// IngressDecision holds the verified claims and target upstream after edge validation.
type IngressDecision struct {
	Claims   domain.Claims
	Upstream UpstreamTarget
}

// GatewayService orchestrates edge rate limiting, JWT verification, 15-min idle timeout, MFA, and RBAC.
type GatewayService struct {
	verifier   TokenVerifierPort
	sessions   SessionStorePort
	limiter    RateLimiterPort
	rbacPolicy *domain.RBACPolicy
}

// NewGatewayService constructs a GatewayService with its required ports.
func NewGatewayService(
	verifier TokenVerifierPort,
	sessions SessionStorePort,
	limiter RateLimiterPort,
	rbacPolicy *domain.RBACPolicy,
) *GatewayService {
	if rbacPolicy == nil {
		rbacPolicy = domain.NewDefaultRBACPolicy()
	}
	return &GatewayService{
		verifier:   verifier,
		sessions:   sessions,
		limiter:    limiter,
		rbacPolicy: rbacPolicy,
	}
}

// AuthorizeRequest executes the edge security pipeline (S2, S3, S4, S5).
func (s *GatewayService) AuthorizeRequest(ctx context.Context, req IngressRequest) (IngressDecision, error) {
	if s.limiter != nil {
		key := strings.TrimSpace(req.SourceIP)
		if key == "" {
			key = "anonymous"
		}
		if !s.limiter.Allow(key) {
			return IngressDecision{}, domain.ErrRateLimitExceeded
		}
	}

	if strings.TrimSpace(req.Token) == "" {
		return IngressDecision{}, domain.ErrMalformedToken
	}

	claims, err := s.verifier.VerifyToken(req.Token)
	if err != nil {
		return IngressDecision{}, err
	}

	// S3: Verify and refresh 15-minute (900s) sliding idle session timeout in Redis.
	if s.sessions != nil {
		ok, err := s.sessions.TouchSession(ctx, claims.SessionID, SessionIdleTTL)
		if err != nil || !ok {
			return IngressDecision{}, domain.ErrIdleSessionExpired
		}
	}

	// S4: Enforce mandatory MFA on clinical and financial routes.
	if domain.RequiresMFAForMethod(req.Method, req.Path) && !claims.MFAVerified {
		return IngressDecision{}, domain.ErrMFARequired
	}

	// S5: Enforce 5-role RBAC matrix at the edge.
	if !s.rbacPolicy.IsAllowed(claims.Role, req.Method, req.Path) {
		return IngressDecision{}, domain.ErrForbiddenRole
	}

	return IngressDecision{
		Claims:   claims,
		Upstream: ResolveUpstream(req.Path),
	}, nil
}

// ResolveUpstream maps an API v1 path to its owning downstream service (S2, SRS §7).
func ResolveUpstream(path string) UpstreamTarget {
	clean := strings.TrimRight(path, "/")
	switch {
	case strings.HasPrefix(clean, "/api/v1/auth"),
		strings.HasPrefix(clean, "/api/v1/users"),
		strings.HasPrefix(clean, "/api/v1/patients"),
		strings.HasPrefix(clean, "/api/v1/clinical"),
		strings.HasPrefix(clean, "/api/v1/treatments"),
		strings.HasPrefix(clean, "/api/v1/treatment-plans"),
		strings.HasPrefix(clean, "/api/v1/consents"),
		strings.HasPrefix(clean, "/api/v1/scheduling"),
		strings.HasPrefix(clean, "/api/v1/audit"):
		return UpstreamClinicalAgenda
	case strings.HasPrefix(clean, "/api/v1/billing"),
		strings.HasPrefix(clean, "/api/v1/settlements"),
		strings.HasPrefix(clean, "/api/v1/inventory"):
		return UpstreamBillingStock
	default:
		return UpstreamUnknown
	}
}
