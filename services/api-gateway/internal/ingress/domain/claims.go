package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrMalformedToken     = errors.New("malformed JWT token")
	ErrUnsupportedAlg     = errors.New("unsupported or insecure JWT algorithm")
	ErrInvalidSignature   = errors.New("invalid JWT signature")
	ErrIdleSessionExpired = errors.New("session expired due to 15 minutes of inactivity")
	ErrMFARequired        = errors.New("multi-factor authentication is required for this resource")
	ErrForbiddenRole      = errors.New("insufficient role permission for this resource")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
)

// Error codes returned in standard JSON error envelopes (plan.md §5).
const (
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeSessionIdleTimeout = "SESSION_IDLE_TIMEOUT"
	CodeMFARequired        = "MFA_REQUIRED"
	CodeInsufficientRole   = "INSUFFICIENT_ROLE_PERMISSION"
	CodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
	CodeBadGateway         = "BAD_GATEWAY"
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// HS256Verifier verifies HMAC-SHA256 JWT tokens and rejects alg=none (RNF-05).
type HS256Verifier struct {
	secret []byte
	nowFn  func() time.Time
}

// NewHS256Verifier creates a new HS256 JWT verifier with the given secret key.
func NewHS256Verifier(secret string, nowFn func() time.Time) *HS256Verifier {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &HS256Verifier{
		secret: []byte(secret),
		nowFn:  nowFn,
	}
}

// VerifyToken parses, verifies signature in constant time, and validates Claims.
func (v *HS256Verifier) VerifyToken(tokenString string) (Claims, error) {
	parts := strings.Split(strings.TrimSpace(tokenString), ".")
	if len(parts) != 3 {
		return Claims{}, ErrMalformedToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrMalformedToken
	}

	var hdr jwtHeader
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return Claims{}, ErrMalformedToken
	}

	if hdr.Alg != "HS256" {
		return Claims{}, ErrUnsupportedAlg
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrMalformedToken
	}

	if !hmac.Equal(actualSig, expectedSig) {
		return Claims{}, ErrInvalidSignature
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrMalformedToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return Claims{}, ErrMalformedToken
	}

	if err := claims.Validate(v.nowFn()); err != nil {
		return Claims{}, err
	}

	return claims, nil
}

// SignHS256Token signs a Claims struct into a compact HS256 JWT string.
func SignHS256Token(claims Claims, secret string) (string, error) {
	hdrBytes, err := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(hdrBytes)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := encodedHeader + "." + encodedPayload

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig, nil
}
