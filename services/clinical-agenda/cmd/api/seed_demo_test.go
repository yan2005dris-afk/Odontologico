package main

import (
	"context"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	clinicalpg "github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/postgres"
	clinicalapp "github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	patientpg "github.com/sigo-dental/clinical-agenda/internal/patients/adapters/postgres"
	patientapp "github.com/sigo-dental/clinical-agenda/internal/patients/application"
	patientdom "github.com/sigo-dental/clinical-agenda/internal/patients/domain"
	schedulingpg "github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/postgres"
	schedulingapp "github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	schedulingdom "github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
	treatmentpg "github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/postgres"
	treatmentapp "github.com/sigo-dental/clinical-agenda/internal/treatments/application"
)

// demoFixture wires the same repositories the service uses on boot so the
// seed can be exercised without the HTTP layer.
type demoFixture struct {
	deps       demoSeedDeps
	patients   patientapp.PatientRepository
	scheduling schedulingapp.SchedulingRepository
	treatments treatmentapp.TreatmentRepository
	odontogram clinicalapp.OdontogramRepository
	periodonto clinicalapp.PeriodontogramRepository
	soep       clinicalapp.SOEPRepository
	consents   clinicalapp.ConsentRepository
}

func newDemoFixture() *demoFixture {
	auditSvc := auditapp.NewAuditService(auditpg.NewAuditRepository(), nil, nil)
	patientRepo := patientpg.NewPatientRepository()
	schedulingRepo := schedulingpg.NewSchedulingRepository()
	treatmentRepo := treatmentpg.NewTreatmentRepository()
	odoRepo := clinicalpg.NewOdontogramRepository()
	perioRepo := clinicalpg.NewPeriodontogramRepository()
	soepRepo := clinicalpg.NewSOEPRepository()
	consentRepo := clinicalpg.NewConsentRepository()

	return &demoFixture{
		deps: demoSeedDeps{
			audit:      auditSvc,
			patients:   patientRepo,
			scheduling: schedulingRepo,
			treatments: treatmentRepo,
			odontogram: odoRepo,
			periodonto: perioRepo,
			soep:       soepRepo,
			consents:   consentRepo,
		},
		patients:   patientRepo,
		scheduling: schedulingRepo,
		treatments: treatmentRepo,
		odontogram: odoRepo,
		periodonto: perioRepo,
		soep:       soepRepo,
		consents:   consentRepo,
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
	if summary.Patients != 0 {
		t.Fatalf("expected no demo data when the switch is off, got %+v", summary)
	}

	_, total, err := fixture.patients.Search(context.Background(), "", 100, 0)
	if err != nil {
		t.Fatalf("expected the patient search to succeed, got %v", err)
	}
	if total != 0 {
		t.Fatalf("expected an empty repository when the switch is off, got %d patients", total)
	}
}

// TestSeedDemoData_CreatesTheClinicalDemoDataset covers S4: the seed creates
// the 6 patients, the agenda of the day on 3 chairs without any chair
// overlap, one treatment plan per domain status, one informed consent per
// public status and a complete clinical record (odontogram, periodontogram and
// SOEP notes inside and outside the 24h lock window).
func TestSeedDemoData_CreatesTheClinicalDemoDataset(t *testing.T) {
	t.Setenv("SIGO_SEED_DEMO_DATA", "true")
	ctx := context.Background()
	fixture := newDemoFixture()
	now := time.Date(2026, 10, 30, 8, 30, 0, 0, time.UTC)

	summary, err := seedDemoDataIfEnabled(ctx, fixture.deps, now)
	if err != nil {
		t.Fatalf("expected the demo seed to succeed, got %v", err)
	}
	if summary.Patients != 6 {
		t.Fatalf("expected 6 seeded patients, got %d", summary.Patients)
	}

	// Patients: the 6 demo identities with their ages, the legal guardian of
	// the minor and the critical alerts (RF-02).
	patients, total, err := fixture.patients.Search(ctx, "", 100, 0)
	if err != nil {
		t.Fatalf("expected the patient search to succeed, got %v", err)
	}
	if total != 6 || len(patients) != 6 {
		t.Fatalf("expected exactly 6 seeded patients, got total=%d len=%d", total, len(patients))
	}

	byID := make(map[string]*patientdom.Patient, len(patients))
	for _, p := range patients {
		byID[p.ID] = p
	}

	ana := byID[demoPatientAnaID]
	if ana == nil {
		t.Fatal("expected the seeded patient Ana Lucía Fernández to exist")
	}
	if ana.Age != 35 || ana.IsMinor {
		t.Fatalf("expected Ana to be a 35 year old adult, got age=%d minor=%t", ana.Age, ana.IsMinor)
	}
	if len(ana.CriticalAlerts) == 0 || ana.CriticalAlerts[0].Severity != "CRITICAL" {
		t.Fatalf("expected a critical allergy alert for Ana, got %+v", ana.CriticalAlerts)
	}

	carlos := byID[demoPatientCarlosID]
	if carlos == nil {
		t.Fatal("expected the seeded patient Carlos Andrés Mendoza to exist")
	}
	if len(carlos.CriticalAlerts) == 0 || carlos.CriticalAlerts[0].Code != "ANTICOAGULANT_ACTIVE" {
		t.Fatalf("expected an anticoagulant alert for Carlos, got %+v", carlos.CriticalAlerts)
	}

	sofia := byID[demoPatientSofiaID]
	if sofia == nil {
		t.Fatal("expected the seeded patient Sofía Ramírez Ortega to exist")
	}
	if len(sofia.CriticalAlerts) != 0 {
		t.Fatalf("expected no critical alert for Sofía, got %+v", sofia.CriticalAlerts)
	}

	lucas := byID[demoPatientLucasID]
	if lucas == nil {
		t.Fatal("expected the seeded patient Lucas Mateo Díaz to exist")
	}
	if !lucas.IsMinor {
		t.Fatal("expected Lucas to be flagged as a minor")
	}
	if lucas.LegalGuardian == nil ||
		lucas.LegalGuardian.FullName == "" ||
		lucas.LegalGuardian.DocumentNumber == "" ||
		lucas.LegalGuardian.Phone == "" ||
		lucas.LegalGuardian.Relationship == "" {
		t.Fatalf("expected a complete legal guardian for the minor, got %+v", lucas.LegalGuardian)
	}

	valentina := byID[demoPatientValentinaID]
	if valentina == nil {
		t.Fatal("expected the seeded patient Valentina Armas to exist")
	}
	if len(valentina.CriticalAlerts) == 0 {
		t.Fatalf("expected a critical allergy alert for Valentina, got %+v", valentina.CriticalAlerts)
	}
	if !valentina.AnamnesisRecorded {
		t.Fatal("expected the anamnesis of Valentina to be recorded")
	}

	// Agenda: appointments on 3 different chairs, every chair free of
	// overlapping appointments (RN-03) and the whole appointment state
	// machine covered (RF-10).
	appointments, err := fixture.scheduling.ListCalendar(ctx, schedulingapp.CalendarFilter{View: "day", Date: now})
	if err != nil {
		t.Fatalf("expected the calendar query to succeed, got %v", err)
	}
	if len(appointments) < 6 {
		t.Fatalf("expected at least 6 appointments on the seeded day, got %d", len(appointments))
	}

	chairs := make(map[string]bool)
	statuses := make(map[string]bool)
	for _, appt := range appointments {
		chairs[appt.ChairID] = true
		statuses[appt.Status] = true
	}
	if len(chairs) != 3 {
		t.Fatalf("expected the appointments to use 3 chairs, got %d (%v)", len(chairs), chairs)
	}
	for _, want := range []string{
		schedulingdom.StatusProgramado,
		schedulingdom.StatusConfirmado,
		schedulingdom.StatusEnSalaDeEspera,
		schedulingdom.StatusEnAtencion,
		schedulingdom.StatusAtendido,
		schedulingdom.StatusCancelado,
	} {
		if !statuses[want] {
			t.Fatalf("expected the agenda to cover the status %q, got %v", want, statuses)
		}
	}

	byChair := make(map[string][]*schedulingdom.Appointment)
	for _, appt := range appointments {
		byChair[appt.ChairID] = append(byChair[appt.ChairID], appt)
	}
	for chair, list := range byChair {
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				if list[i].OverlapsWith(list[j].StartTime, list[j].EndTime) {
					t.Fatalf("RN-03: chair %q has overlapping appointments %s and %s", chair, list[i].ID, list[j].ID)
				}
			}
		}
	}

	// Treatment plans: one per domain status.
	plans, totalPlans, err := fixture.treatments.ListPlans(ctx, treatmentapp.PlanFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the plan listing to succeed, got %v", err)
	}
	if totalPlans != 4 {
		t.Fatalf("expected 4 seeded treatment plans, got %d", totalPlans)
	}
	planStatuses := make(map[string]bool)
	for _, plan := range plans {
		planStatuses[plan.Status] = true
	}
	for _, want := range []string{"DRAFT", "APPROVED_PARTIAL", "APPROVED_TOTAL", "COMPLETED"} {
		if !planStatuses[want] {
			t.Fatalf("expected a treatment plan in status %q, got %v", want, planStatuses)
		}
	}

	// Informed consents: one per public status (PENDING | SIGNED_BIOMETRIC |
	// SIGNED_OTP).
	consents, totalConsents, err := fixture.consents.ListConsents(ctx, clinicalapp.ConsentFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the consent listing to succeed, got %v", err)
	}
	if totalConsents != 4 {
		t.Fatalf("expected 4 seeded consents, got %d", totalConsents)
	}
	methods := make(map[string]bool)
	for _, consent := range consents {
		if consent.Status == "PENDING_SIGNATURE" {
			methods["PENDING"] = true
			continue
		}
		methods[consent.SignatureMethod] = true
	}
	for _, want := range []string{"PENDING", "BIOMETRIC", "OTP"} {
		if !methods[want] {
			t.Fatalf("expected a consent with the signature method %q, got %v", want, methods)
		}
	}

	// Complete record of Ana Lucía Fernández.
	findings, err := fixture.odontogram.ListFindingsByPatient(ctx, demoPatientAnaID)
	if err != nil {
		t.Fatalf("expected the odontogram listing to succeed, got %v", err)
	}
	if len(findings) < 3 {
		t.Fatalf("expected at least 3 odontogram findings, got %d", len(findings))
	}
	findingTypes := make(map[string]bool)
	immutableFound := false
	for _, f := range findings {
		findingTypes[f.FindingType] = true
		if f.Status == "REALIZADO" && f.Immutable {
			immutableFound = true
		}
	}
	for _, want := range []string{"CARIES", "EXISTING_RESTORATION", "EXTRACTION_DONE"} {
		if !findingTypes[want] {
			t.Fatalf("expected an odontogram finding of type %q, got %v", want, findingTypes)
		}
	}
	if !immutableFound {
		t.Fatal("expected at least one immutable (REALIZADO) odontogram finding")
	}

	charts, err := fixture.periodonto.ListByPatient(ctx, demoPatientAnaID)
	if err != nil {
		t.Fatalf("expected the periodontogram listing to succeed, got %v", err)
	}
	if len(charts) != 1 {
		t.Fatalf("expected exactly 1 seeded periodontogram chart, got %d", len(charts))
	}
	pocketFound := false
	for _, tooth := range charts[0].Teeth {
		for _, depth := range tooth.ProbingDepthMM {
			if depth >= 4 {
				pocketFound = true
			}
		}
	}
	if !pocketFound {
		t.Fatal("expected the seeded periodontogram to include a pocket of 4mm or more")
	}

	// SOEP notes: one outside the 24h lock window (read only, RN-01) and one
	// inside it (editable).
	notes, err := fixture.soep.ListNotesByPatient(ctx, demoPatientAnaID)
	if err != nil {
		t.Fatalf("expected the SOEP listing to succeed, got %v", err)
	}
	if len(notes) < 2 {
		t.Fatalf("expected at least 2 seeded SOEP notes, got %d", len(notes))
	}
	lockedFound := false
	editableFound := false
	for _, note := range notes {
		if note.CreatedAt.Before(now.Add(-24 * time.Hour)) {
			lockedFound = true
			if !note.IsLocked {
				t.Fatalf("expected the note created at %s to be locked", note.CreatedAt)
			}
			continue
		}
		editableFound = true
		if note.IsLocked {
			t.Fatalf("expected the note created at %s to still be editable", note.CreatedAt)
		}
	}
	if !lockedFound {
		t.Fatal("expected a SOEP note created more than 24h ago")
	}
	if !editableFound {
		t.Fatal("expected a SOEP note created less than 24h ago")
	}
}

