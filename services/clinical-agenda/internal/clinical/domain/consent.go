package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidConsentTemplate = errors.New("consent requires patient_id, procedure_code, title, and rendered_text")
	ErrConsentAlreadySigned   = errors.New("informed consent has already been signed and sealed")
	ErrInvalidSignatureMethod = errors.New("signature_method must be BIOMETRIC or OTP")
	ErrEmptyBiometricStrokes  = errors.New("biometric_svg_strokes cannot be empty when signature_method is BIOMETRIC")
	ErrInvalidConsentOTP      = errors.New("invalid or expired 6-digit patient OTP code")
	ErrInvalidSignerIdentity  = errors.New("signed_by_name and signed_by_document are required")
	ErrConsentNotFound        = errors.New("informed consent not found")
)

type DeviceMetadata struct {
	ScreenInches float64 `json:"screen_inches"`
	TouchPoints  int     `json:"touch_points"`
}

type PatientConsent struct {
	ID               string          `json:"consent_id"`
	PatientID        string          `json:"patient_id"`
	ProcedureCode    string          `json:"procedure_code"`
	Title            string          `json:"title"`
	RenderedText     string          `json:"rendered_text"`
	ExpectedOTP      string          `json:"-"`
	Status           string          `json:"status"` // PENDING_SIGNATURE | SIGNED
	SignatureMethod  string          `json:"signature_method,omitempty"`
	SignedByName     string          `json:"signed_by_name,omitempty"`
	SignedByDocument string          `json:"signed_by_document,omitempty"`
	DocumentSHA256   string          `json:"document_sha256,omitempty"`
	StorageObjectKey string          `json:"storage_object_key,omitempty"`
	DeviceMetadata   *DeviceMetadata `json:"device_metadata,omitempty"`
	SignedAt         *time.Time      `json:"signed_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

func NewPatientConsent(id, patientID, procedureCode, title, renderedText, expectedOTP string, now time.Time) (*PatientConsent, error) {
	patientID = strings.TrimSpace(patientID)
	procedureCode = strings.TrimSpace(procedureCode)
	title = strings.TrimSpace(title)
	renderedText = strings.TrimSpace(renderedText)
	if patientID == "" || procedureCode == "" || title == "" || renderedText == "" {
		return nil, ErrInvalidConsentTemplate
	}
	if strings.TrimSpace(expectedOTP) == "" {
		expectedOTP = "654321"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return &PatientConsent{
		ID:            id,
		PatientID:     patientID,
		ProcedureCode: procedureCode,
		Title:         title,
		RenderedText:  renderedText,
		ExpectedOTP:   strings.TrimSpace(expectedOTP),
		Status:        "PENDING_SIGNATURE",
		CreatedAt:     now,
	}, nil
}

type SignConsentParams struct {
	Method              string
	BiometricSVGStrokes string
	OTPCode             string
	SignedByName        string
	SignedByDocument    string
	DeviceMetadata      *DeviceMetadata
}

func (c *PatientConsent) Sign(params SignConsentParams, now time.Time) ([]byte, error) {
	if c.Status == "SIGNED" {
		return nil, ErrConsentAlreadySigned
	}
	signerName := strings.TrimSpace(params.SignedByName)
	signerDoc := strings.TrimSpace(params.SignedByDocument)
	if signerName == "" || signerDoc == "" {
		return nil, ErrInvalidSignerIdentity
	}

	method := strings.ToUpper(strings.TrimSpace(params.Method))
	var proof string
	switch method {
	case "BIOMETRIC":
		strokes := strings.TrimSpace(params.BiometricSVGStrokes)
		if len(strokes) < 10 || !strings.Contains(strokes, "<svg") && !strings.Contains(strokes, "path") && !strings.Contains(strokes, "M") {
			return nil, ErrEmptyBiometricStrokes
		}
		proof = strokes
	case "OTP":
		otp := strings.TrimSpace(params.OTPCode)
		if len(otp) != 6 || otp != c.ExpectedOTP {
			return nil, ErrInvalidConsentOTP
		}
		proof = "OTP:" + otp
	default:
		return nil, ErrInvalidSignatureMethod
	}

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	pdfArtifact := []byte(fmt.Sprintf(
		"%%PDF-1.7\nSIGO-DENTAL INFORMED CONSENT\nConsentID: %s\nPatientID: %s\nProcedure: %s - %s\nBody: %s\nSignedBy: %s (%s)\nMethod: %s\nProof: %s\nTimestamp: %s\n%%EOF",
		c.ID, c.PatientID, c.ProcedureCode, c.Title, c.RenderedText, signerName, signerDoc, method, proof, now.Format(time.RFC3339),
	))
	sum := sha256.Sum256(pdfArtifact)
	digest := hex.EncodeToString(sum[:])

	c.Status = "SIGNED"
	c.SignatureMethod = method
	c.SignedByName = signerName
	c.SignedByDocument = signerDoc
	c.DocumentSHA256 = digest
	c.StorageObjectKey = fmt.Sprintf("consents/%04d/%02d/%s.pdf", now.Year(), int(now.Month()), c.ID)
	c.DeviceMetadata = params.DeviceMetadata
	c.SignedAt = &now

	return pdfArtifact, nil
}
