package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	billingapp "github.com/sigo-dental/billing-stock/internal/billing/application"
	billingdom "github.com/sigo-dental/billing-stock/internal/billing/domain"
	inventoryapp "github.com/sigo-dental/billing-stock/internal/inventory/application"
	inventorydom "github.com/sigo-dental/billing-stock/internal/inventory/domain"
	settlementapp "github.com/sigo-dental/billing-stock/internal/settlements/application"
	settlementdom "github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

// Demo dataset of the billing-stock service (work item wi-18, spec S5).
//
// The repositories of the service live in memory, so every boot starts empty
// and the billing screens have nothing to show. The dataset below is written
// by the domain APIs of every bounded context, only when SIGO_SEED_DEMO_DATA
// is enabled, and it is idempotent: a second run never duplicates a record.
//
// The cash registers are the sensitive part of the dataset (RN-02): the live
// register of the shift has no cash movement yet and therefore no theoretical
// total for anybody, while the closed register carries a declared count that
// does not match the theoretical cash, so the reconciliation figures are only
// published for auditor and admin.

const (
	demoClinicID = "CLINICA-CENTRAL"
	demoSourceIP = "127.0.0.1"

	// Canonical identities of the seeded RBAC accounts (wi-15).
	demoReceptionistID = "b1c2d3e4-2222-4a5b-8c9d-0e1f2a3b4c5e"
	demoAdminID        = "b1c2d3e4-1111-4a5b-8c9d-0e1f2a3b4c5e"
	demoAuditorID      = "b1c2d3e4-4444-4a5b-8c9d-0e1f2a3b4c5e"
	demoDentistID      = "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d"
)

const (
	// demoCashRegisterOpenID is the live register of the current shift: it
	// holds the opening float and no cash movement yet.
	demoCashRegisterOpenID = "demo-cash-register-open"
	// demoCashRegisterClosedID is the register of the previous shift, closed
	// with a declared count that does not match the theoretical cash.
	demoCashRegisterClosedID = "demo-cash-register-closed"

	// demoOpeningFloatCents is the opening float of both registers.
	demoOpeningFloatCents int64 = 50000
	// demoDeclaredCashCents is the physical count declared at the blind close
	// of the closed register.
	demoDeclaredCashCents int64 = 48000
	// demoExpectedTheoreticalCents is the theoretical cash of the closed
	// register: the opening float plus the cash collected by its invoices.
	demoExpectedTheoreticalCents int64 = 75500
	// demoExpectedDiscrepancyCents is the difference between the declared
	// count and the theoretical cash of the closed register.
	demoExpectedDiscrepancyCents int64 = -27500
)

// Fixed identifiers of the demo patients. They are the same identifiers the
// clinical-agenda demo seed uses, so the invoices point to real patients.
const (
	demoPatientAnaID       = "a1000000-0001-4000-8000-000000000001"
	demoPatientCarlosID    = "a1000000-0002-4000-8000-000000000002"
	demoPatientSofiaID     = "a1000000-0003-4000-8000-000000000003"
	demoPatientLucasID     = "a1000000-0004-4000-8000-000000000004"
	demoPatientMateoID     = "a1000000-0005-4000-8000-000000000005"
	demoPatientValentinaID = "a1000000-0006-4000-8000-000000000006"
)

// demoSeedDeps groups the collaborators the demo seed writes through. Every
// dependency is an application port, so the seed never depends on a concrete
// adapter.
type demoSeedDeps struct {
	billingRepo    billingapp.BillingRepositoryPort
	billingSvc     *billingapp.BillingService
	settlementRepo settlementapp.SettlementRepositoryPort
	settlementSvc  *settlementapp.SettlementService
	inventoryRepo  inventoryapp.InventoryRepositoryPort
	inventorySvc   *inventoryapp.InventoryService
}

// demoSummary counts the records written by a seed run, for the boot log.
type demoSummary struct {
	CashRegisters   int
	Invoices        int
	Settlements     int
	CommissionRules int
	Supplies        int
	Recipes         int
}