// TestSeedDemoData_IsIdempotent covers S4: replaying the seed never
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

	_, patients, err := fixture.patients.Search(ctx, "", 100, 0)
	if err != nil {
		t.Fatalf("expected the patient search to succeed, got %v", err)
	}
	if patients != 6 {
		t.Fatalf("expected still 6 patients after re-seeding, got %d", patients)
	}

	_, plans, err := fixture.treatments.ListPlans(ctx, treatmentapp.PlanFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the plan listing to succeed, got %v", err)
	}
	if plans != 4 {
		t.Fatalf("expected still 4 treatment plans after re-seeding, got %d", plans)
	}

	_, consents, err := fixture.consents.ListConsents(ctx, clinicalapp.ConsentFilter{Limit: 100})
	if err != nil {
		t.Fatalf("expected the consent listing to succeed, got %v", err)
	}
	if consents != 4 {
		t.Fatalf("expected still 4 consents after re-seeding, got %d", consents)
	}

	notes, err := fixture.soep.ListNotesByPatient(ctx, demoPatientAnaID)
	if err != nil {
		t.Fatalf("expected the SOEP listing to succeed, got %v", err)
	}
	if len(notes) != 3 {
		t.Fatalf("expected still 3 SOEP notes after re-seeding, got %d", len(notes))
	}

	charts, err := fixture.periodonto.ListByPatient(ctx, demoPatientAnaID)
	if err != nil {
		t.Fatalf("expected the periodontogram listing to succeed, got %v", err)
	}
	if len(charts) != 1 {
		t.Fatalf("expected still 1 periodontogram chart after re-seeding, got %d", len(charts))
	}

	findings, err := fixture.odontogram.ListFindingsByPatient(ctx, demoPatientAnaID)
	if err != nil {
		t.Fatalf("expected the odontogram listing to succeed, got %v", err)
	}
	if len(findings) != 6 {
		t.Fatalf("expected still 6 odontogram findings after re-seeding, got %d", len(findings))
	}
}
