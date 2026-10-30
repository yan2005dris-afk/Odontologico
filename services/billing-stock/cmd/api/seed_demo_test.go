package main

import (
	"context"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/billing-stock/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/billing-stock/internal/audit/application"
	billingpg "github.com/sigo-dental/billing-stock/internal/billing/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/billing/adapters/fiscal"
	billingapp "github.com/sigo-dental/billing-stock/internal/billing/application"
	billingdom "github.com/sigo-dental/billing-stock/internal/billing/domain"
	inventorypg "github.com/sigo-dental/billing-stock/internal/inventory/adapters/postgres"
	inventoryapp "github.com/sigo-dental/billing-stock/internal/inventory/application"
	settlementpg "github.com/sigo-dental/billing-stock/internal/settlements/adapters/postgres"
	settlementapp "github.com/sigo-dental/billing-stock/internal/settlements/application"
)

// demoFixture wires the same repositories and services the service uses on
// boot so the seed can be exercised without the HTTP layer.
type demoFixture struct {
	deps        demoSeedDeps
	billingRepo billingapp.BillingRepositoryPort
	settleRepo  settlementapp.SettlementRepositoryPort
	inventRepo  inventoryapp.InventoryRepositoryPort
}

func newDemoFixture() *demoFixture {
	auditSvc := auditapp.NewAuditService(auditpg.NewAuditRepository(), nil, nil)
	billingRepo := billingpg.NewInMemoryBillingRepository()
	settlementRepo := settlementpg.NewInMemorySettlementRepository()
	inventoryRepo := inventorypg.NewInMemoryInventoryRepository()

	return &demoFixture{
		deps: demoSeedDeps{
			billingRepo:    billingRepo,
			billingSvc:     billingapp.NewBillingService(billingRepo, fiscal.NewTaxAuthorityClient(), fiscal.NewPaymentGatewayClient(), auditSvc),
			settlementRepo: settlementRepo,
			settlementSvc:  settlementapp.NewSettlementService(settlementRepo, auditSvc),
			inventoryRepo:  inventoryRepo,
			inventorySvc:   inventoryapp.NewInventoryService(inventoryRepo, auditSvc),
		},
		billingRepo: billingRepo,
		settleRepo:  settlementRepo,
		inventRepo:  inventoryRepo,
	}
}

// TestSeedDemoData_DisabledByDefault covers S0: the demo dataset is never
// seeded unless SIGO_SEED_DEMO_DATA is explicitly enabled.
func TestSeedDemoData_DisabledByDefault(t *testing.T) {
	t.Setenv("SIGO_SEED_DEMO_DATA", "")
	fixture := newDemoFixture()

	summary, err := seedDemoDataIfEnabled(context.Background(), fixture.deps, time.Now().UTC())
	if err != nil {
		t.Fatalf("expected the disabled seed to succeed, got %v", err)
	}
	if summary.Invoices != 0 || summary.CashRegisters != 0 {
		t.Fatalf("expected no demo data when the switch is off, got %+v", summary)
	}

	_, total, err := fixture.billingRepo.ListInvoices(context.Background(), billingapp.InvoiceFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the invoice listing to succeed, got %v", err)
	}
	if total != 0 {
		t.Fatalf("expected an empty repository when the switch is off, got %d invoices", total)
	}
}

