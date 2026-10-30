package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

func TestPatientConsent_BiometricAndOTPSigning(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 5, 0, 0, time.UTC)

	// 1. Biometric touch signature (S4)
	cBio, err := domain.NewPatientConsent("c-1", "p-1", "EXOD-18", "Consentimiento de Exodoncia", "Autorizo exodoncia de tercer molar 18.", "123456", now)
	if err != nil {
		t.Fatalf("expected valid consent, got %v", err)
	}
	// Empty strokes rejected
	if _, err := cBio.Sign(domain.SignConsentParams{
		Method:           "BIOMETRIC",
		SignedByName:     "Martín Echeverría",
		SignedByDocument: "40123987",
	}, now); !errors.Is(err, domain.ErrEmptyBiometricStrokes) {
		t.Fatalf("expected ErrEmptyBiometricStrokes, got %v", err)
	}

	pdfBytes, err := cBio.Sign(domain.SignConsentParams{
		Method:              "BIOMETRIC",
		BiometricSVGStrokes: "<svg><path d='M10 10 L80 80'/></svg>",
		SignedByName:        "Martín Echeverría",
		SignedByDocument:    "40123987",
	}, now)
	if err != nil || len(pdfBytes) == 0 || cBio.Status != "SIGNED" || len(cBio.DocumentSHA256) != 64 {
		t.Fatalf("expected signed biometric consent with SHA-256, got %+v err=%v", cBio, err)
	}

	// Re-signing already signed consent fails with ErrConsentAlreadySigned
	if _, err := cBio.Sign(domain.SignConsentParams{
		Method:              "BIOMETRIC",
		BiometricSVGStrokes: "<svg><path d='M10 10 L80 80'/></svg>",
		SignedByName:        "Martín Echeverría",
		SignedByDocument:    "40123987",
	}, now); !errors.Is(err, domain.ErrConsentAlreadySigned) {
		t.Fatalf("expected ErrConsentAlreadySigned, got %v", err)
	}

	// 2. OTP signature (S4)
	cOTP, _ := domain.NewPatientConsent("c-2", "p-1", "IMPL-36", "Consentimiento de Implante", "Autorizo colocación de implante en pieza 36.", "654321", now)
	if _, err := cOTP.Sign(domain.SignConsentParams{
		Method:           "OTP",
		OTPCode:          "000000",
		SignedByName:     "Martín Echeverría",
		SignedByDocument: "40123987",
	}, now); !errors.Is(err, domain.ErrInvalidConsentOTP) {
		t.Fatalf("expected ErrInvalidConsentOTP, got %v", err)
	}
	if _, err := cOTP.Sign(domain.SignConsentParams{
		Method:           "OTP",
		OTPCode:          "654321",
		SignedByName:     "Martín Echeverría",
		SignedByDocument: "40123987",
	}, now); err != nil || cOTP.Status != "SIGNED" {
		t.Fatalf("expected OTP consent signed, got %+v err=%v", cOTP, err)
	}
}
