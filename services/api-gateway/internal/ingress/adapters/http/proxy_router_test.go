package http_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/sigo-dental/api-gateway/internal/ingress/adapters/http"
	redisadapter "github.com/sigo-dental/api-gateway/internal/ingress/adapters/redis"
	"github.com/sigo-dental/api-gateway/internal/ingress/application"
	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

func TestSpecS1_ComposeTopologyAndTLS13(t *testing.T) {
	composePath := filepath.Join("..", "..", "..", "..", "..", "..", "deploy", "compose.yaml")
	raw, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read deploy/compose.yaml: %v", err)
	}
	content := string(raw)

	requiredServices := []string{
		"postgres:",
		"redis:",
		"rustfs:",
		"api-gateway:",
		"clinical-agenda:",
		"billing-stock:",
		"frontend:",
	}
	for _, svc := range requiredServices {
		if !strings.Contains(content, svc) {
			t.Errorf("S1: deploy/compose.yaml missing service definition %q", svc)
		}
	}

	if !strings.Contains(content, "healthcheck:") {
		t.Errorf("S1: deploy/compose.yaml missing healthcheck definitions")
	}
	if !strings.Contains(content, "internal: true") {
		t.Errorf("S1: deploy/compose.yaml missing isolated internal network")
	}
	if !strings.Contains(content, `TLS_MIN_VERSION: "1.3"`) {
		t.Errorf("S1: deploy/compose.yaml missing TLS 1.3 enforcement")
	}
	if !strings.Contains(content, "S3_ENDPOINT: http://rustfs:9000") {
		t.Errorf("S1: deploy/compose.yaml missing S3 endpoint bound to the rustfs service")
	}

	srv := httpadapter.NewTLS13Server(":8443", http.NotFoundHandler())
	if srv.TLSConfig == nil || srv.TLSConfig.MinVersion != tls.VersionTLS13 {
		t.Fatalf("S1: expected TLSConfig.MinVersion == tls.VersionTLS13")
	}
}

