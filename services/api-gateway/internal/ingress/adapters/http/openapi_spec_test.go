package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sigo-dental/api-gateway/internal/ingress/application"
	"github.com/sigo-dental/api-gateway/internal/ingress/domain"
)

// expectedContract is the published contract of the platform: every operation
// below was read from the RegisterRoutes of the three Go services
// (clinical-agenda, billing-stock) plus the read contract of wi-15.
var expectedContract = []string{
	// Gateway edge endpoints.
	"GET /healthz",
	"GET /metrics",
	"GET /openapi.json",
	"GET /readyz",
	// clinical-agenda · identity.
	"POST /api/v1/auth/heartbeat",
	"POST /api/v1/auth/login",
	"POST /api/v1/auth/logout",
	"POST /api/v1/auth/mfa/verify",
	"GET /api/v1/users",
	// clinical-agenda · patients.
	"GET /api/v1/patients",
	"POST /api/v1/patients",
	"PUT /api/v1/patients/{id}",
	"GET /api/v1/patients/{id}",
	"PUT /api/v1/patients/{id}/anamnesis",
	"GET /api/v1/patients/{id}/header",
	"GET /api/v1/patients/{id}/odontogram",
	"POST /api/v1/patients/{id}/odontogram/findings",
	"DELETE /api/v1/patients/{id}/odontogram/findings/{findingId}",
	"PUT /api/v1/patients/{id}/odontogram/findings/{findingId}",
	"POST /api/v1/patients/{id}/odontogram/findings/{findingId}/addendum",
	"GET /api/v1/patients/{id}/periodontograms",
	"POST /api/v1/patients/{id}/periodontograms",
	"GET /api/v1/patients/{id}/summary",
	// clinical-agenda · clinical evolution and consents.
	"DELETE /api/v1/patients/{id}/soep-notes/{noteId}",
	"GET /api/v1/patients/{id}/soep-notes",
	"POST /api/v1/patients/{id}/soep-notes",
	"PUT /api/v1/patients/{id}/soep-notes/{noteId}",
	"POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda",
	"GET /api/v1/patients/{id}/consents",
	"POST /api/v1/patients/{id}/consents",
	"POST /api/v1/patients/{id}/consents/{consentId}/sign",
	"GET /api/v1/consents",
	// clinical-agenda · treatment plans.
	"GET /api/v1/patients/{id}/schedulable-tasks",
	"GET /api/v1/patients/{id}/treatment-plans",
	"POST /api/v1/patients/{id}/treatment-plans",
	"GET /api/v1/treatment-plans",
	"DELETE /api/v1/treatment-plans/{planId}/items/{itemId}",
	"POST /api/v1/treatment-plans/{planId}/items/{itemId}/complete",
	"POST /api/v1/treatment-plans/{planId}/approve",
	// clinical-agenda · scheduling and audit.
	"GET /api/v1/audit/logs",
	"GET /api/v1/scheduling/appointments",
	"POST /api/v1/scheduling/appointments",
	"PATCH /api/v1/scheduling/appointments/{id}/reschedule",
	"POST /api/v1/scheduling/appointments/{id}/transition",
	"POST /api/v1/scheduling/webhooks/confirmation",
	"POST /api/v1/scheduling/webhooks/patient-reply",
	// billing-stock · billing.
	"GET /api/v1/billing/cash-registers",
	"POST /api/v1/billing/cash-registers",
	"GET /api/v1/billing/cash-registers/{id}",
	"GET /api/v1/billing/cash-registers/{id}/audit-report",
	"POST /api/v1/billing/cash-registers/{id}/blind-close",
	"GET /api/v1/billing/invoices",
	"POST /api/v1/billing/invoices",
	// billing-stock · settlements.
	"GET /api/v1/settlements",
	"POST /api/v1/settlements/generate",
	"GET /api/v1/settlements/rules",
	"POST /api/v1/settlements/rules",
	// billing-stock · inventory.
	"POST /api/v1/inventory/deductions/procedure-completed",
	"GET /api/v1/inventory/items",
	"POST /api/v1/inventory/items",
	"GET /api/v1/inventory/recipes",
	"PUT /api/v1/inventory/recipes/{procedureCode}",
}

// readContractRoutes are the 11 listings added by wi-15.
var readContractRoutes = []string{
	"GET /api/v1/users",
	"GET /api/v1/treatment-plans",
	"GET /api/v1/patients/{id}/treatment-plans",
	"GET /api/v1/consents",
	"GET /api/v1/patients/{id}/consents",
	"GET /api/v1/billing/invoices",
	"GET /api/v1/billing/cash-registers",
	"GET /api/v1/settlements",
	"GET /api/v1/settlements/rules",
	"GET /api/v1/inventory/items",
	"GET /api/v1/inventory/recipes",
}

// fictitiousRoutes were published by the old default spec but do not exist in
// any of the three services.
var fictitiousRoutes = []string{
	"/api/v1/clinical/odontograms",
	"/api/v1/treatments/plans",
	"/api/v1/settlements/commissions",
	"/api/v1/inventory/supplies",
}

