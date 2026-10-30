package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	audithttp "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/http"
	"github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/audit/domain"
)

func TestAuditHandler_ListLogs(t *testing.T) {
	repo := postgres.NewAuditRepository()
	svc := application.NewAuditService(repo, nil, nil)
	_, _ = svc.Record(context.Background(), application.RecordInput{
		ActorID:      "dentist-1",
		ActorRole:    "dentist",
		Action:       domain.ActionUpdate,
		ResourceType: "ODONTOGRAM",
		ResourceID:   "patient-1",
		SourceIP:     "10.0.0.1",
	})

	handler := audithttp.NewAuditHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. Missing MFA header -> 403
	reqNoMFA := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs", nil)
	reqNoMFA.Header.Set("X-Actor-Role", "admin")
	recNoMFA := httptest.NewRecorder()
	mux.ServeHTTP(recNoMFA, reqNoMFA)
	if recNoMFA.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when MFA not verified, got %d", recNoMFA.Code)
	}

	// 2. Dentist role -> 403 Forbidden (S5)
	reqDentist := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs", nil)
	reqDentist.Header.Set("X-Actor-Role", "dentist")
	reqDentist.Header.Set("X-MFA-Verified", "true")
	recDentist := httptest.NewRecorder()
	mux.ServeHTTP(recDentist, reqDentist)
	if recDentist.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for dentist, got %d", recDentist.Code)
	}

	// 3. Invalid query params -> 400
	for _, query := range []string{
		"?limit=0",
		"?limit=999",
		"?offset=-1",
		"?from=not-a-date",
		"?to=not-a-date",
	} {
		reqBad := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs"+query, nil)
		reqBad.Header.Set("X-Actor-Role", "auditor")
		reqBad.Header.Set("X-MFA-Verified", "true")
		recBad := httptest.NewRecorder()
		mux.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for query %s, got %d", query, recBad.Code)
		}
	}

	// 4. Valid auditor request -> 200 OK
	from := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	reqOK := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs?resource_type=ODONTOGRAM&resource_id=patient-1&actor_id=dentist-1&action=UPDATE&limit=10&offset=0&from="+from+"&to="+to, nil)
	reqOK.Header.Set("X-Actor-Role", "auditor")
	reqOK.Header.Set("X-MFA-Verified", "true")
	recOK := httptest.NewRecorder()
	mux.ServeHTTP(recOK, reqOK)
	if recOK.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recOK.Code, recOK.Body.String())
	}

	var resp struct {
		Items []domain.AuditEntry `json:"items"`
		Total int                 `json:"total"`
	}
	if err := json.Unmarshal(recOK.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("expected 1 audit item, got total=%d len=%d", resp.Total, len(resp.Items))
	}
}
