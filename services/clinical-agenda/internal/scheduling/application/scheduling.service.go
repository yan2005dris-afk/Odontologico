package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

type stdClock struct{}

func (stdClock) Now() time.Time { return time.Now().UTC() }

type SchedulingService struct {
	repo       SchedulingRepository
	dispatcher NotificationDispatcherPort
	audit      auditapp.Recorder
	clock      Clock
}

func NewSchedulingService(repo SchedulingRepository, dispatcher NotificationDispatcherPort, audit auditapp.Recorder, clock Clock) *SchedulingService {
	if clock == nil {
		clock = stdClock{}
	}
	return &SchedulingService{
		repo:       repo,
		dispatcher: dispatcher,
		audit:      audit,
		clock:      clock,
	}
}

func (s *SchedulingService) CreateAppointment(ctx context.Context, actor ActorMeta, input CreateAppointmentInput) (*domain.Appointment, error) {
	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.StartAt))
	if err != nil {
		return nil, domain.ErrInvalidAppointmentWindow
	}
	endAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.EndAt))
	if err != nil {
		return nil, domain.ErrInvalidAppointmentWindow
	}

	now := s.clock.Now()
	apptID := genUUID()
	appt, err := domain.NewAppointment(
		apptID,
		input.PatientID,
		input.DentistID,
		input.ChairID,
		input.ClinicID,
		input.SchedulableTaskID,
		input.Reason,
		startAt,
		endAt,
		now,
		input.NotificationChannels,
	)
	if err != nil {
		return nil, err
	}

	allJobs := domain.BuildAppointmentReminders(appt.ID, appt.StartTime, now, input.NotificationChannels)
	if err := s.repo.CreateAppointment(ctx, appt, allJobs); err != nil {
		return nil, err
	}

	if s.dispatcher != nil {
		for _, job := range allJobs {
			_ = s.dispatcher.EnqueueAndDispatch(ctx, job)
		}
	}

	s.recordAudit(ctx, actor, auditdom.ActionCreate, "APPOINTMENT", appt.ID, map[string]string{
		"chair_id":   appt.ChairID,
		"dentist_id": appt.DentistID,
		"start_at":   appt.StartAt,
	})
	return appt, nil
}

func (s *SchedulingService) RescheduleAppointment(ctx context.Context, actor ActorMeta, appointmentID string, input RescheduleAppointmentInput) (*domain.Appointment, error) {
	appt, err := s.repo.GetByID(ctx, strings.TrimSpace(appointmentID))
	if err != nil {
		return nil, err
	}
	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.StartAt))
	if err != nil {
		return nil, domain.ErrInvalidAppointmentWindow
	}
	endAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.EndAt))
	if err != nil {
		return nil, domain.ErrInvalidAppointmentWindow
	}
	if err := appt.Reschedule(input.ChairID, startAt, endAt); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAppointment(ctx, appt); err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "APPOINTMENT", appt.ID, map[string]string{
		"rescheduled": "true",
		"chair_id":    appt.ChairID,
	})
	return appt, nil
}

func (s *SchedulingService) TransitionAppointment(ctx context.Context, actor ActorMeta, appointmentID string, input TransitionAppointmentInput) (*domain.Appointment, error) {
	appt, err := s.repo.GetByID(ctx, strings.TrimSpace(appointmentID))
	if err != nil {
		return nil, err
	}
	if err := appt.TransitionTo(input.TargetStatus, input.Notes); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAppointment(ctx, appt); err != nil {
		return nil, err
	}
	if appt.Status == domain.StatusCancelado || appt.Status == domain.StatusNoShow {
		_ = s.repo.CancelPendingReminders(ctx, appt.ID)
	}
	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "APPOINTMENT", appt.ID, map[string]string{
		"status": appt.Status,
	})
	return appt, nil
}

func (s *SchedulingService) HandlePatientReply(ctx context.Context, actor ActorMeta, input PatientReplyWebhookInput) (*domain.Appointment, error) {
	appt, err := s.repo.GetByID(ctx, strings.TrimSpace(input.AppointmentID))
	if err != nil {
		return nil, err
	}
	action := strings.ToUpper(strings.TrimSpace(input.ReplyAction))
	target := domain.StatusConfirmado
	if action == "CANCEL" || action == "CANCELAR" {
		target = domain.StatusCancelado
	}
	if appt.Status == target {
		return appt, nil
	}
	return s.TransitionAppointment(ctx, actor, appt.ID, TransitionAppointmentInput{
		TargetStatus: target,
		Notes:        "Updated via patient reply webhook (" + input.Channel + ")",
	})
}

func (s *SchedulingService) QueryCalendar(ctx context.Context, filter CalendarFilter) ([]*domain.Appointment, error) {
	if strings.TrimSpace(filter.View) == "" {
		filter.View = "day"
	}
	return s.repo.ListCalendar(ctx, filter)
}

func (s *SchedulingService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	actorID := strings.TrimSpace(actor.ActorID)
	if actorID == "" {
		actorID = "system-scheduler"
	}
	role := strings.TrimSpace(actor.ActorRole)
	if role == "" {
		role = "receptionist"
	}
	_, _ = s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actorID,
		ActorRole:    role,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	})
}

func genUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
