package http_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	httpadapter "github.com/sigo-dental/api-gateway/internal/ingress/adapters/http"
	redisadapter "github.com/sigo-dental/api-gateway/internal/ingress/adapters/redis"
	"github.com/sigo-dental/api-gateway/internal/ingress/application"
	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

// pathRecorder captures the last path seen by the upstream handler. The handler
// runs in its own goroutine, so the value is guarded by a mutex: without it the
// test would race with the handler under `-race`.
type pathRecorder struct {
	mu   sync.Mutex
	path string
}

func (r *pathRecorder) set(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = path
}

func (r *pathRecorder) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.path
}

// S10 (wi-13): the MFA challenge endpoint must be reachable without a Bearer
// token, because the client only holds the challengeToken at that point. Every
// other /api/v1 path keeps requiring a token.
func TestSpecS10_MfaVerifyIsPublic(t *testing.T) {
	now := time.Unix(1700000000, 0)

	recorder := &pathRecorder{}
	clinicalUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.set(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mfaVerified":true}`))
	}))
	defer clinicalUpstream.Close()

	router, err := httpadapter.NewProxyRouter(
		application.NewGatewayService(
			domain.NewHS256Verifier("s10-test-secret", func() time.Time { return now }),
			redisadapter.NewInMemorySessionStore(func() time.Time { return now }),
			httpadapter.NewTokenBucketRateLimiter(100, 100, func() time.Time { return now }),
			domain.NewDefaultRBACPolicy(),
		),
		httpadapter.NewLatencyMetricsCollector(),
		httpadapter.RouterConfig{
			ClinicalAgendaURL: clinicalUpstream.URL,
			BillingStockURL:   clinicalUpstream.URL,
			AllowedOrigin:     "https://portal.sigo-dental.local",
			OpenAPISpecPath:   filepath.Join("..", "..", "..", "..", "..", "..", "openapi", "sigo-dental.v1.yaml"),
		},
	)
	if err != nil {
		t.Fatalf("NewProxyRouter failed: %v", err)
	}

	gatewayServer := httptest.NewServer(router)
	defer gatewayServer.Close()

	t.Run("S10: mfa/verify reaches the upstream without a Bearer token", func(t *testing.T) {
		recorder.set("")
		req, _ := http.NewRequest(
			http.MethodPost,
			gatewayServer.URL+"/api/v1/auth/mfa/verify",
			strings.NewReader(`{"challengeToken":"challenge-abc","totpCode":"123456"}`),
		)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for public mfa/verify, got %d", resp.StatusCode)
		}
		if got := recorder.get(); got != "/api/v1/auth/mfa/verify" {
			t.Fatalf("expected mfa/verify forwarded to clinical upstream, got %q", got)
		}
	})

	t.Run("S10: protected paths still reject a tokenless request", func(t *testing.T) {
		recorder.set("")
		req, _ := http.NewRequest(http.MethodGet, gatewayServer.URL+"/api/v1/patients", nil)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for tokenless /api/v1/patients, got %d", resp.StatusCode)
		}
		if got := recorder.get(); got != "" {
			t.Fatalf("tokenless request must not reach the upstream, got %q", got)
		}
	})
}
