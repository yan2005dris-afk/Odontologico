package http

import (
	"encoding/json"
	"errors"
	"net/http"
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
	mux.HandleFunc("PUT /api/v1/inventory/recipes/{procedureCode}", h.handleSaveRecipe)
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
