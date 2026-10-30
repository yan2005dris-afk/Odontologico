package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	StatusProgramado     = "PROGRAMADO"
	StatusConfirmado     = "CONFIRMADO"
	StatusEnSalaDeEspera = "EN_SALA_DE_ESPERA"
	StatusEnAtencion     = "EN_ATENCION"
	StatusAtendido       = "ATENDIDO"
	StatusCancelado      = "CANCELADO"
	StatusNoShow         = "NO_SHOW"
)

var (
	ErrInvalidAppointmentFields      = errors.New("patient_id, dentist_id, chair_id, clinic_id, and reason are required")
	ErrInvalidAppointmentWindow      = errors.New("end_at must be strictly after start_at")
	ErrChairOverlapConflict          = errors.New("RN-03: physical dental chair already has an overlapping active appointment")
	ErrDentistOverlapConflict        = errors.New("dentist already has an overlapping active appointment across chairs")
	ErrInvalidAppointmentTransition  = errors.New("RF-10: invalid appointment state transition")
	ErrAppointmentNotFound           = errors.New("appointment not found")
)

var validStateTransitions = map[string]map[string]bool{
	StatusProgramado: {
		StatusConfirmado:     true,
		StatusEnSalaDeEspera: true,
		StatusCancelado:      true,
		StatusNoShow:         true,
	},
	StatusConfirmado: {
		StatusConfirmado:     true, // idempotent confirmation
		StatusEnSalaDeEspera: true,
		StatusCancelado:      true,
		StatusNoShow:         true,
	},
	StatusEnSalaDeEspera: {
		StatusEnAtencion: true,
		StatusCancelado:  true,
		StatusNoShow:     true,
	},
	StatusEnAtencion: {
		StatusAtendido: true,
	},
	StatusAtendido:  {},
	StatusCancelado: {},
	StatusNoShow:    {},
}

type Appointment struct {
	ID                 string              `json:"id"`
	PatientID          string              `json:"patient_id"`
	DentistID          string              `json:"dentist_id"`
	ChairID            string              `json:"chair_id"`
	ClinicID           string              `json:"clinic_id"`
	SchedulableTaskID  string              `json:"schedulable_task_id,omitempty"`
	StartAt            string              `json:"start_at"`
	EndAt              string              `json:"end_at"`
	StartTime          time.Time           `json:"-"`
	EndTime            time.Time           `json:"-"`
	Status             string              `json:"status"`
	Reason             string              `json:"reason"`
	Notes              string              `json:"notes,omitempty"`
	ScheduledReminders []ScheduledReminder `json:"scheduled_reminders"`
}

func NewAppointment(id, patientID, dentistID, chairID, clinicID, taskID, reason string, startAt, endAt, now time.Time, channels []string) (*Appointment, error) {
	patientID = strings.TrimSpace(patientID)
	dentistID = strings.TrimSpace(dentistID)
	chairID = strings.TrimSpace(chairID)
	clinicID = strings.TrimSpace(clinicID)
	reason = strings.TrimSpace(reason)

	if patientID == "" || dentistID == "" || chairID == "" || clinicID == "" || reason == "" {
		return nil, ErrInvalidAppointmentFields
	}
	startUTC := startAt.UTC()
	endUTC := endAt.UTC()
	if !endUTC.After(startUTC) {
		return nil, ErrInvalidAppointmentWindow
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	allJobs := BuildAppointmentReminders(id, startUTC, now, channels)
	reminders48And24 := []ScheduledReminder{allJobs[1], allJobs[2]}

	return &Appointment{
		ID:                 id,
		PatientID:          patientID,
		DentistID:          dentistID,
		ChairID:            chairID,
		ClinicID:           clinicID,
		SchedulableTaskID:  strings.TrimSpace(taskID),
		StartAt:            startUTC.Format(time.RFC3339),
		EndAt:              endUTC.Format(time.RFC3339),
		StartTime:          startUTC,
		EndTime:            endUTC,
		Status:             StatusProgramado,
		Reason:             reason,
		ScheduledReminders: reminders48And24,
	}, nil
}

func (a *Appointment) BlocksChair() bool {
	return a.Status != StatusCancelado && a.Status != StatusNoShow
}

// OverlapsWith checks half-open interval intersection [startA, endA) && [startB, endB).
func (a *Appointment) OverlapsWith(startB, endB time.Time) bool {
	return a.StartTime.Before(endB.UTC()) && startB.UTC().Before(a.EndTime)
}

func (a *Appointment) TransitionTo(targetStatus, notes string) error {
	target := strings.ToUpper(strings.TrimSpace(targetStatus))
	allowed, ok := validStateTransitions[a.Status]
	if !ok || !allowed[target] {
		return ErrInvalidAppointmentTransition
	}
	a.Status = target
	if strings.TrimSpace(notes) != "" {
		a.Notes = strings.TrimSpace(notes)
	}
	return nil
}

func (a *Appointment) Reschedule(newChairID string, newStart, newEnd time.Time) error {
	if !a.BlocksChair() || a.Status == StatusAtendido {
		return ErrInvalidAppointmentTransition
	}
	startUTC := newStart.UTC()
	endUTC := newEnd.UTC()
	if !endUTC.After(startUTC) {
		return ErrInvalidAppointmentWindow
	}
	if strings.TrimSpace(newChairID) != "" {
		a.ChairID = strings.TrimSpace(newChairID)
	}
	a.StartTime = startUTC
	a.EndTime = endUTC
	a.StartAt = startUTC.Format(time.RFC3339)
	a.EndAt = endUTC.Format(time.RFC3339)
	return nil
}
