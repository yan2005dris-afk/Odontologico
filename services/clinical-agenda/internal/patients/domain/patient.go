package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidPatientDemographics  = errors.New("missing or invalid required patient demographic fields")
	ErrInvalidBirthDate            = errors.New("birth_date must be a valid YYYY-MM-DD not in the future")
	ErrEmergencyContactRequired    = errors.New("emergency_contact (full_name, phone, relationship) is mandatory")
	ErrLegalGuardianRequiredMinor  = errors.New("legal_guardian (full_name, document_number, phone, relationship) is mandatory for minors under 18")
	ErrDuplicatePatientDocument    = errors.New("a patient with the same document_number already exists")
	ErrPatientNotFound             = errors.New("patient not found")
)

type EmergencyContact struct {
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	Relationship string `json:"relationship"`
}

type LegalGuardian struct {
	FullName       string `json:"full_name"`
	DocumentNumber string `json:"document_number"`
	Phone          string `json:"phone"`
	Relationship   string `json:"relationship"`
}

type InsurancePlan struct {
	ProviderName string `json:"provider_name"`
	PlanName     string `json:"plan_name"`
	MemberNumber string `json:"member_number"`
}

type CriticalAlert struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Severity string `json:"severity"`
	Category string `json:"category"`
}

type Patient struct {
	ID                string           `json:"id"`
	HCNumber          string           `json:"hc_number"`
	DocumentType      string           `json:"document_type"`
	DocumentNumber    string           `json:"document_number"`
	FirstName         string           `json:"first_name"`
	LastName          string           `json:"last_name"`
	BirthDate         string           `json:"birth_date"`
	Age               int              `json:"age"`
	IsMinor           bool             `json:"is_minor"`
	Sex               string           `json:"sex"`
	Phone             string           `json:"phone"`
	Email             string           `json:"email,omitempty"`
	EmergencyContact  EmergencyContact `json:"emergency_contact"`
	LegalGuardian     *LegalGuardian   `json:"legal_guardian,omitempty"`
	InsurancePlan     *InsurancePlan   `json:"insurance_plan,omitempty"`
	AnamnesisRecorded bool             `json:"anamnesis_recorded"`
	CriticalAlerts    []CriticalAlert  `json:"critical_alerts"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type CreatePatientParams struct {
	ID               string
	HCNumber         string
	DocumentType     string
	DocumentNumber   string
	FirstName        string
	LastName         string
	BirthDate        string
	Sex              string
	Phone            string
	Email            string
	EmergencyContact EmergencyContact
	LegalGuardian    *LegalGuardian
	InsurancePlan    *InsurancePlan
}

func NewPatient(params CreatePatientParams, now time.Time) (*Patient, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	docType := strings.ToUpper(strings.TrimSpace(params.DocumentType))
	switch docType {
	case "DNI", "PASSPORT", "FOREIGN_ID":
	default:
		return nil, ErrInvalidPatientDemographics
	}

	docNum := strings.TrimSpace(params.DocumentNumber)
	firstName := strings.TrimSpace(params.FirstName)
	lastName := strings.TrimSpace(params.LastName)
	phone := strings.TrimSpace(params.Phone)
	sex := strings.ToUpper(strings.TrimSpace(params.Sex))

	if len(docNum) < 5 || len(docNum) > 30 || firstName == "" || lastName == "" || phone == "" {
		return nil, ErrInvalidPatientDemographics
	}
	if sex != "F" && sex != "M" && sex != "X" {
		return nil, ErrInvalidPatientDemographics
	}

	dob, err := time.Parse("2006-01-02", strings.TrimSpace(params.BirthDate))
	if err != nil || dob.After(now) {
		return nil, ErrInvalidBirthDate
	}

	if strings.TrimSpace(params.EmergencyContact.FullName) == "" ||
		strings.TrimSpace(params.EmergencyContact.Phone) == "" ||
		strings.TrimSpace(params.EmergencyContact.Relationship) == "" {
		return nil, ErrEmergencyContactRequired
	}

	age := CalculateExactAge(dob, now)
	isMinor := age < 18
	if isMinor {
		if params.LegalGuardian == nil ||
			strings.TrimSpace(params.LegalGuardian.FullName) == "" ||
			strings.TrimSpace(params.LegalGuardian.DocumentNumber) == "" ||
			strings.TrimSpace(params.LegalGuardian.Phone) == "" ||
			strings.TrimSpace(params.LegalGuardian.Relationship) == "" {
			return nil, ErrLegalGuardianRequiredMinor
		}
	}

	return &Patient{
		ID:                params.ID,
		HCNumber:          params.HCNumber,
		DocumentType:      docType,
		DocumentNumber:    docNum,
		FirstName:         firstName,
		LastName:          lastName,
		BirthDate:         dob.Format("2006-01-02"),
		Age:               age,
		IsMinor:           isMinor,
		Sex:               sex,
		Phone:             phone,
		Email:             strings.TrimSpace(params.Email),
		EmergencyContact:  params.EmergencyContact,
		LegalGuardian:     params.LegalGuardian,
		InsurancePlan:     params.InsurancePlan,
		AnamnesisRecorded: false,
		CriticalAlerts:    []CriticalAlert{},
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func CalculateExactAge(birthDate, now time.Time) int {
	years := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}
