package http

import (
	"encoding/json"
	"errors"
	"net/http"
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
