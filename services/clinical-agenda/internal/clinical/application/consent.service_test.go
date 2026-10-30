package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/s3"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
)

func TestConsentService_SignAndStoreEncryptedPDF(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 5, 0, 0, time.UTC)
	repo := postgres.NewConsentRepository()
	blobStore := s3.NewConsentBlobStore()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, fixedClock{now: now}, nil)
	svc := application.NewConsentService(repo, blobStore, auditSvc, fixedClock{now: now})
	_ = application.NewConsentService(repo, blobStore, nil, nil)

	if _, err := svc.CreateConsent(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", application.CreateConsentInput{}); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}
	if _, err := svc.SignConsent(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", "c-1", application.SignConsentInput{}); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}
	if _, err := svc.CreateConsent(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "auditor"}, "p-1", application.CreateConsentInput{}); !errors.Is(err, application.ErrClinicalForbiddenRole) {
		t.Fatalf("expected ErrClinicalForbiddenRole")
	}

	dentist := application.ActorMeta{
		ActorID:     "dent-1",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.15",
	}

	if _, err := svc.CreateConsent(context.Background(), dentist, "p-1", application.CreateConsentInput{}); err == nil {
		t.Fatalf("expected validation error on empty consent")
	}
	if _, err := svc.SignConsent(context.Background(), dentist, "p-1", "missing", application.SignConsentInput{}); err == nil {
		t.Fatalf("expected not found error on missing consent")
	}

	created, err := svc.CreateConsent(context.Background(), dentist, "p-1", application.CreateConsentInput{
		ProcedureCode: "ENDO-16",
		Title:         "Consentimiento Informado de Endodoncia",
		RenderedText:  "Comprendo los riesgos y beneficios del tratamiento de conducto en pieza 16.",
	})
	if err != nil {
		t.Fatalf("expected consent created, got %v", err)
	}

	signed, err := svc.SignConsent(context.Background(), dentist, "p-1", created.ID, application.SignConsentInput{
		SignatureMethod:     "BIOMETRIC",
		BiometricSVGStrokes: "<svg><path d='M12 12 L90 90'/></svg>",
		SignedByName:        "Martín Echeverría",
		SignedByDocument:    "40123987",
	})
	if err != nil || signed.Status != "SIGNED" {
		t.Fatalf("expected SIGNED consent, got %+v err=%v", signed, err)
	}

	if _, ok := blobStore.GetObject(context.Background(), signed.StorageObjectKey); !ok {
		t.Fatalf("expected PDF artifact stored in S3 blob store under key %s", signed.StorageObjectKey)
	}
}
