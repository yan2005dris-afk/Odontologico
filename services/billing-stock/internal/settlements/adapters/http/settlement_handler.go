package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	mux.HandleFunc("GET /api/v1/settlements/rules", h.handleListCommissionRules)
	mux.HandleFunc("POST /api/v1/settlements/generate", h.handleGenerateSettlement)
	mux.HandleFunc("GET /api/v1/settlements", h.handleListSettlements)
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

// parseLimitOffset reads the pagination parameters. Unparseable values fall back to the
// defaults applied by the service (limit=50 with a hard cap of 200, offset=0).
func parseLimitOffset(query url.Values) (int, int) {
	limit, _ := strconv.Atoi(strings.TrimSpace(query.Get("limit")))
	offset, _ := strconv.Atoi(strings.TrimSpace(query.Get("offset")))
	return limit, offset
}

// parseDateParam reads an ISO-8601 date filter. It accepts RFC3339 timestamps and plain
// calendar dates; a plain date used as upper bound covers the whole day.
func parseDateParam(raw string, endOfDay bool) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		utc := ts.UTC()
		return &utc, nil
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		utc := ts.UTC()
		if endOfDay {
			utc = utc.Add(24*time.Hour - time.Nanosecond)
		}
		return &utc, nil
	}
	return nil, fmt.Errorf("invalid date filter %q", raw)
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

// handleListSettlements answers GET /api/v1/settlements with the {items,total} contract.
// Filters: dentist_id, from, to, limit, offset.
func (h *SettlementHandler) handleListSettlements(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.SettlementFilter{
		DentistID: strings.TrimSpace(query.Get("dentist_id")),
	}
	filter.Limit, filter.Offset = parseLimitOffset(query)

	from, err := parseDateParam(query.Get("from"), false)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_QUERY"})
		return
	}
	to, err := parseDateParam(query.Get("to"), true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_QUERY"})
		return
	}
	filter.From = from
	filter.To = to

	items, total, err := h.service.ListSettlements(r.Context(), extractActor(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

// handleListCommissionRules answers GET /api/v1/settlements/rules with the {items,total}
// contract. Filters: dentist_id, procedure_code, limit, offset.
func (h *SettlementHandler) handleListCommissionRules(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.CommissionRuleFilter{
		DentistID:     strings.TrimSpace(query.Get("dentist_id")),
		ProcedureCode: strings.TrimSpace(query.Get("procedure_code")),
	}
	filter.Limit, filter.Offset = parseLimitOffset(query)

	items, total, err := h.service.ListCommissionRules(r.Context(), extractActor(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}
