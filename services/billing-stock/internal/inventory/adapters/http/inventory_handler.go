package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sigo-dental/billing-stock/internal/inventory/application"
	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

type InventoryHandler struct {
	service *application.InventoryService
}

func NewInventoryHandler(service *application.InventoryService) *InventoryHandler {
	return &InventoryHandler{service: service}
}

func (h *InventoryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/inventory/items", h.handleUpsertSupply)
	mux.HandleFunc("GET /api/v1/inventory/items", h.handleListSupplies)
	mux.HandleFunc("PUT /api/v1/inventory/recipes/{procedureCode}", h.handleSaveRecipe)
	mux.HandleFunc("GET /api/v1/inventory/recipes", h.handleListRecipes)
	mux.HandleFunc("POST /api/v1/inventory/deductions/procedure-completed", h.handleDeductProcedure)
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

// parseBoolParam reads a boolean flag. An absent or unparseable value is false.
func parseBoolParam(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	return value == "true" || value == "1"
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrMFARequired):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "MFA_REQUIRED"})
	case errors.Is(err, application.ErrForbiddenRole):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "FORBIDDEN_ROLE"})
	case errors.Is(err, domain.ErrRecipeNotFound), errors.Is(err, domain.ErrSupplyNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInsufficientSupplyStock):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "INSUFFICIENT_SUPPLY_STOCK"})
	case errors.Is(err, domain.ErrInvalidSupplyInput), errors.Is(err, domain.ErrInvalidRecipeInput):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
	}
}

func (h *InventoryHandler) handleUpsertSupply(w http.ResponseWriter, r *http.Request) {
	var cmd application.UpsertSupplyCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	sup, err := h.service.UpsertSupply(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sup)
}

func (h *InventoryHandler) handleSaveRecipe(w http.ResponseWriter, r *http.Request) {
	procCode := r.PathValue("procedureCode")
	var cmd application.SaveRecipeCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	cmd.ProcedureCode = procCode
	rec, err := h.service.SaveProcedureRecipe(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h *InventoryHandler) handleDeductProcedure(w http.ResponseWriter, r *http.Request) {
	var cmd application.DeductProcedureCompletedCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_JSON"})
		return
	}
	res, err := h.service.DeductForCompletedProcedure(r.Context(), extractActor(r), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleListSupplies answers GET /api/v1/inventory/items with the {items,total} contract.
// Filters: clinic_id, q, low_stock, limit, offset.
func (h *InventoryHandler) handleListSupplies(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.SupplyFilter{
		ClinicID:     strings.TrimSpace(query.Get("clinic_id")),
		Query:        strings.TrimSpace(query.Get("q")),
		LowStockOnly: parseBoolParam(query.Get("low_stock")),
	}
	filter.Limit, filter.Offset = parseLimitOffset(query)

	items, total, err := h.service.ListSupplies(r.Context(), extractActor(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

// handleListRecipes answers GET /api/v1/inventory/recipes with the {items,total}
// contract. Filters: procedure_code, limit, offset.
func (h *InventoryHandler) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.RecipeFilter{
		ProcedureCode: strings.TrimSpace(query.Get("procedure_code")),
	}
	filter.Limit, filter.Offset = parseLimitOffset(query)

	items, total, err := h.service.ListRecipes(r.Context(), extractActor(r), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}
