package application

import (
	"context"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

type CalendarFilter struct {
	View      string // day | week
	Date      time.Time
	ChairID   string
	DentistID string
	ClinicID  string
	Status    string
}

type SchedulingRepository interface {
	CreateAppointment(ctx context.Context, appt *domain.Appointment, jobs []domain.ScheduledReminder) error
	UpdateAppointment(ctx context.Context, appt *domain.Appointment) error
	GetByID(ctx context.Context, id string) (*domain.Appointment, error)
	ListCalendar(ctx context.Context, filter CalendarFilter) ([]*domain.Appointment, error)
	CancelPendingReminders(ctx context.Context, appointmentID string) error
	ListReminderJobs(ctx context.Context, appointmentID string) ([]domain.ScheduledReminder, error)
}

type NotificationDispatcherPort interface {
	EnqueueAndDispatch(ctx context.Context, job domain.ScheduledReminder) error
}

type Clock interface {
	Now() time.Time
}

type ActorMeta struct {
	ActorID   string
	ActorRole string
	SourceIP  string
}

type CreateAppointmentInput struct {
	PatientID            string   `json:"patient_id"`
	DentistID            string   `json:"dentist_id"`
	ChairID              string   `json:"chair_id"`
	ClinicID             string   `json:"clinic_id"`
	SchedulableTaskID    string   `json:"schedulable_task_id"`
	StartAt              string   `json:"start_at"`
	EndAt                string   `json:"end_at"`
	Reason               string   `json:"reason"`
	NotificationChannels []string `json:"notification_channels"`
}

type RescheduleAppointmentInput struct {
	ChairID string `json:"chair_id"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}

type TransitionAppointmentInput struct {
	TargetStatus string `json:"target_status"`
	Notes        string `json:"notes"`
}

type PatientReplyWebhookInput struct {
	AppointmentID string `json:"appointment_id"`
	ReplyAction   string `json:"reply_action"` // CONFIRM | CANCEL
	Channel       string `json:"channel"`
}
