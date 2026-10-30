package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidAnamnesis = errors.New("invalid anamnesis payload")
)

type SystemicDisease struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Controlled bool   `json:"controlled"`
	Notes      string `json:"notes"`
}

type DrugAllergy struct {
	Substance string `json:"substance"`
	Reaction  string `json:"reaction"`
	Severity  string `json:"severity"` // MILD | MODERATE | CRITICAL
}

type SurgicalHistory struct {
	Procedure     string `json:"procedure"`
	Year          int    `json:"year"`
	Complications string `json:"complications"`
}

type Coagulopathy struct {
	Condition        string  `json:"condition"`
	Medication       string  `json:"medication"`
	INRLastValue     float64 `json:"inr_last_value"`
	IsAnticoagulated bool    `json:"is_anticoagulated"`
}

type Anamnesis struct {
	ID               string            `json:"id"`
	PatientID        string            `json:"patient_id"`
	SystemicDiseases []SystemicDisease `json:"systemic_diseases"`
	DrugAllergies    []DrugAllergy     `json:"drug_allergies"`
	SurgicalHistory  []SurgicalHistory `json:"surgical_history"`
	Coagulopathies   []Coagulopathy    `json:"coagulopathies"`
	IsPregnant       bool              `json:"is_pregnant"`
	Notes            string            `json:"notes"`
	RecordedBy       string            `json:"recorded_by"`
	RecordedAt       time.Time         `json:"recorded_at"`
}

func NewAnamnesis(
	id, patientID, recordedBy string,
	diseases []SystemicDisease,
	allergies []DrugAllergy,
	surgeries []SurgicalHistory,
	coagulopathies []Coagulopathy,
	isPregnant bool,
	notes string,
	now time.Time,
) (*Anamnesis, []CriticalAlert, error) {
	if strings.TrimSpace(patientID) == "" || strings.TrimSpace(recordedBy) == "" {
		return nil, nil, ErrInvalidAnamnesis
	}
	for _, a := range allergies {
		sev := strings.ToUpper(strings.TrimSpace(a.Severity))
		if strings.TrimSpace(a.Substance) == "" || (sev != "MILD" && sev != "MODERATE" && sev != "CRITICAL") {
			return nil, nil, ErrInvalidAnamnesis
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	anam := &Anamnesis{
		ID:               id,
		PatientID:        patientID,
		SystemicDiseases: diseases,
		DrugAllergies:    allergies,
		SurgicalHistory:  surgeries,
		Coagulopathies:   coagulopathies,
		IsPregnant:       isPregnant,
		Notes:            strings.TrimSpace(notes),
		RecordedBy:       recordedBy,
		RecordedAt:       now,
	}

	alerts := DeriveCriticalAlerts(anam)
	return anam, alerts, nil
}

func DeriveCriticalAlerts(a *Anamnesis) []CriticalAlert {
	if a == nil {
		return []CriticalAlert{}
	}
	alerts := make([]CriticalAlert, 0)

	for _, allergy := range a.DrugAllergies {
		sev := strings.ToUpper(strings.TrimSpace(allergy.Severity))
		subUpper := strings.ToUpper(strings.TrimSpace(allergy.Substance))
		if sev == "CRITICAL" || strings.Contains(subUpper, "PENICIL") || strings.Contains(subUpper, "LATEX") || strings.Contains(subUpper, "ANEST") {
			code := "ALLERGY_CRITICAL"
			if strings.Contains(subUpper, "PENICIL") {
				code = "ALLERGY_PENICILLIN"
			}
			badgeSev := "CRITICAL"
			if sev == "MODERATE" {
				badgeSev = "HIGH"
			}
			alerts = append(alerts, CriticalAlert{
				Code:     code,
				Label:    fmt.Sprintf("ALERGIA CRÍTICA: %s (%s)", strings.ToUpper(strings.TrimSpace(allergy.Substance)), strings.TrimSpace(allergy.Reaction)),
				Severity: badgeSev,
				Category: "DRUG_ALLERGY",
			})
		}
	}

	for _, coag := range a.Coagulopathies {
		if coag.IsAnticoagulated || strings.TrimSpace(coag.Medication) != "" {
			med := strings.TrimSpace(coag.Medication)
			if med == "" {
				med = strings.TrimSpace(coag.Condition)
			}
			alerts = append(alerts, CriticalAlert{
				Code:     "ANTICOAGULANT_ACTIVE",
				Label:    fmt.Sprintf("ANTICOAGULADO: %s (INR %.1f)", med, coag.INRLastValue),
				Severity: "CRITICAL",
				Category: "COAGULOPATHY",
			})
		} else if strings.TrimSpace(coag.Condition) != "" {
			alerts = append(alerts, CriticalAlert{
				Code:     "COAGULOPATHY_ALERT",
				Label:    fmt.Sprintf("COAGULOPATÍA: %s", strings.TrimSpace(coag.Condition)),
				Severity: "HIGH",
				Category: "COAGULOPATHY",
			})
		}
	}

	for _, dis := range a.SystemicDiseases {
		nameUpper := strings.ToUpper(strings.TrimSpace(dis.Name))
		if !dis.Controlled || strings.Contains(nameUpper, "ENDOCARDITIS") || strings.Contains(nameUpper, "BISFOSFONAT") || strings.Contains(nameUpper, "BIFOSFONAT") {
			sev := "HIGH"
			if strings.Contains(nameUpper, "ENDOCARDITIS") || strings.Contains(nameUpper, "BISFOSFONAT") || strings.Contains(nameUpper, "BIFOSFONAT") {
				sev = "CRITICAL"
			}
			alerts = append(alerts, CriticalAlert{
				Code:     "UNCONTROLLED_SYSTEMIC",
				Label:    fmt.Sprintf("SISTÉMICA NO CONTROLADA / RIESGO: %s", strings.TrimSpace(dis.Name)),
				Severity: sev,
				Category: "SYSTEMIC_DISEASE",
			})
		}
	}

	if a.IsPregnant {
		alerts = append(alerts, CriticalAlert{
			Code:     "PREGNANCY_ACTIVE",
			Label:    "GESTANTE: Precaución radiológica y farmacológica",
			Severity: "HIGH",
			Category: "PREGNANCY",
		})
	}

	return alerts
}