// demoSeeder writes the demo dataset through the domain APIs.
type demoSeeder struct {
	deps    demoSeedDeps
	now     time.Time
	summary demoSummary
	errs    []error
}

// seedDemoDataIfEnabled writes the demo dataset only when the switch
// SIGO_SEED_DEMO_DATA is enabled. The default of the switch is false.
func seedDemoDataIfEnabled(ctx context.Context, deps demoSeedDeps, now time.Time) (demoSummary, error) {
	if !envBool("SIGO_SEED_DEMO_DATA", false) {
		return demoSummary{}, nil
	}

	seeder := &demoSeeder{deps: deps, now: now.UTC()}
	seeder.seedCashRegisters(ctx)
	seeder.seedInvoices(ctx)
	seeder.seedCashClose(ctx)
	seeder.seedCommissionRules(ctx)
	seeder.seedSettlements(ctx)
	seeder.seedSupplies(ctx)
	seeder.seedRecipes(ctx)

	return seeder.summary, errors.Join(seeder.errs...)
}

func (s *demoSeeder) fail(step string, err error) {
	s.errs = append(s.errs, fmt.Errorf("demo seed %s: %w", step, err))
}

func (s *demoSeeder) receptionistActor() billingapp.ActorContext {
	return billingapp.ActorContext{
		ActorID:     demoReceptionistID,
		ActorRole:   "receptionist",
		MFAVerified: true,
		SourceIP:    demoSourceIP,
	}
}

func (s *demoSeeder) adminActor() billingapp.ActorContext {
	return billingapp.ActorContext{
		ActorID:     demoAdminID,
		ActorRole:   "admin",
		MFAVerified: true,
		SourceIP:    demoSourceIP,
	}
}

// settlementAdminActor returns the admin identity for the settlements
// context, which declares its own actor contract.
func (s *demoSeeder) settlementAdminActor() settlementapp.ActorContext {
	return settlementapp.ActorContext{
		ActorID:     demoAdminID,
		ActorRole:   "admin",
		MFAVerified: true,
		SourceIP:    demoSourceIP,
	}
}

// inventoryAdminActor returns the admin identity for the inventory context,
// which declares its own actor contract.
func (s *demoSeeder) inventoryAdminActor() inventoryapp.ActorContext {
	return inventoryapp.ActorContext{
		ActorID:     demoAdminID,
		ActorRole:   "admin",
		MFAVerified: true,
		SourceIP:    demoSourceIP,
	}
}

// ---------------------------------------------------------------------------
// Cash registers (RN-02)
// ---------------------------------------------------------------------------

func (s *demoSeeder) seedCashRegisters(ctx context.Context) {
	registers := []string{demoCashRegisterClosedID, demoCashRegisterOpenID}
	for _, id := range registers {
		if existing, err := s.deps.billingRepo.GetCashRegisterByID(ctx, id); err == nil && existing != nil {
			continue
		}
		if _, err := s.deps.billingSvc.OpenCashRegister(ctx, s.receptionistActor(), billingapp.OpenCashRegisterCommand{
			ID:                id,
			ClinicID:          demoClinicID,
			OpeningFloatCents: demoOpeningFloatCents,
		}); err != nil {
			s.fail("cash registers", err)
			continue
		}
		s.summary.CashRegisters++
	}
}

