package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/audit/domain"
)

type AuditHandler struct {
	service *application.AuditService
}

func NewAuditHandler(service *application.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

func (h *AuditHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/audit/logs", h.handleListLogs)
}

type listLogsResponse struct {
	Items []*domain.AuditEntry `json:"items"`
	Total int                  `json:"total"`
}

func (h *AuditHandler) handleListLogs(w http.ResponseWriter, r *http.Request) {
	mfaVerified := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-MFA-Verified")), "true")
	if !mfaVerified {
		writeJSONError(w, http.StatusForbidden, "MFA_REQUIRED", "MFA verification is required to inspect audit logs")
		return
	}

	actorRole := strings.TrimSpace(r.Header.Get("X-Actor-Role"))
	q := r.URL.Query()
	filter := application.AuditFilter{
		ResourceType: strings.TrimSpace(q.Get("resource_type")),
		ResourceID:   strings.TrimSpace(q.Get("resource_id")),
		ActorID:      strings.TrimSpace(q.Get("actor_id")),
		Action:       strings.TrimSpace(q.Get("action")),
		Limit:        50,
		Offset:       0,
	}

	if lStr := q.Get("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil || l < 1 || l > 200 {
			writeJSONError(w, http.StatusBadRequest, "INVALID_LIMIT", "limit must be an integer between 1 and 200")
			return
		}
		filter.Limit = l
	}
	if oStr := q.Get("offset"); oStr != "" {
		o, err := strconv.Atoi(oStr)
		if err != nil || o < 0 {
			writeJSONError(w, http.StatusBadRequest, "INVALID_OFFSET", "offset must be a non-negative integer")
			return
		}
		filter.Offset = o
	}
	if fromStr := q.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_FROM_TIMESTAMP", "from must be ISO-8601 / RFC3339")
			return
		}
		filter.From = &t
	}
	if toStr := q.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_TO_TIMESTAMP", "to must be ISO-8601 / RFC3339")
			return
		}
		filter.To = &t
	}

	items, total, err := h.service.ListLogs(r.Context(), actorRole, filter)
	if err != nil {
		if errors.Is(err, application.ErrAuditForbidden) {
			writeJSONError(w, http.StatusForbidden, "FORBIDDEN_ROLE", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listLogsResponse{
		Items: items,
		Total: total,
	})
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}