func TestProxyRouter_SpecsS2ToS6(t *testing.T) {
	now := time.Unix(1700000000, 0)
	currentNow := now
	secret := "integration-test-hs256-secret-key"

	var lastClinicalHeaders http.Header
	var lastClinicalPath string
	var lastClinicalQuery string
	clinicalUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastClinicalHeaders = r.Header.Clone()
		lastClinicalPath = r.URL.Path
		lastClinicalQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"upstream":"clinical-agenda"}`))
	}))
	defer clinicalUpstream.Close()

	var lastBillingHeaders http.Header
	var lastBillingPath string
	billingUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBillingHeaders = r.Header.Clone()
		lastBillingPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upstream":"billing-stock"}`))
	}))
	defer billingUpstream.Close()

	verifier := domain.NewHS256Verifier(secret, func() time.Time { return currentNow })
	sessionStore := redisadapter.NewInMemorySessionStore(func() time.Time { return currentNow })
	rateLimiter := httpadapter.NewTokenBucketRateLimiter(100, 100, func() time.Time { return currentNow })
	telemetry := httpadapter.NewLatencyMetricsCollector()

	gatewaySvc := application.NewGatewayService(
		verifier,
		sessionStore,
		rateLimiter,
		domain.NewDefaultRBACPolicy(),
	)

	openAPIPath := filepath.Join("..", "..", "..", "..", "..", "..", "openapi", "sigo-dental.v1.yaml")
	router, err := httpadapter.NewProxyRouter(gatewaySvc, telemetry, httpadapter.RouterConfig{
		ClinicalAgendaURL: clinicalUpstream.URL,
		BillingStockURL:   billingUpstream.URL,
		AllowedOrigin:     "https://portal.sigo-dental.local",
		OpenAPISpecPath:   openAPIPath,
	})
	if err != nil {
		t.Fatalf("NewProxyRouter failed: %v", err)
	}

	gatewayServer := httptest.NewServer(router)
	defer gatewayServer.Close()

	// S2: Reverse proxy routing + anti-spoofing header injection
	t.Run("S2: routes clinical and billing contexts and strips spoofed X-Actor headers", func(t *testing.T) {
		sessionStore.RegisterSession("sess-dentist-1", 900*time.Second)
		dentistToken, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "dentist-uuid-42",
			SessionID:   "sess-dentist-1",
			Role:        domain.RoleDentist,
			MFAVerified: true,
			ExpiresAt:   currentNow.Add(30 * time.Minute).Unix(),
		}, secret)

		req, _ := http.NewRequest(http.MethodGet, gatewayServer.URL+"/api/v1/clinical/odontograms?notation=FDI", nil)
		req.Header.Set("Authorization", "Bearer "+dentistToken)
		req.Header.Set("X-Actor-ID", "spoofed-attacker-id")
		req.Header.Set("X-Actor-Role", "admin")
		req.Header.Set("X-MFA-Verified", "false")
		req.Header.Set("X-Forwarded-For", "203.0.113.15, 10.0.0.1")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		if lastClinicalPath != "/api/v1/clinical/odontograms" || lastClinicalQuery != "notation=FDI" {
			t.Fatalf("unexpected upstream path/query: %s?%s", lastClinicalPath, lastClinicalQuery)
		}
		if got := lastClinicalHeaders.Get("X-Actor-ID"); got != "dentist-uuid-42" {
			t.Fatalf("anti-spoofing failed: X-Actor-ID = %q", got)
		}
		if got := lastClinicalHeaders.Get("X-Actor-Role"); got != "dentist" {
			t.Fatalf("anti-spoofing failed: X-Actor-Role = %q", got)
		}
		if got := lastClinicalHeaders.Get("X-MFA-Verified"); got != "true" {
			t.Fatalf("expected X-MFA-Verified=true, got %q", got)
		}
		if got := lastClinicalHeaders.Get("X-Source-IP"); got != "203.0.113.15" {
			t.Fatalf("expected X-Source-IP=203.0.113.15, got %q", got)
		}

		// Route to billing-stock
		sessionStore.RegisterSession("sess-recep-1", 900*time.Second)
		recepToken, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "recep-uuid-07",
			SessionID:   "sess-recep-1",
			Role:        domain.RoleReceptionist,
			MFAVerified: true,
			ExpiresAt:   currentNow.Add(30 * time.Minute).Unix(),
		}, secret)

		bReq, _ := http.NewRequest(http.MethodPost, gatewayServer.URL+"/api/v1/billing/invoices", strings.NewReader(`{"amount":120}`))
		bReq.Header.Set("Authorization", "Bearer "+recepToken)
		bResp, err := http.DefaultClient.Do(bReq)
		if err != nil {
			t.Fatalf("billing request failed: %v", err)
		}
		defer bResp.Body.Close()
		if bResp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created from billing upstream, got %d", bResp.StatusCode)
		}
		if lastBillingPath != "/api/v1/billing/invoices" {
			t.Fatalf("expected /api/v1/billing/invoices, got %s", lastBillingPath)
		}
		if lastBillingHeaders.Get("X-Actor-ID") != "recep-uuid-07" {
			t.Fatalf("unexpected billing X-Actor-ID: %s", lastBillingHeaders.Get("X-Actor-ID"))
		}
	})

	// S3: 15-minute idle session timeout
	t.Run("S3: session idle > 900s returns 401 SESSION_IDLE_TIMEOUT", func(t *testing.T) {
		sessionStore.RegisterSession("sess-idle-test", 900*time.Second)
		tok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "dentist-idle",
			SessionID:   "sess-idle-test",
			Role:        domain.RoleDentist,
			MFAVerified: true,
			ExpiresAt:   currentNow.Add(2 * time.Hour).Unix(),
		}, secret)

		// Advance clock by 15 minutes + 1 second without activity
		currentNow = currentNow.Add(901 * time.Second)
		defer func() { currentNow = now }()

		req, _ := http.NewRequest(http.MethodGet, gatewayServer.URL+"/api/v1/patients", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", resp.StatusCode)
		}
		var env httpadapter.ErrorEnvelope
		_ = json.NewDecoder(resp.Body).Decode(&env)
		if env.Error.Code != domain.CodeSessionIdleTimeout {
			t.Fatalf("expected error code %s, got %s", domain.CodeSessionIdleTimeout, env.Error.Code)
		}
	})

	// S4: Mandatory MFA on clinical and financial routes
	t.Run("S4: mfa_verified=false on clinical/financial routes returns 403 MFA_REQUIRED", func(t *testing.T) {
		sessionStore.RegisterSession("sess-no-mfa", 900*time.Second)
		tok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "dentist-no-mfa",
			SessionID:   "sess-no-mfa",
			Role:        domain.RoleDentist,
			MFAVerified: false,
			ExpiresAt:   currentNow.Add(30 * time.Minute).Unix(),
		}, secret)

		protectedPaths := []string{
			"/api/v1/clinical/odontograms",
			"/api/v1/patients/p-123/anamnesis",
			"/api/v1/treatments/plans",
			"/api/v1/billing/invoices",
			"/api/v1/settlements/commissions",
		}
		for _, p := range protectedPaths {
			req, _ := http.NewRequest(http.MethodGet, gatewayServer.URL+p, nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed for %s: %v", p, err)
			}
			var env httpadapter.ErrorEnvelope
			_ = json.NewDecoder(resp.Body).Decode(&env)
			resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden || env.Error.Code != domain.CodeMFARequired {
				t.Fatalf("path %s: expected 403 MFA_REQUIRED, got %d (%s)", p, resp.StatusCode, env.Error.Code)
			}
		}
	})

	// S5: 5-Role RBAC matrix at the edge
	t.Run("S5: receptionist on odontograms and assistant on billing return 403 INSUFFICIENT_ROLE_PERMISSION", func(t *testing.T) {
		sessionStore.RegisterSession("sess-rbac-recep", 900*time.Second)
		recepTok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "recep-rbac",
			SessionID:   "sess-rbac-recep",
			Role:        domain.RoleReceptionist,
			MFAVerified: true,
			ExpiresAt:   currentNow.Add(30 * time.Minute).Unix(),
		}, secret)

		req1, _ := http.NewRequest(http.MethodPost, gatewayServer.URL+"/api/v1/clinical/odontograms", nil)
		req1.Header.Set("Authorization", "Bearer "+recepTok)
		resp1, _ := http.DefaultClient.Do(req1)
		var env1 httpadapter.ErrorEnvelope
		_ = json.NewDecoder(resp1.Body).Decode(&env1)
		resp1.Body.Close()

		if resp1.StatusCode != http.StatusForbidden || env1.Error.Code != domain.CodeInsufficientRole {
			t.Fatalf("expected 403 INSUFFICIENT_ROLE_PERMISSION for receptionist, got %d (%s)", resp1.StatusCode, env1.Error.Code)
		}

		sessionStore.RegisterSession("sess-rbac-asst", 900*time.Second)
		asstTok, _ := domain.SignHS256Token(domain.Claims{
			Subject:     "asst-rbac",
			SessionID:   "sess-rbac-asst",
			Role:        domain.RoleAssistant,
			MFAVerified: true,
			ExpiresAt:   currentNow.Add(30 * time.Minute).Unix(),
		}, secret)

		req2, _ := http.NewRequest(http.MethodPost, gatewayServer.URL+"/api/v1/billing/invoices", nil)
		req2.Header.Set("Authorization", "Bearer "+asstTok)
		resp2, _ := http.DefaultClient.Do(req2)
		var env2 httpadapter.ErrorEnvelope
		_ = json.NewDecoder(resp2.Body).Decode(&env2)
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusForbidden || env2.Error.Code != domain.CodeInsufficientRole {
			t.Fatalf("expected 403 INSUFFICIENT_ROLE_PERMISSION for assistant, got %d (%s)", resp2.StatusCode, env2.Error.Code)
		}
	})

	// S6: Observability (/healthz, /metrics SLA < 1.5s, /openapi.json)
	t.Run("S6: healthz, metrics (gateway_request_duration_ms), and openapi.json", func(t *testing.T) {
		hResp, err := http.Get(gatewayServer.URL + "/healthz")
		if err != nil || hResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK on /healthz, got %v", err)
		}
		hResp.Body.Close()

		oResp, err := http.Get(gatewayServer.URL + "/openapi.json")
		if err != nil || oResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK on /openapi.json, got %v", err)
		}
		var openAPIDoc map[string]any
		if err := json.NewDecoder(oResp.Body).Decode(&openAPIDoc); err != nil {
			t.Fatalf("invalid JSON from /openapi.json: %v", err)
		}
		oResp.Body.Close()
		if openAPIDoc["openapi"] != "3.0.3" {
			t.Fatalf("expected openapi 3.0.3, got %v", openAPIDoc["openapi"])
		}

		mResp, err := http.Get(gatewayServer.URL + "/metrics")
		if err != nil || mResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK on /metrics, got %v", err)
		}
		scanner := bufio.NewScanner(mResp.Body)
		var metricsText strings.Builder
		for scanner.Scan() {
			metricsText.WriteString(scanner.Text() + "\n")
		}
		mResp.Body.Close()
		if !strings.Contains(metricsText.String(), "gateway_request_duration_ms") {
			t.Fatalf("expected gateway_request_duration_ms in /metrics output, got:\n%s", metricsText.String())
		}
	})
}