// seedCashClose closes the register of the previous shift with a declared
// count that does not match the theoretical cash, so the reconciliation of
// RN-02 can be exercised with a real discrepancy.
func (s *demoSeeder) seedCashClose(ctx context.Context) {
	reg, err := s.deps.billingRepo.GetCashRegisterByID(ctx, demoCashRegisterClosedID)
	if err != nil {
		s.fail("cash close", err)
		return
	}
	if reg.Status == billingdom.CashRegisterStatusClosed {
		return
	}
	if _, err := s.deps.billingSvc.ExecuteBlindClose(ctx, s.receptionistActor(), billingapp.BlindCloseCommand{
		CashRegisterID:    demoCashRegisterClosedID,
		DeclaredCashCents: demoDeclaredCashCents,
		Denominations: map[string]int{
			"20":  6,
			"10":  4,
			"5":   8,
			"1":   60,
			"0.5": 40,
		},
		CashierNotes: "Conteo físico declarado por recepción; el monto declarado no coincide con el teórico.",
	}); err != nil {
		s.fail("cash close", err)
	}
}

// ---------------------------------------------------------------------------
// Invoices (RF-12)
// ---------------------------------------------------------------------------

// demoInvoice describes one invoice of the demo dataset. The amounts are
// expressed in cents and the health services are IVA exempt.
type demoInvoice struct {
	ID            string
	PatientID     string
	CustomerTaxID string
	CustomerName  string
	RegisterID    string
	// Stamped marks the invoices issued through the billing service, which
	// stamps them with the tax authority (fiscal status AUTHORIZED). The
	// unstamped ones are created through the domain and stay PENDING.
	Stamped  bool
	Items    []billingdom.InvoiceItemInput
	Payments []billingdom.InvoicePaymentInput
}

var demoInvoices = []demoInvoice{
	{
		ID:            "demo-inv-001",
		PatientID:     demoPatientAnaID,
		CustomerTaxID: "0912345678",
		CustomerName:  "Ana Lucía Fernández",
		RegisterID:    demoCashRegisterClosedID,
		Stamped:       true,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D2391", Description: "Resina compuesta posterior", Quantity: 1, UnitPriceCents: 12000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodCash, AmountCents: 12000, ExternalReference: "CAJA-2026-10-29-01"},
		},
	},
	{
		ID:            "demo-inv-002",
		PatientID:     demoPatientMateoID,
		CustomerTaxID: "0712345672",
		CustomerName:  "Mateo Emiliano Cruz",
		RegisterID:    demoCashRegisterClosedID,
		Stamped:       true,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D7140", Description: "Exodoncia", Quantity: 1, UnitPriceCents: 9000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodCash, AmountCents: 9000, ExternalReference: "CAJA-2026-10-29-02"},
		},
	},
	{
		ID:            "demo-inv-003",
		PatientID:     demoPatientSofiaID,
		CustomerTaxID: "1712345670",
		CustomerName:  "Sofía Ramírez Ortega",
		RegisterID:    demoCashRegisterClosedID,
		Stamped:       true,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D1110", Description: "Profilaxis dental", Quantity: 1, UnitPriceCents: 4500},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodCash, AmountCents: 4500, ExternalReference: "CAJA-2026-10-29-03"},
		},
	},
	{
		ID:            "demo-inv-004",
		PatientID:     demoPatientAnaID,
		CustomerTaxID: "0912345678",
		CustomerName:  "Ana Lucía Fernández",
		RegisterID:    demoCashRegisterOpenID,
		Stamped:       true,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D2392", Description: "Resina compuesta anterior", Quantity: 1, UnitPriceCents: 9000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodCreditCard, AmountCents: 9000, ExternalReference: "TC-2026-000431"},
		},
	},
	{
		ID:            "demo-inv-005",
		PatientID:     demoPatientValentinaID,
		CustomerTaxID: "1012345673",
		CustomerName:  "Valentina Armas",
		RegisterID:    demoCashRegisterOpenID,
		Stamped:       true,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D6010", Description: "Corona de porcelana", Quantity: 1, UnitPriceCents: 38000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodDentalInsurance, AmountCents: 38000, ExternalReference: "SEG-2026-0099"},
		},
	},
	{
		ID:            "demo-inv-006",
		PatientID:     demoPatientLucasID,
		CustomerTaxID: "2112345671",
		CustomerName:  "Lucas Mateo Díaz",
		RegisterID:    demoCashRegisterOpenID,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D1110", Description: "Profilaxis dental", Quantity: 1, UnitPriceCents: 4500},
			{ProcedureCode: "D1210", Description: "Destartraje por sextante", Quantity: 1, UnitPriceCents: 3000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodDebitCard, AmountCents: 7500, ExternalReference: "TD-2026-000512"},
		},
	},
	{
		ID:            "demo-inv-007",
		PatientID:     demoPatientCarlosID,
		CustomerTaxID: "1312345679",
		CustomerName:  "Carlos Andrés Mendoza",
		RegisterID:    demoCashRegisterOpenID,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D3330", Description: "Endodoncia de molar", Quantity: 1, UnitPriceCents: 45000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodBankTransfer, AmountCents: 45000, ExternalReference: "TRANS-2026-0071"},
		},
	},
	{
		ID:            "demo-inv-008",
		PatientID:     demoPatientMateoID,
		CustomerTaxID: "0712345672",
		CustomerName:  "Mateo Emiliano Cruz",
		RegisterID:    demoCashRegisterOpenID,
		Items: []billingdom.InvoiceItemInput{
			{ProcedureCode: "D6010", Description: "Corona de porcelana", Quantity: 1, UnitPriceCents: 25000},
		},
		Payments: []billingdom.InvoicePaymentInput{
			{Method: billingdom.PaymentMethodCreditCard, AmountCents: 15000, ExternalReference: "TC-2026-000433"},
			{Method: billingdom.PaymentMethodDebitCard, AmountCents: 10000, ExternalReference: "TD-2026-000514"},
		},
	},
}

