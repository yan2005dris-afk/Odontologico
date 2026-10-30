package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

func TestCashRegister_BlindCashCloseInvariants_RN02(t *testing.T) {
	openedAt := time.Date(2026, 10, 30, 7, 0, 0, 0, time.UTC)
	reg, err := domain.OpenCashRegister("reg-001", "clinic-001", "recep-01", 50000, openedAt)
	if err != nil {
		t.Fatalf("unexpected error opening cash register: %v", err)
	}

	// Record 2 cash-in movements (+100,000 and +40,000) and 1 cash-out movement (-5,000)
	// Theoretical = 50,000 + 140,000 - 5,000 = 185,000
	if err := reg.RecordCashMovement("mov-1", "inv-1", domain.MovementDirectionIn, 100000, "Invoice cash payment", "recep-01", openedAt.Add(time.Hour)); err != nil {
		t.Fatalf("unexpected error recording movement: %v", err)
	}
	if err := reg.RecordCashMovement("mov-2", "inv-2", domain.MovementDirectionIn, 40000, "Invoice cash payment", "recep-01", openedAt.Add(2*time.Hour)); err != nil {
		t.Fatalf("unexpected error recording movement: %v", err)
	}
	if err := reg.RecordCashMovement("mov-3", "", domain.MovementDirectionOut, 5000, "Petty cash courier", "recep-01", openedAt.Add(3*time.Hour)); err != nil {
		t.Fatalf("unexpected error recording movement: %v", err)
	}

	// RN-02 Check 1: Public view for receptionist NEVER exposes theoretical_cash_cents
	pubView := reg.ToReceptionistStatusView()
	rawPub, err := json.Marshal(pubView)
	if err != nil {
		t.Fatal(err)
	}
	jsonStr := string(rawPub)
	if strings.Contains(jsonStr, "theoretical") || strings.Contains(jsonStr, "185000") || strings.Contains(jsonStr, "discrepancy") {
		t.Fatalf("RN-02 violation! PublicCashRegisterView leaked theoretical cash: %s", jsonStr)
	}

	// RN-02 Check 2: Blind close requires declared_cash_cents >= 0
	closedAt := openedAt.Add(14 * time.Hour)
	if _, err := reg.ExecuteBlindClose(-100, nil, "invalid", "recep-01", closedAt); err != domain.ErrNegativeDeclaredCash {
		t.Fatalf("expected ErrNegativeDeclaredCash, got %v", err)
	}

	// Execute blind close with 182,000 declared (discrepancy = -3,000)
	receipt, err := reg.ExecuteBlindClose(182000, map[string]int{"1000": 182}, "Counted at shift end", "recep-01", closedAt)
	if err != nil {
		t.Fatalf("unexpected error executing blind close: %v", err)
	}
	if receipt.Status != domain.CashRegisterStatusClosed {
		t.Fatalf("expected CLOSED status, got %s", receipt.Status)
	}
	if !strings.HasPrefix(receipt.ReceiptHash, "sha256:") {
		t.Fatalf("expected sha256 receipt hash, got %s", receipt.ReceiptHash)
	}

	// RN-02 Check 3: Receptionist blind-close receipt MUST NOT expose theoretical_cash_cents or discrepancy_cents
	rawReceipt, _ := json.Marshal(receipt)
	receiptJSON := string(rawReceipt)
	if strings.Contains(receiptJSON, "theoretical") || strings.Contains(receiptJSON, "discrepancy") || strings.Contains(receiptJSON, "185000") || strings.Contains(receiptJSON, "-3000") {
		t.Fatalf("RN-02 violation! Receptionist blind-close receipt leaked theoretical/discrepancy: %s", receiptJSON)
	}

	// RN-02 Check 4: Re-closing or adding cash movements to a CLOSED register is rejected with 409 semantics
	if _, err := reg.ExecuteBlindClose(185000, nil, "retry", "recep-01", closedAt); err != domain.ErrCashRegisterAlreadyClosed {
		t.Fatalf("expected ErrCashRegisterAlreadyClosed on second close, got %v", err)
	}
	if err := reg.RecordCashMovement("mov-4", "inv-3", domain.MovementDirectionIn, 1000, "Late payment", "recep-01", closedAt); err != domain.ErrCashRegisterAlreadyClosed {
		t.Fatalf("expected ErrCashRegisterAlreadyClosed when adding movement to closed register, got %v", err)
	}

	// S4 Check: Auditor/Admin reconciliation report exposes theoretical_cash_cents and discrepancy_cents
	auditView, err := reg.ToAuditReportView()
	if err != nil {
		t.Fatalf("expected audit report view after close, got %v", err)
	}
	if auditView.TheoreticalCashCents != 185000 {
		t.Fatalf("expected theoretical_cash_cents 185000, got %d", auditView.TheoreticalCashCents)
	}
	if auditView.DeclaredCashCents != 182000 {
		t.Fatalf("expected declared_cash_cents 182000, got %d", auditView.DeclaredCashCents)
	}
	if auditView.DiscrepancyCents != -3000 {
		t.Fatalf("expected discrepancy_cents -3000, got %d", auditView.DiscrepancyCents)
	}
}
