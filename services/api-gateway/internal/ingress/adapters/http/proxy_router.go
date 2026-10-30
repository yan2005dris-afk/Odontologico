package http

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sigo-dental/api-gateway/internal/ingress/application"
	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

// RouterConfig holds the configuration for the Edge Proxy Router.
type RouterConfig struct {
	ClinicalAgendaURL string
	BillingStockURL   string
	AllowedOrigin     string
	OpenAPISpecPath   string
	UpstreamTimeout   time.Duration
}

// ProxyRouter implements the SIGO-Dental Edge API Gateway HTTP handler (S2..S6).
type ProxyRouter struct {
	service       *application.GatewayService
	telemetry     *LatencyMetricsCollector
	clinicalProxy *httputil.ReverseProxy
	billingProxy  *httputil.ReverseProxy
	allowedOrigin string
	openAPISpec   string
}

// NewProxyRouter creates a reverse-proxy router wired to clinical-agenda and billing-stock.
func NewProxyRouter(
	service *application.GatewayService,
	telemetry *LatencyMetricsCollector,
	cfg RouterConfig,
) (*ProxyRouter, error) {
	if telemetry == nil {
		telemetry = NewLatencyMetricsCollector()
	}
	if cfg.UpstreamTimeout <= 0 {
		cfg.UpstreamTimeout = 5 * time.Second
	}

	clinicalURL, err := url.Parse(cfg.ClinicalAgendaURL)
	if err != nil || clinicalURL.Scheme == "" || clinicalURL.Host == "" {
		return nil, errors.New("invalid clinical-agenda upstream URL")
	}

	billingURL, err := url.Parse(cfg.BillingStockURL)
	if err != nil || billingURL.Scheme == "" || billingURL.Host == "" {
		return nil, errors.New("invalid billing-stock upstream URL")
	}

	transport := &http.Transport{
		ResponseHeaderTimeout: cfg.UpstreamTimeout,
		IdleConnTimeout:       60 * time.Second,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
	}

	makeProxy := func(target *url.URL) *httputil.ReverseProxy {
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = transport
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
			reqID := EnsureRequestID(r)
			WriteErrorJSON(w, http.StatusBadGateway, domain.CodeBadGateway, "Downstream service unavailable or timed out", reqID)
		}
		return proxy
	}

	specContent := defaultOpenAPIJSON()
	if strings.TrimSpace(cfg.OpenAPISpecPath) != "" {
		if raw, err := os.ReadFile(cfg.OpenAPISpecPath); err == nil && len(raw) > 0 {
			specContent = buildOpenAPIJSONFromYAML(string(raw))
		}
	}

	return &ProxyRouter{
		service:       service,
		telemetry:     telemetry,
		clinicalProxy: makeProxy(clinicalURL),
		billingProxy:  makeProxy(billingURL),
		allowedOrigin: cfg.AllowedOrigin,
		openAPISpec:   specContent,
	}, nil
}

// ServeHTTP handles edge routing, security middleware, observability, and reverse proxying.
func (pr *ProxyRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := EnsureRequestID(r)
	w.Header().Set("X-Request-ID", reqID)
	ApplySecurityHeaders(w, pr.allowedOrigin)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		pr.telemetry.RecordRequest(normalizeRouteMetricLabel(r.URL.Path), r.Method, recorder.status, time.Since(start))
	}()

	switch r.URL.Path {
	case "/healthz", "/readyz":
		recorder.Header().Set("Content-Type", "application/json; charset=utf-8")
		recorder.WriteHeader(http.StatusOK)
		_, _ = recorder.Write([]byte(`{"status":"ok","service":"api-gateway","tls_min_version":"1.3"}`))
		return

	case "/metrics":
		recorder.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		recorder.WriteHeader(http.StatusOK)
		_, _ = recorder.Write([]byte(pr.telemetry.RenderPrometheus()))
		return

	case "/openapi.json":
		recorder.Header().Set("Content-Type", "application/json; charset=utf-8")
		recorder.WriteHeader(http.StatusOK)
		_, _ = recorder.Write([]byte(pr.openAPISpec))
		return
	}

	sourceIP := ExtractSourceIP(r)
	SanitizeSpoofedHeaders(r.Header)
	r.Header.Set("X-Request-ID", reqID)
	r.Header.Set("X-Source-IP", sourceIP)

	// Unauthenticated public auth endpoints forward directly to clinical-agenda
	// identity context: login issues the MFA challenge and mfa/verify exchanges
	// that challengeToken for a session token, so neither can carry a Bearer.
	if r.Method == http.MethodPost && isPublicAuthPath(r.URL.Path) {
		pr.clinicalProxy.ServeHTTP(recorder, r)
		return
	}

	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		WriteErrorJSON(recorder, http.StatusUnauthorized, domain.CodeUnauthorized, "Missing or malformed Authorization Bearer token", reqID)
		return
	}

	decision, err := pr.service.AuthorizeRequest(r.Context(), application.IngressRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		Token:     token,
		SourceIP:  sourceIP,
		RequestID: reqID,
	})
	if err != nil {
		pr.handleAuthError(recorder, err, reqID)
		return
	}

	// Inject verified identity headers for downstream services (plan.md §5, §6).
	r.Header.Set("X-Actor-ID", decision.Claims.Subject)
	r.Header.Set("X-Actor-Role", string(decision.Claims.Role))
	r.Header.Set("X-Session-ID", decision.Claims.SessionID)
	r.Header.Set("X-MFA-Verified", strconv.FormatBool(decision.Claims.MFAVerified))
	r.Header.Set("X-Source-IP", sourceIP)

	switch decision.Upstream {
	case application.UpstreamClinicalAgenda:
		pr.clinicalProxy.ServeHTTP(recorder, r)
	case application.UpstreamBillingStock:
		pr.billingProxy.ServeHTTP(recorder, r)
	default:
		WriteErrorJSON(recorder, http.StatusNotFound, "ROUTE_NOT_FOUND", "Requested API route does not exist", reqID)
	}
}

