package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type OdontogramHandler struct {
	odontogramService     *application.OdontogramService
	periodontogramService *application.PeriodontogramService
}

func NewOdontogramHandler(odoSvc *application.OdontogramService, perioSvc *application.PeriodontogramService) *OdontogramHandler {
	return &OdontogramHandler{
		odontogramService:     odoSvc,
		periodontogramService: perioSvc,
	}
}

func (h *OdontogramHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/patients/{id}/odontogram", h.handleGetOdontogram)
	mux.HandleFunc("POST /api/v1/patients/{id}/odontogram/findings", h.handleRecordFinding)
	mux.HandleFunc("PUT /api/v1/patients/{id}/odontogram/findings/{findingId}", h.handleUpdateFinding)
	mux.HandleFunc("DELETE /api/v1/patients/{id}/odontogram/findings/{findingId}", h.handleDeleteFinding)
	mux.HandleFunc("POST /api/v1/patients/{id}/odontogram/findings/{findingId}/addendum", h.handleAddFindingAddendum)
	mux.HandleFunc("POST /api/v1/patients/{id}/periodontograms", h.handleRecordPeriodontogram)
	mux.HandleFunc("GET /api/v1/patients/{id}/periodontograms", h.handleListPeriodontograms)
}

func (h *OdontogramHandler) handleGetOdontogram(w http.ResponseWriter, r *http.Request) {
	out, err := h.odontogramService.GetOdontogram(r.Context(), extractActor(r), r.PathValue("id"))
	if err != nil {
		mapClinicalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *OdontogramHandler) handleRecordFinding(w http.ResponseWriter, r *http.Request) {
	var req application.RecordFindingInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	out, err := h.odontogramService.RecordFinding(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapClinicalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *OdontogramHandler) handleUpdateFinding(w http.ResponseWriter, r *http.Request) {
	var req application.UpdateFindingInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	out, err := h.odontogramService.UpdateFinding(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("findingId"), req)
	if err != nil {
		mapClinicalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *OdontogramHandler) handleDeleteFinding(w http.ResponseWriter, r *http.Request) {
	if err := h.odontogramService.DeleteFinding(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("findingId")); err != nil {
		mapClinicalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OdontogramHandler) handleAddFindingAddendum(w http.ResponseWriter, r *http.Request) {
	var req application.FindingAddendumInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	out, err := h.odontogramService.AddFindingAddendum(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("findingId"), req)
	if err != nil {
		mapClinicalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *OdontogramHandler) handleRecordPeriodontogram(w http.ResponseWriter, r *http.Request) {
	var req application.RecordPeriodontogramInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	out, err := h.periodontogramService.RecordChart(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapClinicalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *OdontogramHandler) handleListPeriodontograms(w http.ResponseWriter, r *http.Request) {
	out, err := h.periodontogramService.ListCharts(r.Context(), extractActor(r), r.PathValue("id"))
	if err != nil {
		mapClinicalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func extractActor(r *http.Request) application.ActorMeta {
	actorID := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actorID == "" {
		actorID = "dentist-default"
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

func mapClinicalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrClinicalMFARequired):
		writeErr(w, http.StatusForbidden, "MFA_REQUIRED", err.Error())
	case errors.Is(err, application.ErrClinicalForbiddenRole):
		writeErr(w, http.StatusForbidden, "FORBIDDEN_ROLE", err.Error())
	case errors.Is(err, domain.ErrFindingNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrClinicalRecordImmutable):
		writeErr(w, http.StatusConflict, "CLINICAL_RECORD_IMMUTABLE", err.Error())
	case errors.Is(err, domain.ErrInvalidToothCode):
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_TOOTH_CODE", err.Error())
	case errors.Is(err, domain.ErrInvalidNotationSystem),
		errors.Is(err, domain.ErrInvalidDentalSurface),
		errors.Is(err, domain.ErrInvalidFindingType),
		errors.Is(err, domain.ErrInvalidFindingStatus),
		errors.Is(err, domain.ErrInvalidAddendum),
		errors.Is(err, domain.ErrInvalidPerioChart),
		errors.Is(err, domain.ErrInvalidProbingDepth),
		errors.Is(err, domain.ErrInvalidGingivalMargin),
		errors.Is(err, domain.ErrInvalidFurcationGrade),
		errors.Is(err, domain.ErrInvalidMobilityGrade):
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
