package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrorDetail represents the inner error payload (plan.md §5).
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// ErrorEnvelope is the canonical gateway JSON error format (plan.md §5).
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// WriteErrorJSON writes a standardized JSON error response with the given status code.
func WriteErrorJSON(w http.ResponseWriter, status int, code, message, requestID string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

// EnsureRequestID returns the incoming X-Request-ID or generates a new UUID v4 format identifier.
func EnsureRequestID(r *http.Request) string {
	if existing := strings.TrimSpace(r.Header.Get("X-Request-ID")); existing != "" {
		return existing
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// ExtractSourceIP normalizes the client source IP from X-Forwarded-For or RemoteAddr (RNF-06).
func ExtractSourceIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "127.0.0.1"
}

// SanitizeSpoofedHeaders strips untrusted client identity headers before edge injection (plan.md §6, RNF-06).
func SanitizeSpoofedHeaders(h http.Header) {
	h.Del("X-Actor-ID")
	h.Del("X-Actor-Role")
	h.Del("X-Session-ID")
	h.Del("X-MFA-Verified")
	h.Del("X-Source-IP")
}

// ApplySecurityHeaders attaches mandatory TLS 1.3 / HSTS / CORS / nosniff edge headers (RNF-04).
func ApplySecurityHeaders(w http.ResponseWriter, allowedOrigin string) {
	w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	if strings.TrimSpace(allowedOrigin) != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	}
}

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

// TokenBucketRateLimiter implements RateLimiterPort to sustain >= 150 concurrent users (RNF-03).
type TokenBucketRateLimiter struct {
	mu         sync.Mutex
	ratePerSec float64
	burst      float64
	nowFn      func() time.Time
	buckets    map[string]*clientBucket
}

// NewTokenBucketRateLimiter creates a per-key token bucket rate limiter.
func NewTokenBucketRateLimiter(ratePerSec, burst float64, nowFn func() time.Time) *TokenBucketRateLimiter {
	if nowFn == nil {
		nowFn = time.Now
	}
	if ratePerSec <= 0 {
		ratePerSec = 300
	}
	if burst <= 0 {
		burst = 600
	}
	return &TokenBucketRateLimiter{
		ratePerSec: ratePerSec,
		burst:      burst,
		nowFn:      nowFn,
		buckets:    make(map[string]*clientBucket),
	}
}

// Allow checks and consumes 1 token for key.
func (l *TokenBucketRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFn()
	b, exists := l.buckets[key]
	if !exists {
		l.buckets[key] = &clientBucket{
			tokens:     l.burst - 1,
			lastRefill: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.ratePerSec
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastRefill = now
	}

	if b.tokens < 1.0 {
		return false
	}
	b.tokens -= 1.0
	return true
}

type routeStats struct {
	count        uint64
	totalMs      float64
	withinSLACnt uint64
}

// LatencyMetricsCollector implements TelemetryPort to monitor the < 1.5s SLA (S6, RNF-01).
type LatencyMetricsCollector struct {
	mu     sync.Mutex
	routes map[string]*routeStats
}

// NewLatencyMetricsCollector constructs an in-memory Prometheus/OpenTelemetry-compatible latency collector.
func NewLatencyMetricsCollector() *LatencyMetricsCollector {
	return &LatencyMetricsCollector{
		routes: make(map[string]*routeStats),
	}
}

// RecordRequest records request duration in milliseconds and checks the 1500ms RNF-01 budget.
func (m *LatencyMetricsCollector) RecordRequest(route, method string, statusCode int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s|%s|%d", route, method, statusCode)
	st, ok := m.routes[key]
	if !ok {
		st = &routeStats{}
		m.routes[key] = st
	}
	ms := float64(duration.Microseconds()) / 1000.0
	st.count++
	st.totalMs += ms
	if ms < 1500.0 {
		st.withinSLACnt++
	}
}

// RenderPrometheus formats collected latency metrics in Prometheus text exposition format.
func (m *LatencyMetricsCollector) RenderPrometheus() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# HELP gateway_request_duration_ms Edge API Gateway request latency in milliseconds (RNF-01 SLA < 1500ms)\n")
	sb.WriteString("# TYPE gateway_request_duration_ms summary\n")
	for key, st := range m.routes {
		parts := strings.Split(key, "|")
		if len(parts) != 3 {
			continue
		}
		route, method, status := parts[0], parts[1], parts[2]
		sb.WriteString(fmt.Sprintf(
			"gateway_request_duration_ms_sum{route=%q,method=%q,status=%q} %.3f\n",
			route, method, status, st.totalMs,
		))
		sb.WriteString(fmt.Sprintf(
			"gateway_request_duration_ms_count{route=%q,method=%q,status=%q} %d\n",
			route, method, status, st.count,
		))
		sb.WriteString(fmt.Sprintf(
			"gateway_request_sla_under_1500ms_total{route=%q,method=%q,status=%q} %d\n",
			route, method, status, st.withinSLACnt,
		))
	}
	return sb.String()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
