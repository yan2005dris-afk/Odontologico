package postgres

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

const (
	InsertAppointmentSQL = `INSERT INTO appointments (id, patient_id, dentist_id, chair_id, clinic_id, schedulable_task_id, start_at, end_at, status, reason, notes, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())`
)

type SchedulingRepository struct {
	mu           sync.RWMutex
	appointments map[string]*domain.Appointment
	jobsByAppt   map[string][]domain.ScheduledReminder
}

func NewSchedulingRepository() *SchedulingRepository {
	return &SchedulingRepository{
		appointments: make(map[string]*domain.Appointment),
		jobsByAppt:   make(map[string][]domain.ScheduledReminder),
	}
}

func (r *SchedulingRepository) CreateAppointment(_ context.Context, appt *domain.Appointment, jobs []domain.ScheduledReminder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.checkOverlapLocked(appt); err != nil {
		return err
	}

	r.appointments[appt.ID] = cloneAppointment(appt)
	copiedJobs := make([]domain.ScheduledReminder, len(jobs))
	copy(copiedJobs, jobs)
	r.jobsByAppt[appt.ID] = copiedJobs
	return nil
}

func (r *SchedulingRepository) UpdateAppointment(_ context.Context, appt *domain.Appointment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.appointments[appt.ID]; !ok {
		return domain.ErrAppointmentNotFound
	}
	if appt.BlocksChair() {
		if err := r.checkOverlapLocked(appt); err != nil {
			return err
		}
	}
	r.appointments[appt.ID] = cloneAppointment(appt)
	return nil
}

func (r *SchedulingRepository) checkOverlapLocked(candidate *domain.Appointment) error {
	for _, existing := range r.appointments {
		if existing.ID == candidate.ID || !existing.BlocksChair() {
			continue
		}
		if existing.OverlapsWith(candidate.StartTime, candidate.EndTime) {
			// RN-03: Never allow two simultaneous appointments on the same physical chair regardless of dentist
			if existing.ChairID == candidate.ChairID {
				return domain.ErrChairOverlapConflict
			}
			if existing.DentistID == candidate.DentistID {
				return domain.ErrDentistOverlapConflict
			}
		}
	}
	return nil
}

func (r *SchedulingRepository) GetByID(_ context.Context, id string) (*domain.Appointment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	appt, ok := r.appointments[id]
	if !ok {
		return nil, domain.ErrAppointmentNotFound
	}
	return cloneAppointment(appt), nil
}

func (r *SchedulingRepository) ListCalendar(_ context.Context, filter application.CalendarFilter) ([]*domain.Appointment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var windowStart, windowEnd time.Time
	if !filter.Date.IsZero() {
		y, m, d := filter.Date.UTC().Date()
		windowStart = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		if strings.EqualFold(filter.View, "week") {
			windowEnd = windowStart.Add(7 * 24 * time.Hour)
		} else {
			windowEnd = windowStart.Add(24 * time.Hour)
		}
	}

	var matched []*domain.Appointment
	for _, appt := range r.appointments {
		if filter.ChairID != "" && appt.ChairID != filter.ChairID {
			continue
		}
		if filter.DentistID != "" && appt.DentistID != filter.DentistID {
			continue
		}
		if filter.ClinicID != "" && appt.ClinicID != filter.ClinicID {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(appt.Status, filter.Status) {
			continue
		}
		if !windowStart.IsZero() {
			if appt.StartTime.Before(windowStart) || !appt.StartTime.Before(windowEnd) {
				continue
			}
		}
		matched = append(matched, cloneAppointment(appt))
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].StartTime.Before(matched[j].StartTime)
	})
	return matched, nil
}

func (r *SchedulingRepository) CancelPendingReminders(_ context.Context, appointmentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	jobs := r.jobsByAppt[appointmentID]
	for i := range jobs {
		if jobs[i].Status == "PENDING" {
			jobs[i].Status = "CANCELLED"
		}
	}
	r.jobsByAppt[appointmentID] = jobs
	return nil
}

func (r *SchedulingRepository) ListReminderJobs(_ context.Context, appointmentID string) ([]domain.ScheduledReminder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	jobs := r.jobsByAppt[appointmentID]
	out := make([]domain.ScheduledReminder, len(jobs))
	copy(out, jobs)
	return out, nil
}

func cloneAppointment(a *domain.Appointment) *domain.Appointment {
	cp := *a
	cp.ScheduledReminders = make([]domain.ScheduledReminder, len(a.ScheduledReminders))
	copy(cp.ScheduledReminders, a.ScheduledReminders)
	return &cp
}
