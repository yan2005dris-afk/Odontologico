package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sigo-dental/billing-stock/internal/billing/adapters/fiscal"
	billinghttp "github.com/sigo-dental/billing-stock/internal/billing/adapters/http"
	billingpg "github.com/sigo-dental/billing-stock/internal/billing/adapters/postgres"
	billingapp "github.com/sigo-dental/billing-stock/internal/billing/application"

	inventoryhttp "github.com/sigo-dental/billing-stock/internal/inventory/adapters/http"
	inventorypg "github.com/sigo-dental/billing-stock/internal/inventory/adapters/postgres"
	inventoryapp "github.com/sigo-dental/billing-stock/internal/inventory/application"

	settlementhttp "github.com/sigo-dental/billing-stock/internal/settlements/adapters/http"
	settlementpg "github.com/sigo-dental/billing-stock/internal/settlements/adapters/postgres"
	settlementapp "github.com/sigo-dental/billing-stock/internal/settlements/application"
)

func buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"billing-stock"}`))
	})

	// Billing context (POS, Electronic Invoicing RF-12, Blind Cash Close RN-02)
	billingRepo := billingpg.NewInMemoryBillingRepository()
	taxClient := fiscal.NewTaxAuthorityClient()
	payGateway := fiscal.NewPaymentGatewayClient()
	billingSvc := billingapp.NewBillingService(billingRepo, taxClient, payGateway)
	billingHandler := billinghttp.NewBillingHandler(billingSvc)
	billingHandler.RegisterRoutes(mux)

	// Settlements context (Dentist Commissions RF-13)
	settlementRepo := settlementpg.NewInMemorySettlementRepository()
	settlementSvc := settlementapp.NewSettlementService(settlementRepo)
	settlementHandler := settlementhttp.NewSettlementHandler(settlementSvc)
	settlementHandler.RegisterRoutes(mux)

	// Inventory context (Clinical Supplies & Automatic Deduction RF-14)
	inventoryRepo := inventorypg.NewInMemoryInventoryRepository()
	inventorySvc := inventoryapp.NewInventoryService(inventoryRepo)
	inventoryHandler := inventoryhttp.NewInventoryHandler(inventorySvc)
	inventoryHandler.RegisterRoutes(mux)

	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           buildMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("billing-stock service listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
