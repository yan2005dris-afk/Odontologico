package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	clinicalapp "github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	clinicaldom "github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
	patientapp "github.com/sigo-dental/clinical-agenda/internal/patients/application"
	patientdom "github.com/sigo-dental/clinical-agenda/internal/patients/domain"
	schedulingapp "github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	schedulingdom "github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
	treatmentapp "github.com/sigo-dental/clinical-agenda/internal/treatments/application"
	treatmentdom "github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

// Demo dataset of the clinical-agenda service (work item wi-18, spec S4).
//
// The repositories of the service are in-memory, so every boot starts empty
// and the portal has nothing to show. The dataset below is written by the
// domain APIs of every bounded context, only when SIGO_SEED_DEMO_DATA is
// enabled, and it is idempotent: a second run never duplicates a record.
//
// The data is fictitious but coherent: Ecuadorian names, documents and phone
// numbers, ages that match the requested profiles and amounts expressed in
// cents. No secret of any kind (TOTP seed, API key) is part of the dataset.

const (
	demoClinicID    = "CLINICA-CENTRAL"
	demoCurrency    = "USD"
	demoDentistID   = "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d"
	demoDentistRole = "dentist"
	demoSourceIP    = "127.0.0.1"

	// demoSOEPLockWindow is the RN-01 lock window of the SOEP notes.
	demoSOEPLockWindow = 24 * time.Hour
)

// Statuses of the treatment plan aggregate (RF-07) and of its items.
const (
	demoPlanStatusDraft           = "DRAFT"
	demoPlanStatusApprovedPartial = "APPROVED_PARTIAL"
	demoPlanStatusApprovedTotal   = "APPROVED_TOTAL"
	demoPlanStatusCompleted       = "COMPLETED"
	demoItemStatusRealizado       = "REALIZADO"
)

// Fixed identifiers of the demo patients. The billing-stock service reuses
// them so the invoices of the demo dataset point to real seeded patients.
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
	audit      auditapp.Recorder
	patients   patientapp.PatientRepository
	scheduling schedulingapp.SchedulingRepository
	treatments treatmentapp.TreatmentRepository
	odontogram clinicalapp.OdontogramRepository
	periodonto clinicalapp.PeriodontogramRepository
	soep       clinicalapp.SOEPRepository
	consents   clinicalapp.ConsentRepository
	blobs      clinicalapp.BlobStoragePort
}

// demoSummary counts the records written by a seed run, for the boot log.
type demoSummary struct {
	Patients            int
	Anamneses           int
	Appointments        int
	TreatmentPlans      int
	OdontogramFindings  int
	PeriodontogramCards int
	SOEPNotes           int
	Consents            int
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
	seeder.seedPatients(ctx)
	seeder.seedAnamneses(ctx)
	seeder.seedAppointments(ctx)
	seeder.seedTreatmentPlans(ctx)
	seeder.seedOdontogram(ctx)
	seeder.seedPeriodontogram(ctx)
	seeder.seedSOEPNotes(ctx)
	seeder.seedConsents(ctx)

	return seeder.summary, errors.Join(seeder.errs...)
}

func (s *demoSeeder) fail(step string, err error) {
	s.errs = append(s.errs, fmt.Errorf("demo seed %s: %w", step, err))
}

func (s *demoSeeder) audit(ctx context.Context, action auditdom.AuditAction, resourceType, resourceID string) {
	if s.deps.audit == nil {
		return
	}
	_, _ = s.deps.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      demoDentistID,
		ActorRole:    demoDentistRole,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		SourceIP:     demoSourceIP,
		Metadata:     map[string]string{"dataset": "demo"},
	})
}

// dayStart returns the UTC midnight of the day offset days away from the
// seed moment.
func (s *demoSeeder) dayStart(offsetDays int) time.Time {
	base := time.Date(s.now.Year(), s.now.Month(), s.now.Day(), 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, offsetDays)
}

// ---------------------------------------------------------------------------
// Patients (RF-01) and anamnesis (RF-02)
// ---------------------------------------------------------------------------

// demoPatient describes one patient of the demo dataset.
type demoPatient struct {
	ID                string
	DocumentNumber    string
	FirstName         string
	LastName          string
	BirthDate         string
	Sex               string
	Phone             string
	Email             string
	EmergencyName     string
	EmergencyPhone    string
	EmergencyRelation string
	GuardianName      string
	GuardianDocument  string
	GuardianPhone     string
	GuardianRelation  string
}

