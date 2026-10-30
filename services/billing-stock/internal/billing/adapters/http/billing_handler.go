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
	mux.HandleFunc("GET /api/v1/billing/cash-registers", h.handleListCashRegisters)
	mux.HandleFunc("GET /api/v1/billing/cash-registers/{id}", h.handleGetCashRegister)
	mux.HandleFunc("POST /api/v1/billing/cash-registers/{id}/blind-close", h.handleBlindClose)
	mux.HandleFunc("GET /api/v1/billing/cash-registers/{id}/audit-report", h.handleGetAuditReport)
	mux.HandleFunc("POST /api/v1/billing/invoices", h.handleIssueInvoice)
	mux.HandleFunc("GET /api/v1/billing/invoices", h.handleListInvoices)
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

// handleListInvoices answers GET /api/v1/billing/invoices with the {items,total}
// contract. Filters: patient_id, fiscal_status, from, to, limit, offset.
func (h *BillingHandler) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.InvoiceFilter{
		PatientID:    strings.TrimSpace(query.Get("patient_id")),
		FiscalStatus: strings.ToUpper(strings.TrimSpace(query.Get("fiscal_status"))),
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

	items, total, err := h.service.ListInvoices(r.Context(), extractActor(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

// handleListCashRegisters answers GET /api/v1/billing/cash-registers with the
// {items,total} contract. RN-02: an OPEN register never exposes the theoretical total.
func (h *BillingHandler) handleListCashRegisters(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.CashRegisterFilter{
		ClinicID: strings.TrimSpace(query.Get("clinic_id")),
		Status:   strings.ToUpper(strings.TrimSpace(query.Get("status"))),
	}
	filter.Limit, filter.Offset = parseLimitOffset(query)

	items, total, err := h.service.ListCashRegisters(r.Context(), extractActor(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}