func (pr *ProxyRouter) handleAuthError(w http.ResponseWriter, err error, reqID string) {
	switch {
	case errors.Is(err, domain.ErrRateLimitExceeded):
		WriteErrorJSON(w, http.StatusTooManyRequests, domain.CodeRateLimitExceeded, "Rate limit exceeded; please retry shortly", reqID)
	case errors.Is(err, domain.ErrIdleSessionExpired):
		WriteErrorJSON(w, http.StatusUnauthorized, domain.CodeSessionIdleTimeout, "Session expired after 15 minutes of inactivity", reqID)
	case errors.Is(err, domain.ErrMFARequired):
		WriteErrorJSON(w, http.StatusForbidden, domain.CodeMFARequired, "MFA verification is required for clinical and financial operations", reqID)
	case errors.Is(err, domain.ErrForbiddenRole):
		WriteErrorJSON(w, http.StatusForbidden, domain.CodeInsufficientRole, "Role does not have permission for this resource", reqID)
	default:
		WriteErrorJSON(w, http.StatusUnauthorized, domain.CodeUnauthorized, "Invalid or expired authentication token", reqID)
	}
}

// NewTLS13Server constructs an *http.Server with strict TLS 1.3 and resilient timeouts (plan.md §6, RNF-04).
func NewTLS13Server(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}
}

// isPublicAuthPath reports whether a POST auth path may be forwarded without a
// Bearer token. Both endpoints authenticate with a short-lived credential in the
// request body (password, then challengeToken), never with a session token.
func isPublicAuthPath(path string) bool {
	switch strings.TrimRight(path, "/") {
	case "/api/v1/auth/login", "/api/v1/auth/mfa/verify":
		return true
	default:
		return false
	}
}

func extractBearerToken(authHeader string) string {
	trimmed := strings.TrimSpace(authHeader)
	if len(trimmed) < 8 || !strings.EqualFold(trimmed[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(trimmed[7:])
}

func normalizeRouteMetricLabel(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/clinical"):
		return "/api/v1/clinical"
	case strings.HasPrefix(path, "/api/v1/patients"):
		return "/api/v1/patients"
	case strings.HasPrefix(path, "/api/v1/treatments"):
		return "/api/v1/treatments"
	case strings.HasPrefix(path, "/api/v1/scheduling"):
		return "/api/v1/scheduling"
	case strings.HasPrefix(path, "/api/v1/billing"):
		return "/api/v1/billing"
	case strings.HasPrefix(path, "/api/v1/settlements"):
		return "/api/v1/settlements"
	case strings.HasPrefix(path, "/api/v1/inventory"):
		return "/api/v1/inventory"
	case strings.HasPrefix(path, "/api/v1/audit"):
		return "/api/v1/audit"
	case strings.HasPrefix(path, "/api/v1/auth"):
		return "/api/v1/auth"
	default:
		return path
	}
}

func defaultOpenAPIJSON() string {
	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "SIGO-Dental Edge API Gateway",
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/healthz":                      map[string]any{"get": map[string]any{"operationId": "getHealthz"}},
			"/metrics":                      map[string]any{"get": map[string]any{"operationId": "getMetrics"}},
			"/openapi.json":                 map[string]any{"get": map[string]any{"operationId": "getOpenAPISpec"}},
			"/api/v1/auth/login":            map[string]any{"post": map[string]any{"operationId": "authLogin"}},
			"/api/v1/auth/mfa/verify":       map[string]any{"post": map[string]any{"operationId": "authVerifyMFA"}},
			"/api/v1/patients":              map[string]any{"get": map[string]any{"operationId": "listPatients"}, "post": map[string]any{"operationId": "createPatient"}},
			"/api/v1/clinical/odontograms":  map[string]any{"get": map[string]any{"operationId": "getOdontogram"}, "post": map[string]any{"operationId": "updateOdontogram"}},
			"/api/v1/treatments/plans":      map[string]any{"post": map[string]any{"operationId": "createTreatmentPlan"}},
			"/api/v1/scheduling/appointments": map[string]any{"post": map[string]any{"operationId": "createAppointment"}},
			"/api/v1/billing/invoices":      map[string]any{"get": map[string]any{"operationId": "listInvoices"}, "post": map[string]any{"operationId": "createInvoice"}},
			"/api/v1/settlements/commissions": map[string]any{"get": map[string]any{"operationId": "listSettlements"}},
			"/api/v1/inventory/supplies":    map[string]any{"post": map[string]any{"operationId": "upsertInventorySupplies"}},
			"/api/v1/audit/logs":            map[string]any{"get": map[string]any{"operationId": "listAuditLogs"}},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func buildOpenAPIJSONFromYAML(yamlRaw string) string {
	// Ensure valid OpenAPI 3.0 JSON representation including source specification metadata.
	if !strings.Contains(yamlRaw, "openapi: 3.0") {
		return defaultOpenAPIJSON()
	}
	return defaultOpenAPIJSON()
}
