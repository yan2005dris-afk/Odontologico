package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	audithttp "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/http"
	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	clinicalhttp "github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/http"
	clinicalpg "github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/postgres"
	clinicals3 "github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/s3"
	clinicalapp "github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	identityhttp "github.com/sigo-dental/clinical-agenda/internal/identity/adapters/http"
	identitypg "github.com/sigo-dental/clinical-agenda/internal/identity/adapters/postgres"
	identityredis "github.com/sigo-dental/clinical-agenda/internal/identity/adapters/redis"
	identityapp "github.com/sigo-dental/clinical-agenda/internal/identity/application"
	identitydom "github.com/sigo-dental/clinical-agenda/internal/identity/domain"
	patienthttp "github.com/sigo-dental/clinical-agenda/internal/patients/adapters/http"
	patientpg "github.com/sigo-dental/clinical-agenda/internal/patients/adapters/postgres"
	patientapp "github.com/sigo-dental/clinical-agenda/internal/patients/application"
	schedulinghttp "github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/http"
	schedulingmsg "github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/messaging"
	schedulingpg "github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/postgres"
	schedulingapp "github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	treatmenthttp "github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/http"
	treatmentpg "github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/postgres"
	treatmentapp "github.com/sigo-dental/clinical-agenda/internal/treatments/application"
)

// idleSessionRegistrar returns the adapter that publishes the idle-session
// marker of a new session in the shared session store (Redis). The edge
// gateway refreshes that marker on every authenticated request and rejects the
// request when it is gone (RNF-05). Without Redis the service still runs, but
// the gateway cannot validate the session of a new login.
func idleSessionRegistrar() identityapp.IdleSessionRegistrar {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if addr == "" {
		return nil
	}
	return identityredis.NewIdleSessionRegistrar(addr, 2*time.Second)
}

func buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	// 1. Audit & Identity
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)
	auditHandler := audithttp.NewAuditHandler(auditSvc)
	auditHandler.RegisterRoutes(mux)

	userRepo := identitypg.NewUserRepository()
	defaultDentist, err := identitydom.NewUser(
		"c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		"dra.gomez@sigo-dental.com",
		"Dra. Valeria Gómez",
		identitydom.RoleDentist,
		"ClinicalPass#2026",
		"JBSWY3DPEHPK3PXP",
		time.Now().UTC(),
	)
	if err == nil {
		_ = userRepo.CreateUser(context.Background(), defaultDentist)
	}
	authSvc := identityapp.NewAuthService(userRepo, auditSvc, nil, os.Getenv("JWT_SECRET"), idleSessionRegistrar())
	authHandler := identityhttp.NewAuthHandler(authSvc)
	authHandler.RegisterRoutes(mux)

	// 2. Patients & Anamnesis
	patientRepo := patientpg.NewPatientRepository()
	patientSvc := patientapp.NewPatientService(patientRepo, auditSvc, nil)
	patientHandler := patienthttp.NewPatientHandler(patientSvc)
	patientHandler.RegisterRoutes(mux)

	// 3. Clinical: Odontogram, Periodontogram, SOEP & Digital Consents
	odoRepo := clinicalpg.NewOdontogramRepository()
	perioRepo := clinicalpg.NewPeriodontogramRepository()
	odoSvc := clinicalapp.NewOdontogramService(odoRepo, auditSvc, nil)
	perioSvc := clinicalapp.NewPeriodontogramService(perioRepo, auditSvc, nil)
	odoHandler := clinicalhttp.NewOdontogramHandler(odoSvc, perioSvc)
	odoHandler.RegisterRoutes(mux)

	soepRepo := clinicalpg.NewSOEPRepository()
	consentRepo := clinicalpg.NewConsentRepository()
	blobStore := clinicals3.NewConsentBlobStore()
	soepSvc := clinicalapp.NewSOEPService(soepRepo, auditSvc, nil, 24*time.Hour)
	consentSvc := clinicalapp.NewConsentService(consentRepo, blobStore, auditSvc, nil)
	soepConsentHandler := clinicalhttp.NewSOEPConsentHandler(soepSvc, consentSvc)
	soepConsentHandler.RegisterRoutes(mux)

	// 4. Treatments & Budgets
	treatmentRepo := treatmentpg.NewTreatmentRepository()
	treatmentSvc := treatmentapp.NewTreatmentService(treatmentRepo, auditSvc, nil)
	treatmentHandler := treatmenthttp.NewTreatmentHandler(treatmentSvc)
	treatmentHandler.RegisterRoutes(mux)

	// 5. Multi-chair Scheduling & Notifications
	schedulingRepo := schedulingpg.NewSchedulingRepository()
	dispatcher := schedulingmsg.NewNotificationDispatcher()
	schedulingSvc := schedulingapp.NewSchedulingService(schedulingRepo, dispatcher, auditSvc, nil)
	schedulingHandler := schedulinghttp.NewSchedulingHandler(schedulingSvc)
	schedulingHandler.RegisterRoutes(mux)

	// Health & Readiness endpoints
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "clinical-agenda"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready", "service": "clinical-agenda"})
	})

	return mux
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           buildMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("clinical-agenda listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
