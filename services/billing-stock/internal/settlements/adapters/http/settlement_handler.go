package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sigo-dental/billing-stock/internal/settlements/application"
	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

type SettlementHandler struct {
	service *application.SettlementService
}

func NewSettlementHandler(service *application.SettlementService) *SettlementHandler {
	return &SettlementHandler{service: service}
}

func (h *SettlementHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/settlements/rules", h.handleConfigureRule)
	mux.HandleFunc("POST /api/v1/settlements/generate", h.handleGenerateSettlement)
}

func extractActor(r *http.Request) application.ActorContext {
	sourceIP := strings.TrimSpace(r.Header.Get("X-Source-IP"))
	if sourceIP == "" {
		sourceIP = r.RemoteAddr
	}
	return application.ActorContext{
		ActorID:     strings.TrimSpace(r.Header.Get("X-Actor-ID")),
		ActorRole:   strings.TrimSpace(r.Header.Get("X-Actor-Role")),
		MFAVerified: strings.EqualFold(strings.TrimSpace(r.Header.Get("X-MFA-Verified")), "true"),
		SourceIP:    sourceIP,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrMFARequired):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "MFA_REQUIRED"})
	case errors.Is(err, application.ErrForbiddenRole):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "FORBIDDEN_ROLE"})
	case errors.Is(err, domain.ErrCommissionRuleNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "COMMISSION_RULE_NOT_FOUND"})
	case errors.Is(err, domain.ErrProcedureAlreadySettled):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "PROCEDURE_ALREADY_SETTLED"})
	case errors.Is(err, domain.ErrInvalidCommissionRule), errors.Is(err, domain.ErrInvalidSettlementInput):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
	}
}

func (h *SettlementHandler) handleConfigureRule(w http.ResponseWriter, r *http.Request) {
	var cmd application.ConfigureRuleCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	rule, err := h.service.ConfigureCommissionRule(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *SettlementHandler) handleGenerateSettlement(w http.ResponseWriter, r *http.Request) {
	var cmd application.GenerateSettlementCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	settlement, err := h.service.GenerateSettlement(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, settlement)
}
