package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/sigo-dental/api-gateway/internal/ingress/adapters/http"
	redisadapter "github.com/sigo-dental/api-gateway/internal/ingress/adapters/redis"
	"github.com/sigo-dental/api-gateway/internal/ingress/application"
	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

func main() {
	port := envOrDefault("PORT", "8080")
	redisAddr := envOrDefault("REDIS_ADDR", "")
	clinicalUpstream := envOrDefault("CLINICAL_AGENDA_UPSTREAM", "http://localhost:8081")
	billingUpstream := envOrDefault("BILLING_STOCK_UPSTREAM", "http://localhost:8082")
	jwtSecret := envOrDefault("JWT_SECRET", "sigo-dental-production-hs256-secret-key-2026")
	allowedOrigin := envOrDefault("ALLOWED_ORIGIN", "https://portal.sigo-dental.local")
	openAPIPath := envOrDefault("OPENAPI_SPEC_PATH", "../../openapi/sigo-dental.v1.yaml")
	tlsCert := os.Getenv("TLS_CERT_FILE")
	tlsKey := os.Getenv("TLS_KEY_FILE")

	verifier := domain.NewHS256Verifier(jwtSecret, time.Now)

	var sessionStore application.SessionStorePort
	if redisAddr != "" {
		sessionStore = redisadapter.NewRedisSessionStore(redisAddr, 2*time.Second)
	} else {
		sessionStore = redisadapter.NewInMemorySessionStore(time.Now)
	}

	rateLimiter := httpadapter.NewTokenBucketRateLimiter(300, 600, time.Now)
	telemetry := httpadapter.NewLatencyMetricsCollector()

	gatewayService := application.NewGatewayService(
		verifier,
		sessionStore,
		rateLimiter,
		domain.NewDefaultRBACPolicy(),
	)

	router, err := httpadapter.NewProxyRouter(gatewayService, telemetry, httpadapter.RouterConfig{
		ClinicalAgendaURL: clinicalUpstream,
		BillingStockURL:   billingUpstream,
		AllowedOrigin:     allowedOrigin,
		OpenAPISpecPath:   openAPIPath,
		UpstreamTimeout:   5 * time.Second,
	})
	if err != nil {
		log.Fatalf("failed to initialize proxy router: %v", err)
	}

	srv := httpadapter.NewTLS13Server(":"+port, router)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("SIGO-Dental API Gateway listening on :%s (TLS 1.3 ready)", port)
		var serveErr error
		if tlsCert != "" && tlsKey != "" {
			serveErr = srv.ListenAndServeTLS(tlsCert, tlsKey)
		} else {
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("gateway server error: %v", serveErr)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
