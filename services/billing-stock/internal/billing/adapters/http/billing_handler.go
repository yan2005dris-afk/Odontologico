package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sigo-dental/billing-stock/internal/billing/application"
	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

type BillingHandler struct {
	service *application.BillingService
}

func NewBillingHandler(service *application.BillingService) *BillingHandler {
	return &BillingHandler{service: service}
}

func (h *BillingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/billing/cash-registers", h.handleOpenCashRegister)
	mux.HandleFunc("GET /api/v1/billing/cash-registers/{id}", h.handleGetCashRegister)
	mux.HandleFunc("POST /api/v1/billing/cash-registers/{id}/blind-close", h.handleBlindClose)
	mux.HandleFunc("GET /api/v1/billing/cash-registers/{id}/audit-report", h.handleGetAuditReport)
	mux.HandleFunc("POST /api/v1/billing/invoices", h.handleIssueInvoice)
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
	case errors.Is(err, domain.ErrBlindCashCloseEnforced):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "BLIND_CASH_CLOSE_ENFORCED"})
	case errors.Is(err, application.ErrForbiddenRole):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "FORBIDDEN_ROLE"})
	case errors.Is(err, domain.ErrCashRegisterNotFound), errors.Is(err, domain.ErrInvoiceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrCashRegisterAlreadyClosed):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "CASH_REGISTER_ALREADY_CLOSED"})
	case errors.Is(err, domain.ErrPaymentSumMismatch):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "PAYMENT_SUM_MISMATCH"})
	case errors.Is(err, domain.ErrNegativeDeclaredCash), errors.Is(err, domain.ErrInvalidInvoiceInput), errors.Is(err, domain.ErrInvalidCashRegisterInput):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
	}
}

func (h *BillingHandler) handleOpenCashRegister(w http.ResponseWriter, r *http.Request) {
	var cmd application.OpenCashRegisterCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	view, err := h.service.OpenCashRegister(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (h *BillingHandler) handleGetCashRegister(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	view, err := h.service.GetCashRegisterStatus(r.Context(), extractActor(r), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *BillingHandler) handleBlindClose(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var cmd application.BlindCloseCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	cmd.CashRegisterID = id
	receipt, err := h.service.ExecuteBlindClose(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

func (h *BillingHandler) handleGetAuditReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	report, err := h.service.GetAuditReport(r.Context(), extractActor(r), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *BillingHandler) handleIssueInvoice(w http.ResponseWriter, r *http.Request) {
	var cmd application.IssueInvoiceCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	inv, err := h.service.IssueInvoice(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}