func TestRedisSessionStore_RESPProtocol(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 512)
				n, _ := c.Read(buf)
				req := string(buf[:n])
				if strings.Contains(req, "session:idle:active-sid") {
					_, _ = c.Write([]byte(":1\r\n"))
				} else {
					_, _ = c.Write([]byte(":0\r\n"))
				}
			}(conn)
		}
	}()

	store := redisadapter.NewRedisSessionStore(ln.Addr().String(), time.Second)
	ok, err := store.TouchSession(context.Background(), "active-sid", 900*time.Second)
	if err != nil || !ok {
		t.Fatalf("expected ok=true for active-sid, got ok=%v err=%v", ok, err)
	}

	ok2, err := store.TouchSession(context.Background(), "expired-sid", 900*time.Second)
	if err != nil || ok2 {
		t.Fatalf("expected ok=false for expired-sid, got ok=%v err=%v", ok2, err)
	}

	memStore := redisadapter.NewInMemorySessionStore(nil)
	memStore.RegisterSession("s-revoke", 900*time.Second)
	memStore.RevokeSession("s-revoke")
	if ok3, _ := memStore.TouchSession(context.Background(), "s-revoke", 900*time.Second); ok3 {
		t.Fatalf("expected revoked session to return ok=false")
	}
}

func TestEdgeCases_OptionsLoginAndRateLimiter(t *testing.T) {
	now := time.Unix(1700000000, 0)
	secret := "edge-test-secret"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	verifier := domain.NewHS256Verifier(secret, func() time.Time { return now })
	sessions := redisadapter.NewInMemorySessionStore(func() time.Time { return now })
	limiter := httpadapter.NewTokenBucketRateLimiter(1, 1, func() time.Time { return now })

	svc := application.NewGatewayService(verifier, sessions, limiter, nil)
	router, err := httpadapter.NewProxyRouter(svc, nil, httpadapter.RouterConfig{
		ClinicalAgendaURL: upstream.URL,
		BillingStockURL:   upstream.URL,
		AllowedOrigin:     "https://portal.sigo-dental.local",
	})
	if err != nil {
		t.Fatalf("unexpected router error: %v", err)
	}

	// OPTIONS preflight
	optReq := httptest.NewRequest(http.MethodOptions, "/api/v1/patients", nil)
	optRec := httptest.NewRecorder()
	router.ServeHTTP(optRec, optReq)
	if optRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", optRec.Code)
	}

	// Public POST /api/v1/auth/login
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/auth/login, got %d", loginRec.Code)
	}

	// Missing Authorization header
	noAuthReq := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	noAuthRec := httptest.NewRecorder()
	router.ServeHTTP(noAuthRec, noAuthReq)
	if noAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", noAuthRec.Code)
	}

	// Invalid upstream config check
	if _, err := httpadapter.NewProxyRouter(svc, nil, httpadapter.RouterConfig{ClinicalAgendaURL: "://bad"}); err == nil {
		t.Fatalf("expected error on invalid clinical upstream URL")
	}
	if _, err := httpadapter.NewProxyRouter(svc, nil, httpadapter.RouterConfig{
		ClinicalAgendaURL: upstream.URL,
		BillingStockURL:   "://bad",
	}); err == nil {
		t.Fatalf("expected error on invalid billing upstream URL")
	}
}
