package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

type SchedulingHandler struct {
	service *application.SchedulingService
}

func NewSchedulingHandler(service *application.SchedulingService) *SchedulingHandler {
	return &SchedulingHandler{service: service}
}

func (h *SchedulingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/scheduling/appointments", h.handleCreateAppointment)
	mux.HandleFunc("GET /api/v1/scheduling/appointments", h.handleQueryCalendar)
	mux.HandleFunc("PATCH /api/v1/scheduling/appointments/{id}/reschedule", h.handleReschedule)
	mux.HandleFunc("POST /api/v1/scheduling/appointments/{id}/transition", h.handleTransition)
	mux.HandleFunc("POST /api/v1/scheduling/webhooks/patient-reply", h.handlePatientReplyWebhook)
	mux.HandleFunc("POST /api/v1/scheduling/webhooks/confirmation", h.handlePatientReplyWebhook)
}

func (h *SchedulingHandler) handleCreateAppointment(w http.ResponseWriter, r *http.Request) {
	var req application.CreateAppointmentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	appt, err := h.service.CreateAppointment(r.Context(), extractActor(r), req)
	if err != nil {
		mapSchedulingError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, appt)
}

func (h *SchedulingHandler) handleQueryCalendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var parsedDate time.Time
	if dStr := strings.TrimSpace(q.Get("date")); dStr != "" {
		t, err := time.Parse("2006-01-02", dStr)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_DATE", "date must be YYYY-MM-DD")
			return
		}
		parsedDate = t
	}
	items, err := h.service.QueryCalendar(r.Context(), application.CalendarFilter{
		View:      q.Get("view"),
		Date:      parsedDate,
		ChairID:   strings.TrimSpace(q.Get("chair_id")),
		DentistID: strings.TrimSpace(q.Get("dentist_id")),
		ClinicID:  strings.TrimSpace(q.Get("clinic_id")),
		Status:    strings.TrimSpace(q.Get("status")),
	})
	if err != nil {
		mapSchedulingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *SchedulingHandler) handleReschedule(w http.ResponseWriter, r *http.Request) {
	var req application.RescheduleAppointmentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	appt, err := h.service.RescheduleAppointment(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapSchedulingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appt)
}

func (h *SchedulingHandler) handleTransition(w http.ResponseWriter, r *http.Request) {
	var req application.TransitionAppointmentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	appt, err := h.service.TransitionAppointment(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapSchedulingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appt)
}

func (h *SchedulingHandler) handlePatientReplyWebhook(w http.ResponseWriter, r *http.Request) {
	var req application.PatientReplyWebhookInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	appt, err := h.service.HandlePatientReply(r.Context(), extractActor(r), req)
	if err != nil {
		mapSchedulingError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appt)
}

func extractActor(r *http.Request) application.ActorMeta {
	actorID := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actorID == "" {
		actorID = "receptionist-1"
	}
	role := strings.TrimSpace(r.Header.Get("X-Actor-Role"))
	if role == "" {
		role = "receptionist"
	}
	ip := strings.TrimSpace(r.Header.Get("X-Source-IP"))
	if ip == "" {
		ip = "127.0.0.1"
	}
	return application.ActorMeta{
		ActorID:   actorID,
		ActorRole: role,
		SourceIP:  ip,
	}
}

func mapSchedulingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrChairOverlapConflict):
		writeErr(w, http.StatusConflict, "CHAIR_OVERLAP_CONFLICT", err.Error())
	case errors.Is(err, domain.ErrDentistOverlapConflict):
		writeErr(w, http.StatusConflict, "DENTIST_OVERLAP_CONFLICT", err.Error())
	case errors.Is(err, domain.ErrAppointmentNotFound):
		writeErr(w, http.StatusNotFound, "APPOINTMENT_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrInvalidAppointmentTransition):
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_APPOINTMENT_TRANSITION", err.Error())
	case errors.Is(err, domain.ErrInvalidAppointmentWindow), errors.Is(err, domain.ErrInvalidAppointmentFields):
		writeErr(w, http.StatusBadRequest, "INVALID_APPOINTMENT_REQUEST", err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{
		"code":    code,
		"message": msg,
	})
}