func (s *demoSeeder) seedInvoices(ctx context.Context) {
	for _, inv := range demoInvoices {
		if s.invoiceExists(ctx, inv.ID) {
			continue
		}
		if inv.Stamped {
			if _, err := s.deps.billingSvc.IssueInvoice(ctx, s.receptionistActor(), billingapp.IssueInvoiceCommand{
				ID:             inv.ID,
				PatientID:      inv.PatientID,
				CashRegisterID: inv.RegisterID,
				CustomerTaxID:  inv.CustomerTaxID,
				CustomerName:   inv.CustomerName,
				Items:          inv.Items,
				Payments:       inv.Payments,
			}); err != nil {
				s.fail("invoices", err)
				continue
			}
			s.summary.Invoices++
			continue
		}

		// Unstamped invoice: it is created through the domain and stays in
		// PENDING fiscal status until the tax authority stamps it.
		created, err := billingdom.NewInvoice(billingdom.CreateInvoiceParams{
			ID:             inv.ID,
			PatientID:      inv.PatientID,
			CashRegisterID: inv.RegisterID,
			CustomerTaxID:  inv.CustomerTaxID,
			CustomerName:   inv.CustomerName,
			Items:          inv.Items,
			Payments:       inv.Payments,
			IssuedBy:       s.receptionistActor().ActorID,
			SourceIP:       demoSourceIP,
			CreatedAt:      s.now,
		})
		if err != nil {
			s.fail("invoices", err)
			continue
		}
		reg, err := s.deps.billingRepo.GetCashRegisterByID(ctx, inv.RegisterID)
		if err != nil {
			s.fail("invoices", err)
			continue
		}
		if err := s.deps.billingRepo.SaveInvoiceAndCashMovement(ctx, created, reg); err != nil {
			s.fail("invoices", err)
			continue
		}
		s.summary.Invoices++
	}
}

