package application

import (
	"context"
	"time"

	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

// SessionIdleTTL is the mandatory 15-minute (900s) idle session timeout (S3, RNF-05).
const SessionIdleTTL = 900 * time.Second

// TokenVerifierPort verifies cryptographic signatures and decodes Bearer JWT tokens.
type TokenVerifierPort interface {
	VerifyToken(tokenString string) (domain.Claims, error)
}

// SessionStorePort tracks sliding idle session state in Redis ("session:idle:<session_id>") (S3, RNF-05).
type SessionStorePort interface {
	// TouchSession checks if session:idle:<sessionID> exists and refreshes its TTL to ttl.
	// Returns ok=false if the session does not exist or has expired.
	TouchSession(ctx context.Context, sessionID string, ttl time.Duration) (ok bool, err error)
}

// RateLimiterPort enforces per-client/IP rate limiting to sustain >= 150 concurrent users (RNF-03).
type RateLimiterPort interface {
	Allow(key string) bool
}

// TelemetryPort records edge request latency to monitor the < 1.5s SLA (S6, RNF-01).
type TelemetryPort interface {
	RecordRequest(route string, method string, statusCode int, duration time.Duration)
}