// demoPatients holds the 6 demo patients. Lucas Mateo Díaz is a minor and
// therefore carries a complete legal guardian.
var demoPatients = []demoPatient{
	{
		ID:                demoPatientAnaID,
		DocumentNumber:    "0912345678",
		FirstName:         "Ana Lucía",
		LastName:          "Fernández",
		BirthDate:         "1991-03-22",
		Sex:               "F",
		Phone:             "+593987654321",
		Email:             "ana.lucia.fernandez@example.com",
		EmergencyName:     "Carlos Fernández",
		EmergencyPhone:    "+593987654301",
		EmergencyRelation: "Cónyuge",
	},
	{
		ID:                demoPatientCarlosID,
		DocumentNumber:    "1312345679",
		FirstName:         "Carlos Andrés",
		LastName:          "Mendoza",
		BirthDate:         "1984-06-09",
		Sex:               "M",
		Phone:             "+593987654322",
		Email:             "carlos.andres.mendoza@example.com",
		EmergencyName:     "Lucía Mendoza",
		EmergencyPhone:    "+593987654302",
		EmergencyRelation: "Hermana",
	},
	{
		ID:                demoPatientSofiaID,
		DocumentNumber:    "1712345670",
		FirstName:         "Sofía",
		LastName:          "Ramírez Ortega",
		BirthDate:         "1998-01-25",
		Sex:               "F",
		Phone:             "+593987654323",
		Email:             "sofia.ramirez.ortega@example.com",
		EmergencyName:     "Andrés Ramírez",
		EmergencyPhone:    "+593987654303",
		EmergencyRelation: "Padre",
	},
	{
		ID:                demoPatientLucasID,
		DocumentNumber:    "2112345671",
		FirstName:         "Lucas Mateo",
		LastName:          "Díaz",
		BirthDate:         "2015-03-15",
		Sex:               "M",
		Phone:             "+593987654324",
		Email:             "lucas.mateo.diaz@example.com",
		EmergencyName:     "Martha Isabel Díaz",
		EmergencyPhone:    "+593987654304",
		EmergencyRelation: "Madre",
		GuardianName:      "Martha Isabel Díaz",
		GuardianDocument:  "1209876543",
		GuardianPhone:     "+593987654304",
		GuardianRelation:  "Madre",
	},
	{
		ID:                demoPatientMateoID,
		DocumentNumber:    "0712345672",
		FirstName:         "Mateo Emiliano",
		LastName:          "Cruz",
		BirthDate:         "1975-09-03",
		Sex:               "M",
		Phone:             "+593987654325",
		Email:             "mateo.emiliano.cruz@example.com",
		EmergencyName:     "Rosa Cruz",
		EmergencyPhone:    "+593987654305",
		EmergencyRelation: "Esposa",
	},
	{
		ID:                demoPatientValentinaID,
		DocumentNumber:    "1012345673",
		FirstName:         "Valentina",
		LastName:          "Armas",
		BirthDate:         "1993-07-19",
		Sex:               "F",
		Phone:             "+593987654326",
		Email:             "valentina.armas@example.com",
		EmergencyName:     "Jorge Armas",
		EmergencyPhone:    "+593987654306",
		EmergencyRelation: "Padre",
	},
}

