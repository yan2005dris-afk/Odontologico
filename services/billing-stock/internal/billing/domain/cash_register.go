package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	CashRegisterStatusOpen   = "OPEN"
	CashRegisterStatusClosed = "CLOSED"

	MovementDirectionIn  = "IN"
	MovementDirectionOut = "OUT"
)

var (
	ErrCashRegisterNotFound      = errors.New("CASH_REGISTER_NOT_FOUND")
	ErrCashRegisterAlreadyClosed = errors.New("CASH_REGISTER_ALREADY_CLOSED")
	ErrBlindCashCloseEnforced    = errors.New("BLIND_CASH_CLOSE_ENFORCED")
	ErrNegativeDeclaredCash      = errors.New("NEGATIVE_DECLARED_CASH")
	ErrInvalidCashRegisterInput  = errors.New("INVALID_CASH_REGISTER_INPUT")
)

type CashMovement struct {
	ID             string    `json:"id"`
	CashRegisterID string    `json:"cash_register_id"`
	InvoiceID      string    `json:"invoice_id,omitempty"`
	Direction      string    `json:"direction"`
	AmountCents    int64     `json:"amount_cents"`
	Reason         string    `json:"reason"`
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// PublicCashRegisterView strictly enforces RN-02: it does NOT contain theoretical_cash_cents
// or discrepancy_cents, making accidental serialization leaks impossible.
type PublicCashRegisterView struct {
	ID                string    `json:"id"`
	ClinicID          string    `json:"clinic_id"`
	OpenedBy          string    `json:"opened_by"`
	OpenedAt          time.Time `json:"opened_at"`
	OpeningFloatCents int64     `json:"opening_float_cents"`
	Status            string    `json:"status"`
}

// BlindCloseReceiptView is returned to the receptionist upon submitting the blind cash count (RN-02).
type BlindCloseReceiptView struct {
	CashRegisterID    string    `json:"cash_register_id"`
	Status            string    `json:"status"`
	DeclaredCashCents int64     `json:"declared_cash_cents"`
	ClosedBy          string    `json:"closed_by"`
	ClosedAt          time.Time `json:"closed_at"`
	ReceiptHash       string    `json:"receipt_hash"`
}

// CashRegisterAuditReportView is accessible exclusively to auditor and admin after close (RN-02, S4).
type CashRegisterAuditReportView struct {
	ID                   string         `json:"id"`
	ClinicID             string         `json:"clinic_id"`
	OpenedBy             string         `json:"opened_by"`
	OpenedAt             time.Time      `json:"opened_at"`
	OpeningFloatCents    int64          `json:"opening_float_cents"`
	Status               string         `json:"status"`
	TheoreticalCashCents int64          `json:"theoretical_cash_cents"`
	DeclaredCashCents    int64          `json:"declared_cash_cents"`
	DiscrepancyCents     int64          `json:"discrepancy_cents"`
	Denominations        map[string]int `json:"denominations,omitempty"`
	CashierNotes         string         `json:"cashier_notes,omitempty"`
	ClosedBy             string         `json:"closed_by"`
	ClosedAt             time.Time      `json:"closed_at"`
	ReceiptHash          string         `json:"receipt_hash"`
}

type CashRegister struct {
	ID                   string
	ClinicID             string
	OpenedBy             string
	OpenedAt             time.Time
	OpeningFloatCents    int64
	Status               string
	Movements            []CashMovement
	DeclaredCashCents    *int64
	TheoreticalCashCents *int64
	DiscrepancyCents     *int64
	Denominations        map[string]int
	CashierNotes         string
	ClosedBy             string
	ClosedAt             *time.Time
	ReceiptHash          string
}

func OpenCashRegister(id, clinicID, openedBy string, openingFloatCents int64, openedAt time.Time) (*CashRegister, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(clinicID) == "" || strings.TrimSpace(openedBy) == "" {
		return nil, ErrInvalidCashRegisterInput
	}
	if openingFloatCents < 0 {
		return nil, ErrInvalidCashRegisterInput
	}
	if openedAt.IsZero() {
		openedAt = time.Now().UTC()
	}
	return &CashRegister{
		ID:                id,
		ClinicID:          clinicID,
		OpenedBy:          openedBy,
		OpenedAt:          openedAt.UTC(),
		OpeningFloatCents: openingFloatCents,
		Status:            CashRegisterStatusOpen,
		Movements:         make([]CashMovement, 0),
	}, nil
}

func (cr *CashRegister) RecordCashMovement(id, invoiceID, direction string, amountCents int64, reason, actorID string, createdAt time.Time) error {
	if cr.Status != CashRegisterStatusOpen {
		return ErrCashRegisterAlreadyClosed
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(reason) == "" || strings.TrimSpace(actorID) == "" {
		return ErrInvalidCashRegisterInput
	}
	if direction != MovementDirectionIn && direction != MovementDirectionOut {
		return ErrInvalidCashRegisterInput
	}
	if amountCents <= 0 {
		return ErrInvalidCashRegisterInput
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	cr.Movements = append(cr.Movements, CashMovement{
		ID:             id,
		CashRegisterID: cr.ID,
		InvoiceID:      invoiceID,
		Direction:      direction,
		AmountCents:    amountCents,
		Reason:         reason,
		ActorID:        actorID,
		CreatedAt:      createdAt.UTC(),
	})
	return nil
}

func (cr *CashRegister) calculateTheoreticalCashCents() int64 {
	total := cr.OpeningFloatCents
	for _, m := range cr.Movements {
		if m.Direction == MovementDirectionIn {
			total += m.AmountCents
		} else if m.Direction == MovementDirectionOut {
			total -= m.AmountCents
		}
	}
	return total
}

func (cr *CashRegister) ToReceptionistStatusView() PublicCashRegisterView {
	return PublicCashRegisterView{
		ID:                cr.ID,
		ClinicID:          cr.ClinicID,
		OpenedBy:          cr.OpenedBy,
		OpenedAt:          cr.OpenedAt,
		OpeningFloatCents: cr.OpeningFloatCents,
		Status:            cr.Status,
	}
}

func (cr *CashRegister) ExecuteBlindClose(declaredCashCents int64, denominations map[string]int, notes, closedBy string, closedAt time.Time) (*BlindCloseReceiptView, error) {
	if cr.Status == CashRegisterStatusClosed {
		return nil, ErrCashRegisterAlreadyClosed
	}
	if declaredCashCents < 0 {
		return nil, ErrNegativeDeclaredCash
	}
	if strings.TrimSpace(closedBy) == "" {
		return nil, ErrInvalidCashRegisterInput
	}
	if closedAt.IsZero() {
		closedAt = time.Now().UTC()
	} else {
		closedAt = closedAt.UTC()
	}

	theoretical := cr.calculateTheoreticalCashCents()
	discrepancy := declaredCashCents - theoretical

	declaredCopy := declaredCashCents
	cr.DeclaredCashCents = &declaredCopy
	cr.TheoreticalCashCents = &theoretical
	cr.DiscrepancyCents = &discrepancy

	if len(denominations) > 0 {
		denomCopy := make(map[string]int, len(denominations))
		for k, v := range denominations {
			denomCopy[k] = v
		}
		cr.Denominations = denomCopy
	}
	cr.CashierNotes = notes
	cr.ClosedBy = closedBy
	cr.ClosedAt = &closedAt
	cr.Status = CashRegisterStatusClosed

	payload := fmt.Sprintf("%s|%s|%d|%d|%d|%s", cr.ID, closedBy, declaredCashCents, theoretical, discrepancy, closedAt.Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(payload))
	cr.ReceiptHash = "sha256:" + hex.EncodeToString(sum[:])

	return &BlindCloseReceiptView{
		CashRegisterID:    cr.ID,
		Status:            cr.Status,
		DeclaredCashCents: declaredCashCents,
		ClosedBy:          closedBy,
		ClosedAt:          closedAt,
		ReceiptHash:       cr.ReceiptHash,
	}, nil
}

// CashRegisterListItemView is the DTO of the cash register listing.
//
// RN-02: the theoretical cash total only exists once the register is CLOSED. While the
// register is OPEN the field is absent from the JSON payload (not null, not zero), so
// a receptionist can never read the theoretical total before submitting the count.
type CashRegisterListItemView struct {
	ID               string     `json:"id"`
	ClinicID         string     `json:"clinic_id"`
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	Status           string     `json:"status"`
	DeclaredCents    *int64     `json:"declared_cents,omitempty"`
	TheoreticalCents *int64     `json:"theoretical_cents,omitempty"`
	DiscrepancyCents *int64     `json:"discrepancy_cents,omitempty"`
}

func (cr *CashRegister) ToListItemView() CashRegisterListItemView {
	view := CashRegisterListItemView{
		ID:       cr.ID,
		ClinicID: cr.ClinicID,
		OpenedAt: cr.OpenedAt,
		Status:   cr.Status,
	}
	if cr.ClosedAt != nil {
		closedAt := *cr.ClosedAt
		view.ClosedAt = &closedAt
	}
	if cr.DeclaredCashCents != nil {
		declared := *cr.DeclaredCashCents
		view.DeclaredCents = &declared
	}
	if cr.DiscrepancyCents != nil {
		discrepancy := *cr.DiscrepancyCents
		view.DiscrepancyCents = &discrepancy
	}
	// RN-02: the theoretical total is only revealed for a CLOSED register.
	if cr.Status == CashRegisterStatusClosed && cr.TheoreticalCashCents != nil {
		theoretical := *cr.TheoreticalCashCents
		view.TheoreticalCents = &theoretical
	}
	return view
}

func (cr *CashRegister) ToAuditReportView() (*CashRegisterAuditReportView, error) {
	if cr.Status != CashRegisterStatusClosed || cr.TheoreticalCashCents == nil || cr.DeclaredCashCents == nil || cr.DiscrepancyCents == nil || cr.ClosedAt == nil {
		return nil, ErrBlindCashCloseEnforced
	}
	return &CashRegisterAuditReportView{
		ID:                   cr.ID,
		ClinicID:             cr.ClinicID,
		OpenedBy:             cr.OpenedBy,
		OpenedAt:             cr.OpenedAt,
		OpeningFloatCents:    cr.OpeningFloatCents,
		Status:               cr.Status,
		TheoreticalCashCents: *cr.TheoreticalCashCents,
		DeclaredCashCents:    *cr.DeclaredCashCents,
		DiscrepancyCents:     *cr.DiscrepancyCents,
		Denominations:        cr.Denominations,
		CashierNotes:         cr.CashierNotes,
		ClosedBy:             cr.ClosedBy,
		ClosedAt:             *cr.ClosedAt,
		ReceiptHash:          cr.ReceiptHash,
	}, nil
}
