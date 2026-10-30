package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sigo-dental/clinical-agenda/internal/patients/application"
	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

type PatientHandler struct {
	service *application.PatientService
}

func NewPatientHandler(service *application.PatientService) *PatientHandler {
	return &PatientHandler{service: service}
}

func (h *PatientHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/patients", h.handleCreatePatient)
	mux.HandleFunc("GET /api/v1/patients", h.handleSearchPatients)
	mux.HandleFunc("GET /api/v1/patients/{id}", h.handleGetPatient)
	mux.HandleFunc("PUT /api/v1/patients/{id}", h.handleUpdatePatient)
	mux.HandleFunc("GET /api/v1/patients/{id}/header", h.handleGetPatientHeader)
	mux.HandleFunc("GET /api/v1/patients/{id}/summary", h.handleGetPatientHeader)
	mux.HandleFunc("PUT /api/v1/patients/{id}/anamnesis", h.handleUpsertAnamnesis)
}

func (h *PatientHandler) handleCreatePatient(w http.ResponseWriter, r *http.Request) {
	var req application.UpsertPatientInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	patient, err := h.service.CreatePatient(r.Context(), extractActor(r), req)
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, patient)
}

func (h *PatientHandler) handleUpdatePatient(w http.ResponseWriter, r *http.Request) {
	var req application.UpsertPatientInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	patient, err := h.service.UpdatePatient(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, patient)
}

func (h *PatientHandler) handleGetPatient(w http.ResponseWriter, r *http.Request) {
	patient, err := h.service.GetPatient(r.Context(), extractActor(r), r.PathValue("id"))
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, patient)
}

func (h *PatientHandler) handleGetPatientHeader(w http.ResponseWriter, r *http.Request) {
	header, err := h.service.GetPatientHeader(r.Context(), extractActor(r), r.PathValue("id"))
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, header)
}

func (h *PatientHandler) handleSearchPatients(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.service.SearchPatients(r.Context(), extractActor(r), q, limit, offset)
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

func (h *PatientHandler) handleUpsertAnamnesis(w http.ResponseWriter, r *http.Request) {
	var req application.UpsertAnamnesisInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	updated, err := h.service.UpsertAnamnesis(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func extractActor(r *http.Request) application.ActorMeta {
	actorID := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actorID == "" {
		actorID = "system-actor"
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

func mapDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrMFARequired):
		writeErr(w, http.StatusForbidden, "MFA_REQUIRED", err.Error())
	case errors.Is(err, application.ErrForbiddenRole):
		writeErr(w, http.StatusForbidden, "FORBIDDEN_ROLE", err.Error())
	case errors.Is(err, domain.ErrPatientNotFound):
		writeErr(w, http.StatusNotFound, "PATIENT_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrDuplicatePatientDocument):
		writeErr(w, http.StatusConflict, "DUPLICATE_PATIENT_DOCUMENT", err.Error())
	case errors.Is(err, domain.ErrLegalGuardianRequiredMinor):
		writeErr(w, http.StatusUnprocessableEntity, "LEGAL_GUARDIAN_REQUIRED_FOR_MINOR", err.Error())
	case errors.Is(err, domain.ErrInvalidBirthDate):
		writeErr(w, http.StatusBadRequest, "INVALID_BIRTH_DATE", err.Error())
	case errors.Is(err, domain.ErrInvalidPatientDemographics),
		errors.Is(err, domain.ErrEmergencyContactRequired),
		errors.Is(err, domain.ErrInvalidAnamnesis):
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