func (s *demoSeeder) seedPatients(ctx context.Context) {
	for i, p := range demoPatients {
		if existing, err := s.deps.patients.GetByID(ctx, p.ID); err == nil && existing != nil {
			continue
		}
		params := patientdom.CreatePatientParams{
			ID:               p.ID,
			HCNumber:         fmt.Sprintf("HC-%d-%06d", s.now.Year(), i+1),
			DocumentType:     "DNI",
			DocumentNumber:   p.DocumentNumber,
			FirstName:        p.FirstName,
			LastName:         p.LastName,
			BirthDate:        p.BirthDate,
			Sex:              p.Sex,
			Phone:            p.Phone,
			Email:            p.Email,
			EmergencyContact: patientdom.EmergencyContact{FullName: p.EmergencyName, Phone: p.EmergencyPhone, Relationship: p.EmergencyRelation},
		}
		if p.GuardianName != "" {
			params.LegalGuardian = &patientdom.LegalGuardian{
				FullName:       p.GuardianName,
				DocumentNumber: p.GuardianDocument,
				Phone:          p.GuardianPhone,
				Relationship:   p.GuardianRelation,
			}
		}
		patient, err := patientdom.NewPatient(params, s.now)
		if err != nil {
			s.fail("patients", err)
			continue
		}
		if err := s.deps.patients.Create(ctx, patient); err != nil {
			s.fail("patients", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "PATIENT", patient.ID)
		s.summary.Patients++
	}
}

// demoAnamnesis describes one anamnesis of the demo dataset. The allergies
// and the coagulopathies are the source of the critical alerts of RF-02.
type demoAnamnesis struct {
	ID             string
	PatientID      string
	Diseases       []patientdom.SystemicDisease
	Allergies      []patientdom.DrugAllergy
	Surgeries      []patientdom.SurgicalHistory
	Coagulopathies []patientdom.Coagulopathy
	Notes          string
}

var demoAnamneses = []demoAnamnesis{
	{
		ID:        "demo-anamnesis-001",
		PatientID: demoPatientAnaID,
		Diseases: []patientdom.SystemicDisease{
			{Name: "Hipertensión arterial", Controlled: true, Notes: "Controlada con enalapril 10 mg/día."},
		},
		Allergies: []patientdom.DrugAllergy{
			{Substance: "Penicilina", Reaction: "Anafilaxia y urticaria", Severity: "CRITICAL"},
		},
		Notes: "Alergia a penicilina documentada desde 2018; debe permanecer visible en la ficha del paciente.",
	},
	{
		ID:        "demo-anamnesis-002",
		PatientID: demoPatientCarlosID,
		Diseases: []patientdom.SystemicDisease{
			{Name: "Hipertensión arterial", Controlled: true, Notes: "Controlada con losartán 50 mg/día."},
		},
		Coagulopathies: []patientdom.Coagulopathy{
			{
				Condition:        "Trombosis venosa profunda",
				Medication:       "Warfarina 5 mg",
				INRLastValue:     2.4,
				IsAnticoagulated: true,
			},
		},
		Notes: "Paciente anticoagulado; cualquier procedimiento invasivo requiere valoración médica previa.",
	},
	{
		ID:        "demo-anamnesis-003",
		PatientID: demoPatientSofiaID,
		Diseases: []patientdom.SystemicDisease{
			{Name: "Hipertensión arterial", Controlled: true, Notes: "Controlada conamlodipino 5 mg/día."},
		},
		Notes: "Sin antecedentes relevantes ni alergias registradas.",
	},
	{
		ID:        "demo-anamnesis-004",
		PatientID: demoPatientValentinaID,
		Allergies: []patientdom.DrugAllergy{
			{Substance: "Látex", Reaction: "Reacción dérmica con guantes de látex", Severity: "CRITICAL"},
		},
		Notes: "Alergia al látex: el consultorio debe usar guantes de nitrilo en todas las citas.",
	},
}

func (s *demoSeeder) seedAnamneses(ctx context.Context) {
	for _, a := range demoAnamneses {
		patient, err := s.deps.patients.GetByID(ctx, a.PatientID)
		if err != nil {
			s.fail("anamnesis", err)
			continue
		}
		if patient.AnamnesisRecorded {
			continue
		}
		anamnesis, alerts, err := patientdom.NewAnamnesis(
			a.ID,
			a.PatientID,
			demoDentistID,
			a.Diseases,
			a.Allergies,
			a.Surgeries,
			a.Coagulopathies,
			false,
			a.Notes,
			s.now,
		)
		if err != nil {
			s.fail("anamnesis", err)
			continue
		}
		if err := s.deps.patients.SaveAnamnesis(ctx, anamnesis, alerts); err != nil {
			s.fail("anamnesis", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "PATIENT_ANAMNESIS", a.PatientID)
		s.summary.Anamneses++
	}
}

// ---------------------------------------------------------------------------
// Agenda (RF-09, RF-10, RN-03)
// ---------------------------------------------------------------------------

// demoAppointment describes one appointment of the demo dataset. The
// appointments are staggered so no chair (and no dentist) is double booked.
type demoAppointment struct {
	ID             string
	PatientID      string
	ChairID        string
	DayOffset      int
	StartHour      int
	DurationMinute int
	Status         string
	Reason         string
	Notes          string
}

var demoAppointments = []demoAppointment{
	{ID: "demo-appt-001", PatientID: demoPatientAnaID, ChairID: "SILLON-01", DayOffset: 0, StartHour: 8, DurationMinute: 60, Status: schedulingdom.StatusAtendido, Reason: "Evaluación inicial y odontograma", Notes: "Consulta completada; se agenda control en 30 días."},
	{ID: "demo-appt-002", PatientID: demoPatientSofiaID, ChairID: "SILLON-02", DayOffset: 0, StartHour: 9, DurationMinute: 60, Status: schedulingdom.StatusEnAtencion, Reason: "Control preventivo y profilaxis", Notes: "En atención con la odontóloga."},
	{ID: "demo-appt-003", PatientID: demoPatientLucasID, ChairID: "SILLON-03", DayOffset: 0, StartHour: 10, DurationMinute: 60, Status: schedulingdom.StatusEnSalaDeEspera, Reason: "Control de dentición temporal", Notes: "Acompañado por su madre, tutora legal."},
	{ID: "demo-appt-004", PatientID: demoPatientCarlosID, ChairID: "SILLON-01", DayOffset: 0, StartHour: 11, DurationMinute: 60, Status: schedulingdom.StatusConfirmado, Reason: "Valoración de sangrado gingival", Notes: "Confirmado por teléfono."},
	{ID: "demo-appt-005", PatientID: demoPatientMateoID, ChairID: "SILLON-02", DayOffset: 0, StartHour: 12, DurationMinute: 60, Status: schedulingdom.StatusProgramado, Reason: "Endodoncia de molar inferior", Notes: "Paciente refiere tratamiento previo en otra clínica."},
	{ID: "demo-appt-006", PatientID: demoPatientValentinaID, ChairID: "SILLON-03", DayOffset: 0, StartHour: 14, DurationMinute: 60, Status: schedulingdom.StatusCancelado, Reason: "Restauración estética del sector anterior", Notes: "Cancelada por la paciente: reprogramar la próxima semana."},
	{ID: "demo-appt-007", PatientID: demoPatientSofiaID, ChairID: "SILLON-01", DayOffset: 1, StartHour: 9, DurationMinute: 90, Status: schedulingdom.StatusProgramado, Reason: "Continuación de tratamiento preventivo", Notes: "Sesión de destartraje y sellantes."},
	{ID: "demo-appt-008", PatientID: demoPatientAnaID, ChairID: "SILLON-03", DayOffset: 2, StartHour: 10, DurationMinute: 60, Status: schedulingdom.StatusConfirmado, Reason: "Control post-operatorio de restorations", Notes: "Control de la resina colocada en la pieza 16."},
	{ID: "demo-appt-009", PatientID: demoPatientCarlosID, ChairID: "SILLON-02", DayOffset: 3, StartHour: 15, DurationMinute: 60, Status: schedulingdom.StatusProgramado, Reason: "Control de coagulación pre-quirúrgico", Notes: "Se solicita INR actualizado antes de la cita."},
}

// appointmentTransitionPath returns the legal transition sequence from
// PROGRAMADO to the target status of the appointment state machine (RF-10).
func appointmentTransitionPath(target string) []string {
	switch target {
	case schedulingdom.StatusConfirmado:
		return []string{schedulingdom.StatusConfirmado}
	case schedulingdom.StatusEnSalaDeEspera:
		return []string{schedulingdom.StatusConfirmado, schedulingdom.StatusEnSalaDeEspera}
	case schedulingdom.StatusEnAtencion:
		return []string{schedulingdom.StatusConfirmado, schedulingdom.StatusEnSalaDeEspera, schedulingdom.StatusEnAtencion}
	case schedulingdom.StatusAtendido:
		return []string{schedulingdom.StatusConfirmado, schedulingdom.StatusEnSalaDeEspera, schedulingdom.StatusEnAtencion, schedulingdom.StatusAtendido}
	case schedulingdom.StatusCancelado:
		return []string{schedulingdom.StatusConfirmado, schedulingdom.StatusCancelado}
	default:
		return nil
	}
}

func (s *demoSeeder) seedAppointments(ctx context.Context) {
	for _, a := range demoAppointments {
		if existing, err := s.deps.scheduling.GetByID(ctx, a.ID); err == nil && existing != nil {
			continue
		}
		start := s.dayStart(a.DayOffset).Add(time.Duration(a.StartHour) * time.Hour)
		end := start.Add(time.Duration(a.DurationMinute) * time.Minute)
		appt, err := schedulingdom.NewAppointment(
			a.ID,
			a.PatientID,
			demoDentistID,
			a.ChairID,
			demoClinicID,
			"",
			a.Reason,
			start,
			end,
			s.now,
			[]string{"WHATSAPP", "EMAIL"},
		)
		if err != nil {
			s.fail("appointments", err)
			continue
		}
		jobs := schedulingdom.BuildAppointmentReminders(appt.ID, appt.StartTime, s.now, []string{"WHATSAPP", "EMAIL"})
		if err := s.deps.scheduling.CreateAppointment(ctx, appt, jobs); err != nil {
			s.fail("appointments", err)
			continue
		}

		path := appointmentTransitionPath(a.Status)
		transitioned := true
		for i, target := range path {
			notes := ""
			if i == len(path)-1 {
				notes = a.Notes
			}
			if err := appt.TransitionTo(target, notes); err != nil {
				s.fail("appointments", err)
				transitioned = false
				break
			}
		}
		if !transitioned {
			continue
		}
		if err := s.deps.scheduling.UpdateAppointment(ctx, appt); err != nil {
			s.fail("appointments", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "APPOINTMENT", appt.ID)
		s.summary.Appointments++
	}
}

// ---------------------------------------------------------------------------
// Treatment plans (RF-07, RF-08)
// ---------------------------------------------------------------------------

// demoPlanItem describes one item of a demo treatment plan. The amounts are
// expressed in cents.
type demoPlanItem struct {
	ID              string
	ProcedureCode   string
	ProcedureName   string
	ToothFDI        string
	Surface         string
	FeeCents        int64
	LabCents        int64
	DurationMinutes int
}

// demoPlanPhase describes one phase of a demo treatment plan.
type demoPlanPhase struct {
	Number int
	Name   string
	Items  []demoPlanItem
}

// demoPlan describes one treatment plan of the demo dataset.
type demoPlan struct {
	ID          string
	PatientID   string
	Title       string
	Phases      []demoPlanPhase
	Target      string
	Acceptance  string
	ApproveItem []string
}

var demoPlans = []demoPlan{
	{
		ID:        "demo-plan-001",
		PatientID: demoPatientSofiaID,
		Title:     "Plan preventivo y sellantes",
		Phases: []demoPlanPhase{
			{Number: 1, Name: "Fase preventiva", Items: []demoPlanItem{
				{ID: "demo-plan-001-item-01", ProcedureCode: "D1110", ProcedureName: "Profilaxis dental", FeeCents: 4500, DurationMinutes: 30},
				{ID: "demo-plan-001-item-02", ProcedureCode: "D1210", ProcedureName: "Destartraje por sextante", FeeCents: 3000, DurationMinutes: 45},
			}},
		},
		Target: demoPlanStatusDraft,
	},
	{
		ID:        "demo-plan-002",
		PatientID: demoPatientAnaID,
		Title:     "Plan restaurador y preventivo",
		Phases: []demoPlanPhase{
			{Number: 1, Name: "Fase restauradora", Items: []demoPlanItem{
				{ID: "demo-plan-002-item-01", ProcedureCode: "D2391", ProcedureName: "Resina compuesta posterior", ToothFDI: "16", Surface: "OCLUSAL_INCISAL", FeeCents: 12000, DurationMinutes: 60},
				{ID: "demo-plan-002-item-02", ProcedureCode: "D2392", ProcedureName: "Resina compuesta anterior", ToothFDI: "11", Surface: "VESTIBULAR", FeeCents: 9000, DurationMinutes: 45},
			}},
			{Number: 2, Name: "Fase preventiva", Items: []demoPlanItem{
				{ID: "demo-plan-002-item-03", ProcedureCode: "D1110", ProcedureName: "Profilaxis dental", FeeCents: 4500, DurationMinutes: 30},
			}},
		},
		Target:      demoPlanStatusApprovedPartial,
		Acceptance:  "ACTA-2026-000214",
		ApproveItem: []string{"demo-plan-002-item-01", "demo-plan-002-item-02"},
	},
	{
		ID:        "demo-plan-003",
		PatientID: demoPatientCarlosID,
		Title:     "Plan de endodoncia y prótesis",
		Phases: []demoPlanPhase{
			{Number: 1, Name: "Fase endodóntica", Items: []demoPlanItem{
				{ID: "demo-plan-003-item-01", ProcedureCode: "D3330", ProcedureName: "Endodoncia de molar", ToothFDI: "36", FeeCents: 45000, DurationMinutes: 90},
			}},
			{Number: 2, Name: "Fase protésica", Items: []demoPlanItem{
				{ID: "demo-plan-003-item-02", ProcedureCode: "D6010", ProcedureName: "Corona de porcelana", ToothFDI: "36", FeeCents: 38000, LabCents: 15000, DurationMinutes: 60},
			}},
		},
		Target:     demoPlanStatusApprovedTotal,
		Acceptance: "ACTA-2026-000215",
	},
	{
		ID:        "demo-plan-004",
		PatientID: demoPatientMateoID,
		Title:     "Plan de extracción y endodoncia",
		Phases: []demoPlanPhase{
			{Number: 1, Name: "Fase quirúrgica", Items: []demoPlanItem{
				{ID: "demo-plan-004-item-01", ProcedureCode: "D7140", ProcedureName: "Exodoncia", ToothFDI: "46", FeeCents: 9000, DurationMinutes: 45},
			}},
			{Number: 2, Name: "Fase endodóntica", Items: []demoPlanItem{
				{ID: "demo-plan-004-item-02", ProcedureCode: "D3310", ProcedureName: "Endodoncia de anterior", ToothFDI: "21", FeeCents: 25000, DurationMinutes: 60},
				{ID: "demo-plan-004-item-03", ProcedureCode: "D1110", ProcedureName: "Profilaxis dental", FeeCents: 4500, DurationMinutes: 30},
			}},
		},
		Target:     demoPlanStatusCompleted,
		Acceptance: "ACTA-2026-000216",
	},
}

func (s *demoSeeder) seedTreatmentPlans(ctx context.Context) {
	for _, p := range demoPlans {
		existing, err := s.deps.treatments.GetPlanByID(ctx, p.ID)
		if err == nil && existing != nil {
			s.advancePlan(ctx, existing, p)
			continue
		}
		phases := make([]treatmentdom.TreatmentPhase, 0, len(p.Phases))
		for _, ph := range p.Phases {
			items := make([]treatmentdom.TreatmentItem, 0, len(ph.Items))
			for _, it := range ph.Items {
				items = append(items, treatmentdom.TreatmentItem{
					ID:                       it.ID,
					ProcedureCode:            it.ProcedureCode,
					ProcedureName:            it.ProcedureName,
					ToothFDI:                 it.ToothFDI,
					Surface:                  it.Surface,
					ProfessionalFeeCents:     it.FeeCents,
					LabCostCents:             it.LabCents,
					DiscountCents:            0,
					EstimatedDurationMinutes: it.DurationMinutes,
				})
			}
			phases = append(phases, treatmentdom.TreatmentPhase{PhaseNumber: ph.Number, Name: ph.Name, Items: items})
		}
		plan, err := treatmentdom.NewTreatmentPlan(p.ID, p.PatientID, p.Title, demoCurrency, demoDentistID, demoDentistRole, phases, s.now)
		if err != nil {
			s.fail("treatment plans", err)
			continue
		}
		if err := s.deps.treatments.SavePlan(ctx, plan); err != nil {
			s.fail("treatment plans", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "TREATMENT_PLAN", plan.ID)
		s.summary.TreatmentPlans++
		s.advancePlan(ctx, plan, p)
	}
}

// advancePlan drives a plan to the status of the demo dataset. It is safe to
// call it again: a plan already in the target status is left untouched.
func (s *demoSeeder) advancePlan(ctx context.Context, plan *treatmentdom.TreatmentPlan, spec demoPlan) {
	if spec.Target == demoPlanStatusDraft {
		return
	}
	if plan.Status == demoPlanStatusDraft {
		mode := "TOTAL"
		if spec.Target == demoPlanStatusApprovedPartial {
			mode = "PARTIAL"
		}
		tasks, err := plan.Approve(mode, spec.ApproveItem, spec.Acceptance, demoTaskIDGen(plan.ID), s.now)
		if err != nil {
			s.fail("treatment plans", err)
			return
		}
		if err := s.deps.treatments.SavePlanApprovalTx(ctx, plan, tasks); err != nil {
			s.fail("treatment plans", err)
			return
		}
		s.audit(ctx, auditdom.ActionUpdate, "TREATMENT_PLAN", plan.ID)
	}
	if spec.Target != demoPlanStatusCompleted || plan.Status != demoPlanStatusApprovedTotal {
		return
	}
	for _, ph := range plan.Phases {
		for _, item := range ph.Items {
			if item.Status == demoItemStatusRealizado {
				continue
			}
			if _, err := plan.CompleteItem(item.ID, demoDentistID, s.now); err != nil {
				s.fail("treatment plans", err)
				return
			}
		}
	}
	if err := s.deps.treatments.SavePlan(ctx, plan); err != nil {
		s.fail("treatment plans", err)
		return
	}
	s.audit(ctx, auditdom.ActionUpdate, "TREATMENT_PLAN", plan.ID)
}

// demoTaskIDGen returns a deterministic identifier generator for the
// schedulable tasks of a plan, so a re-seed never orphans them.
func demoTaskIDGen(planID string) func() string {
	counter := 0
	return func() string {
		counter++
		return fmt.Sprintf("%s-task-%02d", planID, counter)
	}
}

// ---------------------------------------------------------------------------
// Odontogram, periodontogram and SOEP notes (RF-03, RF-04, RN-01)
// ---------------------------------------------------------------------------

// demoFinding describes one odontogram finding of the demo dataset.
type demoFinding struct {
	ID          string
	PatientID   string
	ToothFDI    string
	Surface     string
	FindingType string
	Status      string
	Notes       string
}

var demoFindings = []demoFinding{
	{ID: "demo-odo-001", PatientID: demoPatientAnaID, ToothFDI: "16", Surface: "OCLUSAL_INCISAL", FindingType: "CARIES", Status: "DIAGNOSED", Notes: "Lesión cariosa oclusal profunda, sin compromiso pulpar."},
	{ID: "demo-odo-002", PatientID: demoPatientAnaID, ToothFDI: "26", Surface: "MESIAL", FindingType: "CARIES", Status: "PLANNED", Notes: "Lesión cariosa proximal; se incluye en el plan restaurador."},
	{ID: "demo-odo-003", PatientID: demoPatientAnaID, ToothFDI: "36", Surface: "OCLUSAL_INCISAL", FindingType: "EXISTING_RESTORATION", Status: "REALIZADO", Notes: "Restauración de amalgama clase II en buen estado."},
	{ID: "demo-odo-004", PatientID: demoPatientAnaID, ToothFDI: "46", Surface: "WHOLE_TOOTH", FindingType: "EXTRACTION_DONE", Status: "REALIZADO", Notes: "Pieza 46 extraída por hopeless dentistry."},
	{ID: "demo-odo-005", PatientID: demoPatientAnaID, ToothFDI: "11", Surface: "VESTIBULAR", FindingType: "SEALANT", Status: "DIAGNOSED", Notes: "Fosas y fisuras profundas, candidatas a sellante."},
	{ID: "demo-odo-006", PatientID: demoPatientAnaID, ToothFDI: "21", Surface: "LINGUAL_PALATINO", FindingType: "ENDODONTICS_REQUIRED", Status: "PLANNED", Notes: "Necrosis pulpal con lesión periapical; requiere tratamiento de conductos."},
}

func (s *demoSeeder) seedOdontogram(ctx context.Context) {
	for _, f := range demoFindings {
		if existing, err := s.deps.odontogram.GetFindingByID(ctx, f.PatientID, f.ID); err == nil && existing != nil {
			continue
		}
		finding, err := clinicaldom.NewOdontogramFinding(
			f.ID,
			f.PatientID,
			f.ToothFDI,
			"FDI",
			f.Surface,
			f.FindingType,
			f.Status,
			f.Notes,
			demoDentistID,
			s.now,
		)
		if err != nil {
			s.fail("odontogram", err)
			continue
		}
		if err := s.deps.odontogram.SaveFinding(ctx, finding); err != nil {
			s.fail("odontogram", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "ODONTOGRAM", f.ID)
		s.summary.OdontogramFindings++
	}
}

// demoPerioTooth describes one tooth of the seeded periodontogram chart.
type demoPerioTooth struct {
	FDI        string
	Probing   [6]int
	Margin    [6]int
	Bleeding  [6]bool
	Furcation int
	Mobility  int
}

var demoPerioTeeth = []demoPerioTooth{
	{FDI: "11", Probing: [6]int{3, 2, 3, 3, 2, 3}, Margin: [6]int{-1, -1, 0, 0, -1, -1}, Bleeding: [6]bool{false, false, true, false, false, false}},
	{FDI: "16", Probing: [6]int{4, 3, 3, 4, 3, 5}, Margin: [6]int{-2, -2, -1, -1, -2, -2}, Bleeding: [6]bool{true, false, true, false, true, false}, Furcation: 1},
	{FDI: "21", Probing: [6]int{3, 3, 2, 3, 4, 3}, Margin: [6]int{-1, 0, -1, -1, 0, -1}, Bleeding: [6]bool{false, true, false, true, false, false}},
	{FDI: "26", Probing: [6]int{6, 5, 4, 4, 5, 6}, Margin: [6]int{-3, -2, -2, -1, -3, -2}, Bleeding: [6]bool{true, true, true, false, true, true}, Furcation: 1, Mobility: 1},
	{FDI: "31", Probing: [6]int{3, 2, 3, 2, 3, 3}, Margin: [6]int{0, -1, 0, -1, 0, -1}, Bleeding: [6]bool{false, false, false, true, false, false}},
	{FDI: "36", Probing: [6]int{5, 4, 4, 3, 4, 5}, Margin: [6]int{-2, -2, -1, -2, -2, -2}, Bleeding: [6]bool{true, true, false, true, true, false}, Furcation: 1},
	{FDI: "41", Probing: [6]int{4, 3, 3, 4, 3, 3}, Margin: [6]int{-1, -1, -1, -2, -1, -1}, Bleeding: [6]bool{false, false, true, false, false, false}},
	{FDI: "46", Probing: [6]int{5, 5, 4, 4, 5, 5}, Margin: [6]int{-2, -3, -2, -2, -2, -3}, Bleeding: [6]bool{true, true, true, true, false, true}, Furcation: 2, Mobility: 1},
}

const demoPerioChartID = "demo-perio-001"

func (s *demoSeeder) seedPeriodontogram(ctx context.Context) {
	charts, err := s.deps.periodonto.ListByPatient(ctx, demoPatientAnaID)
	if err != nil {
		s.fail("periodontogram", err)
		return
	}
	if len(charts) > 0 {
		return
	}
	teeth := make([]clinicaldom.PeriodontogramTooth, 0, len(demoPerioTeeth))
	for _, t := range demoPerioTeeth {
		teeth = append(teeth, clinicaldom.PeriodontogramTooth{
			ToothFDI:          t.FDI,
			ProbingDepthMM:   t.Probing,
			GingivalMarginMM: t.Margin,
			BleedingOnProbing: t.Bleeding,
			FurcationGrade:   t.Furcation,
			MobilityGrade:    t.Mobility,
		})
	}
	chart, err := clinicaldom.NewPeriodontogramChart(demoPerioChartID, demoPatientAnaID, demoDentistID, teeth, s.now)
	if err != nil {
		s.fail("periodontogram", err)
		return
	}
	if err := s.deps.periodonto.SaveChart(ctx, chart); err != nil {
		s.fail("periodontogram", err)
		return
	}
	s.audit(ctx, auditdom.ActionCreate, "PERIODONTOGRAM", chart.ID)
	s.summary.PeriodontogramCards++
}

// demoSOEPNote describes one SOEP note of the demo dataset. AgeHours places
// the note inside or outside the 24h lock window of RN-01.
type demoSOEPNote struct {
	ID            string
	PatientID     string
	AppointmentID string
	ProcedureCode string
	AgeHours      int
	Subjective    string
	Objective     string
	Assessment    string
	Plan          string
}

var demoSOEPNotes = []demoSOEPNote{
	{
		ID:            "demo-soep-001",
		PatientID:     demoPatientAnaID,
		AppointmentID: "",
		ProcedureCode: "D1110",
		AgeHours:      30,
		Subjective:    "Paciente refiere cepillado dos veces al día y sangrado gingival al escovar desde hace un mes.",
		Objective:     "Se observan cambios inflamatorios en la gingiva marginal y sangrado al sondaje en el sextante anterior.",
		Assessment:    "Gingivitis crónica localizada con acúmulo de placa supragingival.",
		Plan:          "Profilaxis, refuerzo de técnica de cepillado y control en 30 días.",
	},
	{
		ID:            "demo-soep-002",
		PatientID:     demoPatientAnaID,
		AppointmentID: "demo-appt-001",
		ProcedureCode: "D2391",
		AgeHours:      3,
		Subjective:    "Paciente refiere sensibilidad al frío en la pieza 16 desde hace dos semanas, sin dolor espontáneo.",
		Objective:     "Lesión cariosa oclusal en 16 con respuesta vital y sin dolor a la percusión.",
		Assessment:    "Caries oclusal en la pieza 16, sin compromiso pulpar.",
		Plan:          "Restauración con resina compuesta en 16 bajo anestesia y control de vitalidad.",
	},
	{
		ID:            "demo-soep-003",
		PatientID:     demoPatientAnaID,
		AppointmentID: "",
		ProcedureCode: "D1210",
		AgeHours:      1,
		Subjective:    "Paciente asintomático tras la aplicación de sellante en la pieza 11, sin sensibilidad al frío.",
		Objective:     "Sellante de fosas y fisuras en 11 íntegro y sin pérdidas marginales.",
		Assessment:    "Sellante preventivo en la pieza 11 sin complicaciones.",
		Plan:          "Controles semestrales y refuerzo de cepillado interproximal.",
	},
}

func (s *demoSeeder) seedSOEPNotes(ctx context.Context) {
	for _, n := range demoSOEPNotes {
		if existing, err := s.deps.soep.GetNoteByID(ctx, n.PatientID, n.ID); err == nil && existing != nil {
			continue
		}
		createdAt := s.now.Add(-time.Duration(n.AgeHours) * time.Hour)
		note, err := clinicaldom.NewSOEPNote(
			n.ID,
			n.PatientID,
			n.AppointmentID,
			n.ProcedureCode,
			demoDentistID,
			n.Subjective,
			n.Objective,
			n.Assessment,
			n.Plan,
			demoSOEPLockWindow,
			createdAt,
		)
		if err != nil {
			s.fail("soep notes", err)
			continue
		}
		note.RefreshLockState(s.now)
		if err := s.deps.soep.SaveNote(ctx, note); err != nil {
			s.fail("soep notes", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "SOEP_NOTE", n.ID)
		s.summary.SOEPNotes++
	}
}

// ---------------------------------------------------------------------------
// Informed consents (RF-06)
// ---------------------------------------------------------------------------

// demoConsent describes one informed consent of the demo dataset.
type demoConsent struct {
	ID            string
	PatientID     string
	ProcedureCode string
	Title         string
	RenderedText  string
	SignMethod    string
	Strokes       string
	OTP           string
	SignedByName  string
	SignedByDoc   string
}

const demoConsentOTPCode = "654321"

var demoConsents = []demoConsent{
	{
		ID:            "demo-consent-001",
		PatientID:     demoPatientAnaID,
		ProcedureCode: "D2391",
		Title:         "Consentimiento de resina compuesta",
		RenderedText:  "Yo, Ana Lucía Fernández, IDENTIFICACIÓN 0912345678, autorizo a la odontóloga tratante a realizar una restauración con resina compuesta en la pieza 16, informed del alcance, las alternativas y los riesgos del procedimiento.",
	},
	{
		ID:            "demo-consent-002",
		PatientID:     demoPatientCarlosID,
		ProcedureCode: "D3330",
		Title:         "Consentimiento de tratamiento de conductos",
		RenderedText:  "Yo, Carlos Andrés Mendoza, IDENTIFICACIÓN 1312345679, autorizo el tratamiento de conductos de la pieza 36, informado de la necesidad de anticoagulación suspendida antes del procedimiento y de los riesgos del tratamiento.",
		SignMethod:    "BIOMETRIC",
		Strokes:       "<svg viewBox=\"0 0 240 80\"><path d=\"M12 52 C 60 8, 120 66, 228 26\" /></svg>",
		SignedByName:  "Carlos Andrés Mendoza",
		SignedByDoc:   "1312345679",
	},
	{
		ID:            "demo-consent-003",
		PatientID:     demoPatientMateoID,
		ProcedureCode: "D7140",
		Title:         "Consentimiento de exodoncia",
		RenderedText:  "Yo, Mateo Emiliano Cruz, IDENTIFICACIÓN 0712345672, autorizo la exodoncia de la pieza 46, informado del procedimiento, del proceso de cicatrización y de las alternativas conservadoras.",
		SignMethod:    "OTP",
		OTP:           demoConsentOTPCode,
		SignedByName:  "Mateo Emiliano Cruz",
		SignedByDoc:   "0712345672",
	},
	{
		ID:            "demo-consent-004",
		PatientID:     demoPatientValentinaID,
		ProcedureCode: "D6010",
		Title:         "Consentimiento de corona protésica",
		RenderedText:  "Yo, Valentina Armas, IDENTIFICACIÓN 1012345673, autorizo la colocación de una corona de porcelana, informada de los costos, los tiempos de laboratorio y las alternativas metálicas.",
	},
}

func (s *demoSeeder) seedConsents(ctx context.Context) {
	for _, c := range demoConsents {
		if existing, err := s.deps.consents.GetConsentByID(ctx, c.PatientID, c.ID); err == nil && existing != nil {
			continue
		}
		consent, err := clinicaldom.NewPatientConsent(c.ID, c.PatientID, c.ProcedureCode, c.Title, c.RenderedText, "", s.now)
		if err != nil {
			s.fail("consents", err)
			continue
		}
		if c.SignMethod != "" {
			artifact, err := consent.Sign(clinicaldom.SignConsentParams{
				Method:            c.SignMethod,
				BiometricSVGStrokes: c.Strokes,
				OTPCode:           c.OTP,
				SignedByName:      c.SignedByName,
				SignedByDocument:  c.SignedByDoc,
				DeviceMetadata:    &clinicaldom.DeviceMetadata{ScreenInches: 11.0, TouchPoints: 12},
			}, s.now)
			if err != nil {
				s.fail("consents", err)
				continue
			}
			if s.deps.blobs != nil {
				if err := s.deps.blobs.PutEncryptedPDF(ctx, consent.StorageObjectKey, artifact, consent.DocumentSHA256); err != nil {
					s.fail("consents", err)
					continue
				}
			}
		}
		if err := s.deps.consents.SaveConsent(ctx, consent); err != nil {
			s.fail("consents", err)
			continue
		}
		s.audit(ctx, auditdom.ActionCreate, "INFORMED_CONSENT", c.ID)
		s.summary.Consents++
	}
}

// demoDatasetLabel returns a short human readable label of the dataset, used
// by the boot log.
func demoDatasetLabel(summary demoSummary) string {
	parts := []string{
		fmt.Sprintf("patients=%d", summary.Patients),
		fmt.Sprintf("anamneses=%d", summary.Anamneses),
		fmt.Sprintf("appointments=%d", summary.Appointments),
		fmt.Sprintf("treatment_plans=%d", summary.TreatmentPlans),
		fmt.Sprintf("odontogram_findings=%d", summary.OdontogramFindings),
		fmt.Sprintf("periodontogram_charts=%d", summary.PeriodontogramCards),
		fmt.Sprintf("soep_notes=%d", summary.SOEPNotes),
		fmt.Sprintf("consents=%d", summary.Consents),
	}
	return strings.Join(parts, " ")
}
