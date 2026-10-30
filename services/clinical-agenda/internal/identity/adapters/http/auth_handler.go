package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sigo-dental/clinical-agenda/internal/identity/application"
	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

type AuthHandler struct {
	service *application.AuthService
}

func NewAuthHandler(service *application.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/mfa/verify", h.handleVerifyMFA)
	mux.HandleFunc("POST /api/v1/auth/heartbeat", h.handleHeartbeat)
	mux.HandleFunc("POST /api/v1/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/v1/users", h.handleListUsers)
}

// handleListUsers answers the staff listing with the {items,total} contract.
// The service restricts it to the `admin` role (SRS §3).
func (h *AuthHandler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))

	filter := application.UserFilter{
		Query:  query.Get("q"),
		Role:   domain.Role(strings.TrimSpace(query.Get("role"))),
		Limit:  limit,
		Offset: offset,
	}

	items, total, err := h.service.ListUsers(r.Context(), extractActor(r), filter)
	if err != nil {
		mapDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req application.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "email and password are required")
		return
	}
	req.SourceIP = sourceIPFromRequest(r)

	out, err := h.service.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAccountLocked):
			writeError(w, http.StatusLocked, "ACCOUNT_LOCKED", err.Error())
		case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrAccountInactive):
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (h *AuthHandler) handleVerifyMFA(w http.ResponseWriter, r *http.Request) {
	var req application.VerifyMFAInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "malformed JSON request body")
		return
	}
	if strings.TrimSpace(req.ChallengeToken) == "" || strings.TrimSpace(req.TOTPCode) == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "challenge_token and totp_code are required")
		return
	}
	req.SourceIP = sourceIPFromRequest(r)

	out, err := h.service.VerifyMFA(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrMFADisabled):
			writeError(w, http.StatusConflict, "MFA_DISABLED", err.Error())
		case errors.Is(err, domain.ErrAccountLocked):
			writeError(w, http.StatusLocked, "ACCOUNT_LOCKED", err.Error())
		case errors.Is(err, domain.ErrInvalidMFACode), errors.Is(err, application.ErrInvalidChallengeToken):
			writeError(w, http.StatusUnauthorized, "INVALID_MFA_CODE", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, out)
}

func (h *AuthHandler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.Header.Get("X-Session-ID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_SESSION_ID", "X-Session-ID header is required")
		return
	}
	if _, err := h.service.ValidateAndTouchSession(r.Context(), sessionID); err != nil {
		if errors.Is(err, domain.ErrSessionIdleTimeout) {
			writeError(w, http.StatusUnauthorized, "SESSION_IDLE_TIMEOUT", err.Error())
			return
		}
		writeError(w, http.StatusUnauthorized, "SESSION_REVOKED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.Header.Get("X-Session-ID"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_SESSION_ID", "X-Session-ID header is required")
		return
	}
	_ = h.service.Logout(r.Context(), sessionID)
	w.WriteHeader(http.StatusNoContent)
}

func sourceIPFromRequest(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Source-IP")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); ip != "" {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return "127.0.0.1"
}

// extractActor resolves the caller identity injected by the edge gateway.
func extractActor(r *http.Request) application.ActorMeta {
	actorID := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actorID == "" {
		actorID = "system-actor"
	}
	return application.ActorMeta{
		ActorID:     actorID,
		ActorRole:   strings.TrimSpace(r.Header.Get("X-Actor-Role")),
		MFAVerified: strings.EqualFold(strings.TrimSpace(r.Header.Get("X-MFA-Verified")), "true"),
		SourceIP:    sourceIPFromRequest(r),
	}
}

// mapDomainError translates the domain errors of the identity context into the
// HTTP contract of the service.
func mapDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrForbiddenRole):
		writeError(w, http.StatusForbidden, "FORBIDDEN_ROLE", err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrAccountInactive):
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
	case errors.Is(err, domain.ErrAccountLocked):
		writeError(w, http.StatusLocked, "ACCOUNT_LOCKED", err.Error())
	case errors.Is(err, domain.ErrInvalidMFACode):
		writeError(w, http.StatusUnauthorized, "INVALID_MFA_CODE", err.Error())
	case errors.Is(err, application.ErrInvalidChallengeToken):
		writeError(w, http.StatusUnauthorized, "INVALID_MFA_CODE", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"code":    code,
		"message": message,
	})
}
