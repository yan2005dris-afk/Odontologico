package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type SOEPConsentHandler struct {
	soepService    *application.SOEPService
	consentService *application.ConsentService
}

func NewSOEPConsentHandler(soepSvc *application.SOEPService, consentSvc *application.ConsentService) *SOEPConsentHandler {
	return &SOEPConsentHandler{
		soepService:    soepSvc,
		consentService: consentSvc,
	}
}

func (h *SOEPConsentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/patients/{id}/soep-notes", h.handleCreateSOEPNote)
	mux.HandleFunc("GET /api/v1/patients/{id}/soep-notes", h.handleListSOEPNotes)
	mux.HandleFunc("PUT /api/v1/patients/{id}/soep-notes/{noteId}", h.handleUpdateSOEPNote)
	mux.HandleFunc("DELETE /api/v1/patients/{id}/soep-notes/{noteId}", h.handleDeleteSOEPNote)
	mux.HandleFunc("POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda", h.handleAddSOEPAddendum)
	mux.HandleFunc("POST /api/v1/patients/{id}/consents", h.handleCreateConsent)
	mux.HandleFunc("POST /api/v1/patients/{id}/consents/{consentId}/sign", h.handleSignConsent)
}

func (h *SOEPConsentHandler) handleCreateSOEPNote(w http.ResponseWriter, r *http.Request) {
	var req application.CreateSOEPNoteInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	note, err := h.soepService.CreateNote(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapSOEPConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func (h *SOEPConsentHandler) handleListSOEPNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := h.soepService.ListNotes(r.Context(), extractActor(r), r.PathValue("id"))
	if err != nil {
		mapSOEPConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": notes})
}

func (h *SOEPConsentHandler) handleUpdateSOEPNote(w http.ResponseWriter, r *http.Request) {
	var req application.UpdateSOEPNoteInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	note, err := h.soepService.UpdateNote(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("noteId"), req)
	if err != nil {
		mapSOEPConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *SOEPConsentHandler) handleDeleteSOEPNote(w http.ResponseWriter, r *http.Request) {
	err := h.soepService.DeleteNote(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("noteId"))
	mapSOEPConsentError(w, err)
}

func (h *SOEPConsentHandler) handleAddSOEPAddendum(w http.ResponseWriter, r *http.Request) {
	var req application.CreateSOEPAddendumInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	add, err := h.soepService.AddAddendum(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("noteId"), req)
	if err != nil {
		mapSOEPConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, add)
}

func (h *SOEPConsentHandler) handleCreateConsent(w http.ResponseWriter, r *http.Request) {
	var req application.CreateConsentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	consent, err := h.consentService.CreateConsent(r.Context(), extractActor(r), r.PathValue("id"), req)
	if err != nil {
		mapSOEPConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, consent)
}

func (h *SOEPConsentHandler) handleSignConsent(w http.ResponseWriter, r *http.Request) {
	var req application.SignConsentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	signed, err := h.consentService.SignConsent(r.Context(), extractActor(r), r.PathValue("id"), r.PathValue("consentId"), req)
	if err != nil {
		mapSOEPConsentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, signed)
}

func mapSOEPConsentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrClinicalMFARequired):
		writeErr(w, http.StatusForbidden, "MFA_REQUIRED", err.Error())
	case errors.Is(err, application.ErrClinicalForbiddenRole), errors.Is(err, domain.ErrSOEPDifferentAuthor):
		writeErr(w, http.StatusForbidden, "FORBIDDEN_ROLE", err.Error())
	case errors.Is(err, domain.ErrSOEPNotFound), errors.Is(err, domain.ErrConsentNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrSOEPNoteLocked):
		writeErr(w, http.StatusConflict, "SOEP_NOTE_LOCKED", err.Error())
	case errors.Is(err, domain.ErrConsentAlreadySigned):
		writeErr(w, http.StatusConflict, "CONSENT_ALREADY_SIGNED", err.Error())
	case errors.Is(err, domain.ErrClinicalRecordImmutable):
		writeErr(w, http.StatusConflict, "CLINICAL_RECORD_IMMUTABLE", err.Error())
	case errors.Is(err, domain.ErrInvalidSOEPFields),
		errors.Is(err, domain.ErrInvalidSOEPAddendum),
		errors.Is(err, domain.ErrInvalidConsentTemplate),
		errors.Is(err, domain.ErrInvalidSignatureMethod),
		errors.Is(err, domain.ErrEmptyBiometricStrokes),
		errors.Is(err, domain.ErrInvalidConsentOTP),
		errors.Is(err, domain.ErrInvalidSignerIdentity):
		writeErr(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
