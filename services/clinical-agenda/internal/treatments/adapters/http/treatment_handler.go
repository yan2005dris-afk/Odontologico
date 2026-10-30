package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sigo-dental/clinical-agenda/internal/treatments/application"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

type TreatmentHandler struct {
	service *application.TreatmentService
}

func NewTreatmentHandler(service *application.TreatmentService) *TreatmentHandler {
	return &TreatmentHandler{service: service}
}

func (h *TreatmentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/patients/{id}/treatment-plans", h.handleCreatePlan)
	mux.HandleFunc("POST /api/v1/treatment-plans/{planId}/approve", h.handleApprovePlan)
	mux.HandleFunc("POST /api/v1/treatment-plans/{planId}/items/{itemId}/complete", h.handleCompleteItem)
	mux.HandleFunc("DELETE /api/v1/treatment-plans/{planId}/items/{itemId}", h.handleDeleteItem)
	mux.HandleFunc("GET /api/v1/patients/{id}/schedulable-tasks", h.handleListSchedulableTasks)
}

func (h *TreatmentHandler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req application.CreateTreatmentPlanInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	plan, err := h.service.CreatePlan(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapTreatmentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (h *TreatmentHandler) handleApprovePlan(w http.ResponseWriter, r *http.Request) {
	var req application.ApprovePlanInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	out, err := h.service.ApprovePlan(r.Context(), extractActor(r), r.PathValue("planId"), req)
	if err != nil {
		mapTreatmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *TreatmentHandler) handleCompleteItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.CompleteItem(r.Context(), extractActor(r), r.PathValue("planId"), r.PathValue("itemId"))
	if err != nil {
		mapTreatmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *TreatmentHandler) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteItem(r.Context(), extractActor(r), r.PathValue("planId"), r.PathValue("itemId")); err != nil {
		mapTreatmentError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TreatmentHandler) handleListSchedulableTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.ListSchedulableTasks(r.Context(), extractActor(r), r.PathValue("id"))
	if err != nil {
		mapTreatmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tasks})
}

func extractActor(r *http.Request) application.ActorMeta {
	actorID := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actorID == "" {
		actorID = "dentist-1"
	}
	ip := strings.TrimSpace(r.Header.Get("X-Source-IP"))
	if ip == "" {
		ip = "127.0.0.1"
	}
	return application.ActorMeta{
		ActorID:     actorID,
		ActorRole:   strings.TrimSpace(r.Header.Get("X-Actor-Role")),
		MFAVerified: strings.EqualFold(strings.TrimSpace(r.Header.Get("X-MFA-Verified")), "true"),
		SourceIP:    ip,
	}
}

func mapTreatmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrTreatmentMFARequired):
		writeErr(w, http.StatusForbidden, "MFA_REQUIRED", err.Error())
	case errors.Is(err, application.ErrTreatmentForbiddenRole), errors.Is(err, domain.ErrDiscountLimitExceeded):
		writeErr(w, http.StatusForbidden, "FORBIDDEN_ROLE_OR_DISCOUNT", err.Error())
	case errors.Is(err, domain.ErrTreatmentPlanNotFound), errors.Is(err, domain.ErrTreatmentItemNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrClinicalRecordImmutable):
		writeErr(w, http.StatusConflict, "CLINICAL_RECORD_IMMUTABLE", err.Error())
	case errors.Is(err, domain.ErrInvalidTreatmentPlan),
		errors.Is(err, domain.ErrInvalidMonetaryAmount),
		errors.Is(err, domain.ErrInvalidDurationMinutes),
		errors.Is(err, domain.ErrInvalidApprovalRequest):
		writeErr(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
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
