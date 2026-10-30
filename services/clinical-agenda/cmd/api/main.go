package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
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

// mfaRequired reports whether the deployment demands the RNF-05 second factor.
//
// The default is TRUE: the requirement can only be turned off explicitly, with
// MFA_REQUIRED=false, and the development stack in deploy/compose.yaml is the
// only place that does it.
func mfaRequired() bool {
	return envBool("MFA_REQUIRED", true)
}

// envBool reads a boolean environment variable. An unset or unparseable value
// falls back to the given default, so the secure default can never be lost to
// a typo in the deployment (wi-18 S0).
func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("clinical-agenda: invalid boolean for %s=%q, using %v", name, raw, fallback)
		return fallback
	}
	return parsed
}

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

// seedAccount describes one canonical RBAC account created on service boot.
type seedAccount struct {
	id          string
	email       string
	fullName    string
	role        identitydom.Role
	password    string
	totpEnv     string
	totpDefault string
}

// canonicalSeedAccounts holds the 5 canonical RBAC roles of SRS §3 so that the
// administrative listings and the role gated screens can be exercised without
// an out-of-band provisioning channel.
//
// The `dentist` account keeps the credentials the portal and deploy/README.md
// document: the default TOTP secret is the one already in use.
var canonicalSeedAccounts = []seedAccount{
	{
		id:          "b1c2d3e4-1111-4a5b-8c9d-0e1f2a3b4c5e",
		email:       "admin@sigo-dental.com",
		fullName:    "Andrés Vega Salinas",
		role:        identitydom.RoleAdmin,
		password:    "AdminPass#2026",
		totpEnv:     "SIGO_SEED_TOTP_ADMIN",
		totpDefault: "GEZDGNBVGY3TQOJQ",
	},
	{
		id:          "b1c2d3e4-2222-4a5b-8c9d-0e1f2a3b4c5e",
		email:       "recepcion@sigo-dental.com",
		fullName:    "Lucía Fernández Ruiz",
		role:        identitydom.RoleReceptionist,
		password:    "ReceptionPass#2026",
		totpEnv:     "SIGO_SEED_TOTP_RECEPTIONIST",
		totpDefault: "MFRGGZDFMZTWQ2LK",
	},
	{
		id:          "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		email:       "dra.gomez@sigo-dental.com",
		fullName:    "Dra. Valeria Gómez",
		role:        identitydom.RoleDentist,
		password:    "ClinicalPass#2026",
		totpEnv:     "SIGO_SEED_TOTP_DENTIST",
		totpDefault: "JBSWY3DPEHPK3PXP",
	},
	{
		id:          "b1c2d3e4-3333-4a5b-8c9d-0e1f2a3b4c5e",
		email:       "asistente@sigo-dental.com",
		fullName:    "Marcelo Ríos Cabrera",
		role:        identitydom.RoleAssistant,
		password:    "AssistantPass#2026",
		totpEnv:     "SIGO_SEED_TOTP_ASSISTANT",
		totpDefault: "ONSWG4TFOQQGSZLD",
	},
	{
		id:          "b1c2d3e4-4444-4a5b-8c9d-0e1f2a3b4c5e",
		email:       "auditor@sigo-dental.com",
		fullName:    "Paula Herrera Ortiz",
		role:        identitydom.RoleAuditor,
		password:    "AuditorPass#2026",
		totpEnv:     "SIGO_SEED_TOTP_AUDITOR",
		totpDefault: "P3K6H5RTX2W7YZAB",
	},
}

// seedCanonicalUsers creates the 5 canonical RBAC accounts (SRS §3) on every
// boot. The seed is idempotent: an account that already exists is left
// untouched, so replaying it never fails nor duplicates accounts.
func seedCanonicalUsers(ctx context.Context, repo identityapp.UserRepository, now time.Time) error {
	for _, account := range canonicalSeedAccounts {
		if _, err := repo.GetByEmail(ctx, account.email); err == nil {
			continue
		}
		user, err := identitydom.NewUser(
			account.id,
			account.email,
			account.fullName,
			account.role,
			account.password,
			seedTOTPSecret(account.totpEnv, account.totpDefault),
			now,
		)
		if err != nil {
			return err
		}
		if err := repo.CreateUser(ctx, user); err != nil {
			return err
		}
	}
	return nil
}

// seedTOTPSecret resolves the development TOTP secret of a seeded account.
// The secrets are read from the environment with deterministic defaults so the
// team can authenticate without an out-of-band channel.
func seedTOTPSecret(envName, fallback string) string {
	if secret := strings.TrimSpace(os.Getenv(envName)); secret != "" {
		return strings.ToUpper(secret)
	}
	return fallback
}

func buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	// 1. Audit & Identity
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)
	auditHandler := audithttp.NewAuditHandler(auditSvc)
	auditHandler.RegisterRoutes(mux)

	userRepo := identitypg.NewUserRepository()
	if err := seedCanonicalUsers(context.Background(), userRepo, time.Now().UTC()); err != nil {
		log.Printf("could not seed the canonical RBAC accounts: %v", err)
	}
	authSvc := identityapp.NewAuthService(userRepo, auditSvc, nil, os.Getenv("JWT_SECRET"), idleSessionRegistrar(), mfaRequired())
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
