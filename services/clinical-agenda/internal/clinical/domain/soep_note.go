package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultSOEPLockWindow = 24 * time.Hour

var (
	ErrInvalidSOEPFields  = errors.New("SOEP note requires subjective, objective, assessment, and plan (min 5 chars each)")
	ErrSOEPNoteLocked     = errors.New("RN-01: SOEP note is locked past its 24h edit window; submit a signed addendum")
	ErrSOEPDifferentAuthor = errors.New("only the original author dentist may edit an unlocked SOEP note")
	ErrSOEPNotFound       = errors.New("SOEP note not found")
	ErrInvalidSOEPAddendum = errors.New("SOEP addendum requires reason (>= 5 chars) and correction_text (>= 10 chars)")
)

type SOEPAddendum struct {
	ID              string    `json:"id"`
	ParentNoteID    string    `json:"parent_note_id"`
	AuthorDentistID string    `json:"author_dentist_id"`
	Reason          string    `json:"reason"`
	CorrectionText  string    `json:"correction_text"`
	SignatureHash   string    `json:"signature_hash"`
	CreatedAt       time.Time `json:"created_at"`
}

type SOEPNote struct {
	ID              string         `json:"id"`
	PatientID       string         `json:"patient_id"`
	AppointmentID   string         `json:"appointment_id,omitempty"`
	ProcedureCode   string         `json:"procedure_code,omitempty"`
	AuthorDentistID string         `json:"author_dentist_id"`
	Subjective      string         `json:"subjective"`
	Objective       string         `json:"objective"`
	Assessment      string         `json:"assessment"`
	Plan            string         `json:"plan"`
	SignatureHash   string         `json:"signature_hash"`
	CreatedAt       time.Time      `json:"created_at"`
	LocksAt         time.Time      `json:"locks_at"`
	IsLocked        bool           `json:"is_locked"`
	Addenda         []SOEPAddendum `json:"addenda"`
}

func NewSOEPNote(id, patientID, appointmentID, procedureCode, authorDentistID, subjective, objective, assessment, plan string, lockWindow time.Duration, now time.Time) (*SOEPNote, error) {
	subjective = strings.TrimSpace(subjective)
	objective = strings.TrimSpace(objective)
	assessment = strings.TrimSpace(assessment)
	plan = strings.TrimSpace(plan)
	authorDentistID = strings.TrimSpace(authorDentistID)
	patientID = strings.TrimSpace(patientID)

	if patientID == "" || authorDentistID == "" ||
		len(subjective) < 5 || len(objective) < 5 || len(assessment) < 5 || len(plan) < 5 {
		return nil, ErrInvalidSOEPFields
	}
	if lockWindow <= 0 {
		lockWindow = DefaultSOEPLockWindow
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	locksAt := now.Add(lockWindow)
	sig := computeSOEPSignature(authorDentistID, subjective, objective, assessment, plan, now)

	return &SOEPNote{
		ID:              id,
		PatientID:       patientID,
		AppointmentID:   strings.TrimSpace(appointmentID),
		ProcedureCode:   strings.TrimSpace(procedureCode),
		AuthorDentistID: authorDentistID,
		Subjective:      subjective,
		Objective:       objective,
		Assessment:      assessment,
		Plan:            plan,
		SignatureHash:   sig,
		CreatedAt:       now,
		LocksAt:         locksAt,
		IsLocked:        false,
		Addenda:         []SOEPAddendum{},
	}, nil
}

func (n *SOEPNote) RefreshLockState(now time.Time) {
	if !now.UTC().Before(n.LocksAt) {
		n.IsLocked = true
	}
}

func (n *SOEPNote) UpdateWithinWindow(editorDentistID, subjective, objective, assessment, plan string, now time.Time) error {
	now = now.UTC()
	n.RefreshLockState(now)
	if n.IsLocked {
		return ErrSOEPNoteLocked
	}
	if strings.TrimSpace(editorDentistID) != n.AuthorDentistID {
		return ErrSOEPDifferentAuthor
	}
	subjective = strings.TrimSpace(subjective)
	objective = strings.TrimSpace(objective)
	assessment = strings.TrimSpace(assessment)
	plan = strings.TrimSpace(plan)
	if len(subjective) < 5 || len(objective) < 5 || len(assessment) < 5 || len(plan) < 5 {
		return ErrInvalidSOEPFields
	}
	n.Subjective = subjective
	n.Objective = objective
	n.Assessment = assessment
	n.Plan = plan
	n.SignatureHash = computeSOEPSignature(n.AuthorDentistID, subjective, objective, assessment, plan, now)
	return nil
}

func (n *SOEPNote) Delete() error {
	return ErrClinicalRecordImmutable
}

func (n *SOEPNote) AppendAddendum(id, authorDentistID, reason, correctionText string, now time.Time) (*SOEPAddendum, error) {
	reason = strings.TrimSpace(reason)
	correctionText = strings.TrimSpace(correctionText)
	authorDentistID = strings.TrimSpace(authorDentistID)
	if authorDentistID == "" || len(reason) < 5 || len(correctionText) < 10 {
		return nil, ErrInvalidSOEPAddendum
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s", n.ID, authorDentistID, reason, correctionText, now.Format(time.RFC3339))))
	add := SOEPAddendum{
		ID:              id,
		ParentNoteID:    n.ID,
		AuthorDentistID: authorDentistID,
		Reason:          reason,
		CorrectionText:  correctionText,
		SignatureHash:   "sha256:" + hex.EncodeToString(digest[:]),
		CreatedAt:       now,
	}
	n.Addenda = append(n.Addenda, add)
	return &add, nil
}

func computeSOEPSignature(dentistID, s, o, a, p string, ts time.Time) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s", dentistID, s, o, a, p, ts.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}
