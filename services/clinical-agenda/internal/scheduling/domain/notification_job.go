package domain

import (
	"fmt"
	"strings"
	"time"
)

type ScheduledReminder struct {
	ID             string    `json:"id,omitempty"`
	AppointmentID  string    `json:"appointment_id,omitempty"`
	Type           string    `json:"type"` // CONFIRMATION_IMMEDIATE | REMINDER_48H | REMINDER_24H
	ScheduledFor   string    `json:"scheduled_for"`
	Channels       []string  `json:"channels"`
	Status         string    `json:"status,omitempty"` // PENDING | DISPATCHED | CANCELLED
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"-"`
}

func BuildAppointmentReminders(appointmentID string, startAt, now time.Time, channels []string) []ScheduledReminder {
	normChannels := normalizeChannels(channels)
	startUTC := startAt.UTC()
	nowUTC := now.UTC()

	r48 := startUTC.Add(-48 * time.Hour)
	r24 := startUTC.Add(-24 * time.Hour)

	return []ScheduledReminder{
		{
			ID:             appointmentID + "-imm",
			AppointmentID:  appointmentID,
			Type:           "CONFIRMATION_IMMEDIATE",
			ScheduledFor:   nowUTC.Format(time.RFC3339),
			Channels:       normChannels,
			Status:         "PENDING",
			IdempotencyKey: fmt.Sprintf("%s:CONFIRMATION_IMMEDIATE:%s", appointmentID, strings.Join(normChannels, ",")),
			CreatedAt:      nowUTC,
		},
		{
			ID:             appointmentID + "-48h",
			AppointmentID:  appointmentID,
			Type:           "REMINDER_48H",
			ScheduledFor:   r48.Format(time.RFC3339),
			Channels:       normChannels,
			Status:         "PENDING",
			IdempotencyKey: fmt.Sprintf("%s:REMINDER_48H:%s", appointmentID, strings.Join(normChannels, ",")),
			CreatedAt:      nowUTC,
		},
		{
			ID:             appointmentID + "-24h",
			AppointmentID:  appointmentID,
			Type:           "REMINDER_24H",
			ScheduledFor:   r24.Format(time.RFC3339),
			Channels:       normChannels,
			Status:         "PENDING",
			IdempotencyKey: fmt.Sprintf("%s:REMINDER_24H:%s", appointmentID, strings.Join(normChannels, ",")),
			CreatedAt:      nowUTC,
		},
	}
}

func normalizeChannels(channels []string) []string {
	if len(channels) == 0 {
		return []string{"WHATSAPP", "EMAIL"}
	}
	out := make([]string, 0, len(channels))
	seen := make(map[string]bool)
	for _, ch := range channels {
		upper := strings.ToUpper(strings.TrimSpace(ch))
		if (upper == "WHATSAPP" || upper == "SMS" || upper == "EMAIL") && !seen[upper] {
			seen[upper] = true
			out = append(out, upper)
		}
	}
	if len(out) == 0 {
		return []string{"WHATSAPP", "EMAIL"}
	}
	return out
}