// specOperations flattens an OpenAPI document into a sorted "METHOD /path" list.
func specOperations(t *testing.T, raw string) []string {
	t.Helper()

	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.0") {
		t.Fatalf("expected an OpenAPI 3.0 document, got %q", doc.OpenAPI)
	}

	operations := make([]string, 0, len(doc.Paths))
	for path, item := range doc.Paths {
		for method, operation := range item {
			if strings.TrimSpace(operation.OperationID) == "" {
				t.Errorf("path %s (%s) has no operationId", path, method)
			}
			operations = append(operations, strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(operations)
	return operations
}

func TestDefaultOpenAPIJSON_PublishesOnlyRealRoutes(t *testing.T) {
	got := specOperations(t, defaultOpenAPIJSON())

	want := append([]string{}, expectedContract...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("expected %d published operations, got %d:\n%s", len(want), len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("published operation %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDefaultOpenAPIJSON_ContainsReadContractRoutes(t *testing.T) {
	published := specOperations(t, defaultOpenAPIJSON())
	index := make(map[string]bool, len(published))
	for _, operation := range published {
		index[operation] = true
	}

	for _, route := range readContractRoutes {
		if !index[route] {
			t.Errorf("read-contract route %q is missing from the published contract", route)
		}
	}

	for _, route := range fictitiousRoutes {
		for _, operation := range published {
			if strings.HasSuffix(operation, " "+route) {
				t.Errorf("fictitious route %q is published by the gateway", route)
			}
		}
	}
}

func TestBuildOpenAPIJSONFromYAML(t *testing.T) {
	t.Run("a valid spec of real routes is served as it is", func(t *testing.T) {
		raw := `openapi: 3.0.3
info:
  title: Contrato de pruebas
  version: 2.0.0
paths:
  /api/v1/patients:
    get:
      operationId: listPatients
  /api/v1/users:
    get:
      operationId: listUsers
`
		served := buildOpenAPIJSONFromYAML(raw)
		if served == defaultOpenAPIJSON() {
			t.Fatalf("expected the YAML spec to be served, got the default spec")
		}
		operations := specOperations(t, served)
		if len(operations) != 2 {
			t.Fatalf("expected 2 operations from the YAML spec, got %v", operations)
		}
		if !strings.Contains(served, "Contrato de pruebas") {
			t.Errorf("expected the YAML title to be preserved, got %s", served)
		}
	})

	t.Run("a spec that publishes routes the services do not expose falls back to the default", func(t *testing.T) {
		raw := `openapi: 3.0.3
info:
  title: Spec obsoleto
  version: 1.0.0
paths:
  /api/v1/clinical/odontograms:
    get:
      operationId: getOdontogram
`
		if served := buildOpenAPIJSONFromYAML(raw); served != defaultOpenAPIJSON() {
			t.Fatalf("expected the default spec, got %s", served)
		}
	})

	t.Run("a document that is not OpenAPI 3.0 falls back to the default", func(t *testing.T) {
		if served := buildOpenAPIJSONFromYAML("foo: bar\n"); served != defaultOpenAPIJSON() {
			t.Fatalf("expected the default spec, got %s", served)
		}
	})

	t.Run("an empty document falls back to the default", func(t *testing.T) {
		if served := buildOpenAPIJSONFromYAML("   "); served != defaultOpenAPIJSON() {
			t.Fatalf("expected the default spec, got %s", served)
		}
	})
}

func TestProxyRouter_OpenAPIJSON_ExposesTheRealContract(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	newRouter := func(t *testing.T, specPath string) *ProxyRouter {
		t.Helper()
		router, err := NewProxyRouter(
			application.NewGatewayService(
				domain.NewHS256Verifier("openapi-contract-secret", nil),
				nil,
				nil,
				domain.NewDefaultRBACPolicy(),
			),
			nil,
			RouterConfig{
				ClinicalAgendaURL: upstream.URL,
				BillingStockURL:   upstream.URL,
				OpenAPISpecPath:   specPath,
			},
		)
		if err != nil {
			t.Fatalf("NewProxyRouter failed: %v", err)
		}
		return router
	}

	specPath := filepath.Join("..", "..", "..", "..", "..", "..", "openapi", "sigo-dental.v1.yaml")
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("the OpenAPI artifact is missing: %v", err)
	}

	for name, path := range map[string]string{
		"without an external spec": "",
		"with the repository spec": specPath,
	} {
		t.Run(name, func(t *testing.T) {
			router := newRouter(t, path)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d", recorder.Code)
			}

			published := specOperations(t, recorder.Body.String())
			index := make(map[string]bool, len(published))
			for _, operation := range published {
				index[operation] = true
			}
			for _, route := range readContractRoutes {
				if !index[route] {
					t.Errorf("read-contract route %q is missing from /openapi.json", route)
				}
			}
			for _, route := range fictitiousRoutes {
				for _, operation := range published {
					if strings.HasSuffix(operation, " "+route) {
						t.Errorf("fictitious route %q is published by /openapi.json", route)
					}
				}
			}
		})
	}
}
