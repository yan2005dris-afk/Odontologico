package application

import (
	"context"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type ConsentRepository interface {
	SaveConsent(ctx context.Context, consent *domain.PatientConsent) error
	GetConsentByID(ctx context.Context, patientID, consentID string) (*domain.PatientConsent, error)
	ListConsents(ctx context.Context, filter ConsentFilter) ([]*domain.PatientConsent, int, error)
}

// ConsentFilter narrows an informed consent listing. Status and
// SignatureMethod hold domain values (PENDING_SIGNATURE | SIGNED and
// BIOMETRIC | OTP); the service maps the public vocabulary of the API onto
// them. Limit and Offset are normalized by the service: limit defaults to 50
// with a hard ceiling of 200.
type ConsentFilter struct {
	PatientID       string
	Status          string
	SignatureMethod string
	ProcedureCode   string
	Limit           int
	Offset          int
}

type BlobStoragePort interface {
	PutEncryptedPDF(ctx context.Context, objectKey string, content []byte, sha256Hex string) error
	GetObject(ctx context.Context, objectKey string) ([]byte, bool)
}

type CreateConsentInput struct {
	ProcedureCode string `json:"procedure_code"`
	Title         string `json:"title"`
	RenderedText  string `json:"rendered_text"`
	ExpectedOTP   string `json:"expected_otp,omitempty"`
}

type SignConsentInput struct {
	SignatureMethod     string                 `json:"signature_method"`
	BiometricSVGStrokes string                 `json:"biometric_svg_strokes"`
	DeviceMetadata      *domain.DeviceMetadata `json:"device_metadata"`
	OTPCode             string                 `json:"otp_code"`
	SignedByName        string                 `json:"signed_by_name"`
	SignedByDocument    string                 `json:"signed_by_document"`
}