// invoiceExists reports whether the invoice is already part of the dataset.
func (s *demoSeeder) invoiceExists(ctx context.Context, id string) bool {
	invoices, _, err := s.deps.billingRepo.ListInvoices(ctx, billingapp.InvoiceFilter{Limit: 200})
	if err != nil {
		return false
	}
	for _, inv := range invoices {
		if inv.ID == id {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Commission rules and settlements (RF-13)
// ---------------------------------------------------------------------------

// demoCommissionRule describes one commission rule of the demo dataset.
type demoCommissionRule struct {
	ID               string
	ProcedureCode    string
	CalculationMode  string
	RateBPS          int
	FixedAmountCents int64
}

var demoCommissionRules = []demoCommissionRule{
	{ID: "demo-commission-001", ProcedureCode: "D1110", CalculationMode: settlementdom.ModeFixedPercentage, RateBPS: 3500},
	{ID: "demo-commission-002", ProcedureCode: "D2391", CalculationMode: settlementdom.ModeNetPercentageAfterCosts, RateBPS: 4000},
	{ID: "demo-commission-003", ProcedureCode: "D7140", CalculationMode: settlementdom.ModeFixedAmount, FixedAmountCents: 5000},
	// Wildcard rule: it is the fallback of every procedure without an
	// explicit rule.
	{ID: "demo-commission-004", ProcedureCode: "*", CalculationMode: settlementdom.ModeFixedPercentage, RateBPS: 3000},
}

func (s *demoSeeder) seedCommissionRules(ctx context.Context) {
	for _, rule := range demoCommissionRules {
		if _, err := s.deps.settlementSvc.ConfigureCommissionRule(ctx, s.settlementAdminActor(), settlementapp.ConfigureRuleCommand{
			ID:               rule.ID,
			DentistID:        demoDentistID,
			ProcedureCode:    rule.ProcedureCode,
			CalculationMode:  rule.CalculationMode,
			RateBPS:          rule.RateBPS,
			FixedAmountCents: rule.FixedAmountCents,
		}); err != nil {
			s.fail("commission rules", err)
			continue
		}
		s.summary.CommissionRules++
	}
}

// demoSettlement describes one settlement of the demo dataset.
type demoSettlement struct {
	ID          string
	PeriodStart string
	PeriodEnd   string
	Status      string
	Procedures  []settlementdom.CompletedProcedureInput
}

var demoSettlements = []demoSettlement{
	{
		ID:          "demo-settlement-001",
		PeriodStart: "2026-09-01",
		PeriodEnd:   "2026-09-30",
		Status:      settlementdom.SettlementStatusDraft,
		Procedures: []settlementdom.CompletedProcedureInput{
			{CompletedProcedureID: "demo-procedure-001", ProcedureCode: "D1110", GrossFeeCents: 4500, MaterialCostCents: 500},
			{CompletedProcedureID: "demo-procedure-002", ProcedureCode: "D2391", GrossFeeCents: 12000, LabCostCents: 2000, MaterialCostCents: 800},
		},
	},
	{
		ID:          "demo-settlement-002",
		PeriodStart: "2026-08-01",
		PeriodEnd:   "2026-08-31",
		Status:      settlementdom.SettlementStatusApproved,
		Procedures: []settlementdom.CompletedProcedureInput{
			{CompletedProcedureID: "demo-procedure-003", ProcedureCode: "D7140", GrossFeeCents: 9000, MaterialCostCents: 600},
			{CompletedProcedureID: "demo-procedure-004", ProcedureCode: "D1110", GrossFeeCents: 4500, MaterialCostCents: 500},
		},
	},
	{
		ID:          "demo-settlement-003",
		PeriodStart: "2026-07-01",
		PeriodEnd:   "2026-07-31",
		Status:      settlementdom.SettlementStatusPaid,
		Procedures: []settlementdom.CompletedProcedureInput{
			{CompletedProcedureID: "demo-procedure-005", ProcedureCode: "D6010", GrossFeeCents: 38000, LabCostCents: 12000, MaterialCostCents: 1500},
		},
	},
}

func (s *demoSeeder) seedSettlements(ctx context.Context) {
	for _, spec := range demoSettlements {
		if s.settlementExists(ctx, spec.ID) {
			continue
		}
		settlement, err := settlementdom.BuildDentistSettlement(
			spec.ID,
			demoDentistID,
			spec.PeriodStart,
			spec.PeriodEnd,
			s.adminActor().ActorID,
			spec.Procedures,
			func(procedureCode string) (*settlementdom.CommissionRule, error) {
				return s.deps.settlementRepo.GetCommissionRule(ctx, demoDentistID, procedureCode)
			},
		)
		if err != nil {
			s.fail("settlements", err)
			continue
		}
		// The settlement aggregate has no state machine: the status is a
		// plain field, so the demo dataset can show the three states of the
		// settlement lifecycle.
		settlement.Status = spec.Status
		if err := s.deps.settlementRepo.SaveSettlement(ctx, settlement); err != nil {
			s.fail("settlements", err)
			continue
		}
		s.summary.Settlements++
	}
}

// settlementExists reports whether the settlement is already part of the
// dataset.
func (s *demoSeeder) settlementExists(ctx context.Context, id string) bool {
	settlements, _, err := s.deps.settlementRepo.ListSettlements(ctx, settlementapp.SettlementFilter{Limit: 200})
	if err != nil {
		return false
	}
	for _, settlement := range settlements {
		if settlement.SettlementID == id {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Inventory (RF-14)
// ---------------------------------------------------------------------------

// demoSupply describes one supply of the demo dataset.
type demoSupply struct {
	ID               string
	SKU              string
	Name             string
	Unit             string
	BatchNumber      string
	ExpiryDate       string
	CurrentStock     int
	MinStockQuantity int
	UnitCostCents    int64
}

var demoSupplies = []demoSupply{
	{ID: "demo-supply-001", SKU: "INS-001", Name: "Guantes de nitrilo talla M", Unit: "caja", BatchNumber: "LOTE-2026-014", ExpiryDate: "2028-06-30", CurrentStock: 40, MinStockQuantity: 20, UnitCostCents: 850},
	{ID: "demo-supply-002", SKU: "INS-002", Name: "Mascarilla quirúrgica", Unit: "caja", BatchNumber: "LOTE-2026-022", ExpiryDate: "2027-03-31", CurrentStock: 12, MinStockQuantity: 12, UnitCostCents: 1200},
	{ID: "demo-supply-003", SKU: "INS-003", Name: "Anestesia lidocaína 2%", Unit: "ampolla", BatchNumber: "LOTE-2026-031", ExpiryDate: "2027-01-31", CurrentStock: 5, MinStockQuantity: 15, UnitCostCents: 350},
	{ID: "demo-supply-004", SKU: "INS-004", Name: "Ácido grabador 37%", Unit: "frasco", BatchNumber: "LOTE-2026-007", ExpiryDate: "2027-09-30", CurrentStock: 6, MinStockQuantity: 5, UnitCostCents: 450},
	{ID: "demo-supply-005", SKU: "INS-005", Name: "Cepillo dental suave", Unit: "unidad", BatchNumber: "LOTE-2026-045", ExpiryDate: "2029-12-31", CurrentStock: 3, MinStockQuantity: 10, UnitCostCents: 250},
	{ID: "demo-supply-006", SKU: "INS-006", Name: "Composite A2", Unit: "jeringa", BatchNumber: "LOTE-2026-012", ExpiryDate: "2027-05-31", CurrentStock: 18, MinStockQuantity: 6, UnitCostCents: 4200},
	{ID: "demo-supply-007", SKU: "INS-007", Name: "Adhesivo dentinario", Unit: "jeringa", BatchNumber: "LOTE-2026-013", ExpiryDate: "2027-04-30", CurrentStock: 9, MinStockQuantity: 4, UnitCostCents: 3100},
	{ID: "demo-supply-008", SKU: "INS-008", Name: "Seda dental encerada", Unit: "unidad", BatchNumber: "LOTE-2026-051", ExpiryDate: "2030-01-31", CurrentStock: 25, MinStockQuantity: 10, UnitCostCents: 90},
	{ID: "demo-supply-009", SKU: "INS-009", Name: "Cepillo interproximal 0.6 mm", Unit: "unidad", BatchNumber: "LOTE-2026-058", ExpiryDate: "2030-01-31", CurrentStock: 7, MinStockQuantity: 8, UnitCostCents: 180},
	{ID: "demo-supply-010", SKU: "INS-010", Name: "Anestesia mepivacaína 3%", Unit: "ampolla", BatchNumber: "LOTE-2026-032", ExpiryDate: "2027-02-28", CurrentStock: 14, MinStockQuantity: 6, UnitCostCents: 420},
}

func (s *demoSeeder) seedSupplies(ctx context.Context) {
	for _, supply := range demoSupplies {
		if _, err := s.deps.inventorySvc.UpsertSupply(ctx, s.inventoryAdminActor(), inventoryapp.UpsertSupplyCommand{
			ID:               supply.ID,
			ClinicID:         demoClinicID,
			SKU:              supply.SKU,
			Name:             supply.Name,
			Unit:             supply.Unit,
			BatchNumber:      supply.BatchNumber,
			ExpiryDate:       supply.ExpiryDate,
			CurrentStock:     supply.CurrentStock,
			MinStockQuantity: supply.MinStockQuantity,
			UnitCostCents:    supply.UnitCostCents,
		}); err != nil {
			s.fail("supplies", err)
			continue
		}
		s.summary.Supplies++
	}
}

// demoRecipe describes one procedure recipe of the demo dataset.
type demoRecipe struct {
	ProcedureCode string
	Items         []inventorydom.RecipeItem
}

var demoRecipes = []demoRecipe{
	{
		ProcedureCode: "D1110",
		Items: []inventorydom.RecipeItem{
			{SupplySKU: "INS-008", QuantityPerProcedure: 1},
			{SupplySKU: "INS-001", QuantityPerProcedure: 1},
			{SupplySKU: "INS-002", QuantityPerProcedure: 1},
		},
	},
	{
		ProcedureCode: "D2391",
		Items: []inventorydom.RecipeItem{
			{SupplySKU: "INS-006", QuantityPerProcedure: 1},
			{SupplySKU: "INS-007", QuantityPerProcedure: 1},
			{SupplySKU: "INS-001", QuantityPerProcedure: 1},
		},
	},
	{
		ProcedureCode: "D3330",
		Items: []inventorydom.RecipeItem{
			{SupplySKU: "INS-003", QuantityPerProcedure: 2},
			{SupplySKU: "INS-001", QuantityPerProcedure: 1},
		},
	},
	{
		ProcedureCode: "D7140",
		Items: []inventorydom.RecipeItem{
			{SupplySKU: "INS-003", QuantityPerProcedure: 1},
			{SupplySKU: "INS-002", QuantityPerProcedure: 1},
			{SupplySKU: "INS-001", QuantityPerProcedure: 1},
		},
	},
}

func (s *demoSeeder) seedRecipes(ctx context.Context) {
	for _, recipe := range demoRecipes {
		if _, err := s.deps.inventorySvc.SaveProcedureRecipe(ctx, s.inventoryAdminActor(), inventoryapp.SaveRecipeCommand{
			ProcedureCode: recipe.ProcedureCode,
			Items:         recipe.Items,
		}); err != nil {
			s.fail("recipes", err)
			continue
		}
		s.summary.Recipes++
	}
}

// demoDatasetLabel returns a short human readable label of the dataset, used
// by the boot log.
func demoDatasetLabel(summary demoSummary) string {
	parts := []string{
		fmt.Sprintf("cash_registers=%d", summary.CashRegisters),
		fmt.Sprintf("invoices=%d", summary.Invoices),
		fmt.Sprintf("settlements=%d", summary.Settlements),
		fmt.Sprintf("commission_rules=%d", summary.CommissionRules),
		fmt.Sprintf("supplies=%d", summary.Supplies),
		fmt.Sprintf("recipes=%d", summary.Recipes),
	}
	return strings.Join(parts, " ")
}