// TestSeedDemoData_CreatesTheFinancialDemoDataset covers S5: the seed creates
// the invoices, the cash registers, the settlements, the commission rules, the
// supplies and the procedure recipes of the demo dataset.
func TestSeedDemoData_CreatesTheFinancialDemoDataset(t *testing.T) {
	t.Setenv("SIGO_SEED_DEMO_DATA", "true")
	ctx := context.Background()
	fixture := newDemoFixture()

	summary, err := seedDemoDataIfEnabled(ctx, fixture.deps, time.Date(2026, 10, 30, 8, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected the demo seed to succeed, got %v", err)
	}
	if summary.Invoices != 8 {
		t.Fatalf("expected 8 seeded invoices, got %d", summary.Invoices)
	}
	if summary.CashRegisters != 2 {
		t.Fatalf("expected 2 seeded cash registers, got %d", summary.CashRegisters)
	}
	if summary.Settlements != 3 {
		t.Fatalf("expected 3 seeded settlements, got %d", summary.Settlements)
	}
	if summary.CommissionRules != 4 {
		t.Fatalf("expected 4 seeded commission rules, got %d", summary.CommissionRules)
	}
	if summary.Supplies != 10 {
		t.Fatalf("expected 10 seeded supplies, got %d", summary.Supplies)
	}
	if summary.Recipes != 4 {
		t.Fatalf("expected 4 seeded procedure recipes, got %d", summary.Recipes)
	}

	// Invoices: both fiscal states of the domain are covered.
	invoices, total, err := fixture.billingRepo.ListInvoices(ctx, billingapp.InvoiceFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the invoice listing to succeed, got %v", err)
	}
	if total != 8 {
		t.Fatalf("expected 8 invoices, got %d", total)
	}
	fiscalStatuses := make(map[string]bool)
	paymentMethods := make(map[string]bool)
	for _, inv := range invoices {
		fiscalStatuses[inv.FiscalStatus()] = true
		paymentMethods[inv.PaymentMethod()] = true
		if inv.TotalCents <= 0 {
			t.Fatalf("expected a positive invoice total, got %d", inv.TotalCents)
		}
	}
	for _, want := range []string{billingdom.InvoiceFiscalStatusPending, billingdom.InvoiceFiscalStatusAuthorized} {
		if !fiscalStatuses[want] {
			t.Fatalf("expected an invoice with the fiscal status %q, got %v", want, fiscalStatuses)
		}
	}
	for _, want := range []string{
		billingdom.PaymentMethodCash,
		billingdom.PaymentMethodCreditCard,
		billingdom.PaymentMethodDebitCard,
		billingdom.PaymentMethodBankTransfer,
		billingdom.PaymentMethodDentalInsurance,
		billingdom.InvoicePaymentMethodMixed,
	} {
		if !paymentMethods[want] {
			t.Fatalf("expected an invoice paid with %q, got %v", want, paymentMethods)
		}
	}

	// Inventory: at least 3 supplies at (or below) their minimum stock so the
	// derived low_stock flag is visible in the inventory screen.
	supplies, _, err := fixture.inventRepo.ListSupplies(ctx, inventoryapp.SupplyFilter{ClinicID: demoClinicID, Limit: 100})
	if err != nil {
		t.Fatalf("expected the supply listing to succeed, got %v", err)
	}
	if len(supplies) != 10 {
		t.Fatalf("expected 10 seeded supplies, got %d", len(supplies))
	}
	lowStock := 0
	for _, sup := range supplies {
		if sup.CurrentStock <= sup.MinStockQuantity {
			lowStock++
		}
	}
	if lowStock < 3 {
		t.Fatalf("expected at least 3 supplies at or below the minimum stock, got %d", lowStock)
	}

	recipes, _, err := fixture.inventRepo.ListRecipes(ctx, inventoryapp.RecipeFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the recipe listing to succeed, got %v", err)
	}
	if len(recipes) != 4 {
		t.Fatalf("expected 4 seeded recipes, got %d", len(recipes))
	}

	rules, _, err := fixture.settleRepo.ListCommissionRules(ctx, settlementapp.CommissionRuleFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the commission rule listing to succeed, got %v", err)
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 seeded commission rules, got %d", len(rules))
	}

	settlements, _, err := fixture.settleRepo.ListSettlements(ctx, settlementapp.SettlementFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the settlement listing to succeed, got %v", err)
	}
	if len(settlements) != 3 {
		t.Fatalf("expected 3 seeded settlements, got %d", len(settlements))
	}
}

// TestSeedDemoData_BlindCashCloseHidesTheTheoreticalCashFromTheReceptionist
// is the RN-02 guarantee: the listing of the cash registers must not expose
// the theoretical total nor the discrepancy to a receptionist, while the
// auditor (and the admin) do see them once the register is closed.
func TestSeedDemoData_BlindCashCloseHidesTheTheoreticalCashFromTheReceptionist(t *testing.T) {
	t.Setenv("SIGO_SEED_DEMO_DATA", "true")
	ctx := context.Background()
	fixture := newDemoFixture()

	if _, err := seedDemoDataIfEnabled(ctx, fixture.deps, time.Date(2026, 10, 30, 8, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("expected the demo seed to succeed, got %v", err)
	}

	receptionist := billingapp.ActorContext{
		ActorID:     demoReceptionistID,
		ActorRole:   "receptionist",
		MFAVerified: true,
		SourceIP:    demoSourceIP,
	}
	auditor := billingapp.ActorContext{
		ActorID:     demoAuditorID,
		ActorRole:   "auditor",
		MFAVerified: true,
		SourceIP:    demoSourceIP,
	}

	receptionistView, _, err := fixture.deps.billingSvc.ListCashRegisters(ctx, receptionist, billingapp.CashRegisterFilter{ClinicID: demoClinicID, Limit: 100})
	if err != nil {
		t.Fatalf("expected the cash register listing to succeed, got %v", err)
	}
	auditorView, _, err := fixture.deps.billingSvc.ListCashRegisters(ctx, auditor, billingapp.CashRegisterFilter{ClinicID: demoClinicID, Limit: 100})
	if err != nil {
		t.Fatalf("expected the cash register listing to succeed, got %v", err)
	}
	if len(receptionistView) != 2 {
		t.Fatalf("expected 2 cash registers, got %d", len(receptionistView))
	}

	for _, view := range receptionistView {
		if view.TheoreticalCents != nil {
			t.Fatalf("RN-02: the receptionist must not see the theoretical cash of %q", view.ID)
		}
		if view.DiscrepancyCents != nil {
			t.Fatalf("RN-02: the receptionist must not see the discrepancy of %q", view.ID)
		}
	}

	openView, closedView := findRegisterViews(receptionistView)
	if openView == nil || closedView == nil {
		t.Fatalf("expected one OPEN and one CLOSED register, got %+v", receptionistView)
	}
	if openView.Status != billingdom.CashRegisterStatusOpen {
		t.Fatalf("expected an OPEN register, got %q", openView.Status)
	}
	if closedView.Status != billingdom.CashRegisterStatusClosed {
		t.Fatalf("expected a CLOSED register, got %q", closedView.Status)
	}
	if closedView.DeclaredCents == nil {
		t.Fatal("expected the declared cash of the closed register to be visible")
	}

	auditorOpen, auditorClosed := findRegisterViews(auditorView)
	if auditorOpen == nil || auditorClosed == nil {
		t.Fatalf("expected one OPEN and one CLOSED register for the auditor, got %+v", auditorView)
	}
	if auditorOpen.TheoreticalCents != nil || auditorOpen.DiscrepancyCents != nil {
		t.Fatalf("RN-02: an OPEN register has no theoretical cash for anybody, got %+v", auditorOpen)
	}
	if auditorClosed.TheoreticalCents == nil {
		t.Fatal("expected the auditor to see the theoretical cash of the closed register")
	}
	if auditorClosed.DiscrepancyCents == nil {
		t.Fatal("expected the auditor to see the discrepancy of the closed register")
	}
	if *auditorClosed.DiscrepancyCents == 0 {
		t.Fatal("expected a non zero discrepancy in the closed register of the demo dataset")
	}
	if *auditorClosed.TheoreticalCents != demoExpectedTheoreticalCents {
		t.Fatalf("expected a theoretical cash of %d, got %d", demoExpectedTheoreticalCents, *auditorClosed.TheoreticalCents)
	}
	if *auditorClosed.DiscrepancyCents != demoExpectedDiscrepancyCents {
		t.Fatalf("expected a discrepancy of %d, got %d", demoExpectedDiscrepancyCents, *auditorClosed.DiscrepancyCents)
	}

	// The live register of the shift has no cash movement yet.
	liveRegister, err := fixture.billingRepo.GetCashRegisterByID(ctx, demoCashRegisterOpenID)
	if err != nil {
		t.Fatalf("expected the open register to exist, got %v", err)
	}
	if len(liveRegister.Movements) != 0 {
		t.Fatalf("expected the open register to have no cash movement, got %d", len(liveRegister.Movements))
	}

	// The reconciliation report of the live register is forbidden for every
	// role, including the admin.
	if _, err := fixture.deps.billingSvc.GetAuditReport(ctx, billingapp.ActorContext{
		ActorID:     demoAdminID,
		ActorRole:   "admin",
		MFAVerified: true,
	}, demoCashRegisterOpenID); err != billingdom.ErrBlindCashCloseEnforced {
		t.Fatalf("expected the blind cash close to be enforced for the open register, got %v", err)
	}

	report, err := fixture.deps.billingSvc.GetAuditReport(ctx, auditor, demoCashRegisterClosedID)
	if err != nil {
		t.Fatalf("expected the audit report of the closed register to succeed, got %v", err)
	}
	if report.TheoreticalCashCents != demoExpectedTheoreticalCents {
		t.Fatalf("expected a theoretical cash of %d, got %d", demoExpectedTheoreticalCents, report.TheoreticalCashCents)
	}
	if report.DiscrepancyCents != demoExpectedDiscrepancyCents {
		t.Fatalf("expected a discrepancy of %d, got %d", demoExpectedDiscrepancyCents, report.DiscrepancyCents)
	}
}

// TestSeedDemoData_IsIdempotent covers S5: replaying the seed never
// duplicates a record and never fails.
func TestSeedDemoData_IsIdempotent(t *testing.T) {
	t.Setenv("SIGO_SEED_DEMO_DATA", "true")
	ctx := context.Background()
	fixture := newDemoFixture()
	now := time.Date(2026, 10, 30, 8, 30, 0, 0, time.UTC)

	if _, err := seedDemoDataIfEnabled(ctx, fixture.deps, now); err != nil {
		t.Fatalf("expected the first seed run to succeed, got %v", err)
	}
	if _, err := seedDemoDataIfEnabled(ctx, fixture.deps, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("expected the second seed run to be idempotent, got %v", err)
	}

	_, invoices, err := fixture.billingRepo.ListInvoices(ctx, billingapp.InvoiceFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the invoice listing to succeed, got %v", err)
	}
	if invoices != 8 {
		t.Fatalf("expected still 8 invoices after re-seeding, got %d", invoices)
	}

	_, registers, err := fixture.billingRepo.ListCashRegisters(ctx, billingapp.CashRegisterFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the cash register listing to succeed, got %v", err)
	}
	if registers != 2 {
		t.Fatalf("expected still 2 cash registers after re-seeding, got %d", registers)
	}

	closed, err := fixture.billingRepo.GetCashRegisterByID(ctx, demoCashRegisterClosedID)
	if err != nil {
		t.Fatalf("expected the closed register to exist, got %v", err)
	}
	if len(closed.Movements) != 3 {
		t.Fatalf("expected still 3 cash movements after re-seeding, got %d", len(closed.Movements))
	}
	if closed.Status != billingdom.CashRegisterStatusClosed {
		t.Fatalf("expected the register to stay closed, got %q", closed.Status)
	}

	_, settlements, err := fixture.settleRepo.ListSettlements(ctx, settlementapp.SettlementFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the settlement listing to succeed, got %v", err)
	}
	if settlements != 3 {
		t.Fatalf("expected still 3 settlements after re-seeding, got %d", settlements)
	}

	_, supplies, err := fixture.inventRepo.ListSupplies(ctx, inventoryapp.SupplyFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the supply listing to succeed, got %v", err)
	}
	if supplies != 10 {
		t.Fatalf("expected still 10 supplies after re-seeding, got %d", supplies)
	}
}

// findRegisterViews splits a cash register listing into the OPEN register and
// the CLOSED register.
func findRegisterViews(views []billingdom.CashRegisterListItemView) (*billingdom.CashRegisterListItemView, *billingdom.CashRegisterListItemView) {
	var open, closed *billingdom.CashRegisterListItemView
	for i := range views {
		switch views[i].Status {
		case billingdom.CashRegisterStatusOpen:
			open = &views[i]
		case billingdom.CashRegisterStatusClosed:
			closed = &views[i]
		}
	}
	return open, closed
}
