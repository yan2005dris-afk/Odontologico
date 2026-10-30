package http

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

// apiRoute describes one operation published by the platform.
type apiRoute struct {
	Method      string
	OperationID string
	Summary     string
}

// apiRoutes is the single source of truth of the contract published by the
// gateway. Every entry was verified against the RegisterRoutes of the three Go
// services (clinical-agenda, billing-stock) plus the read contract of wi-15;
// the gateway never publishes a route that no service exposes.
var apiRoutes = map[string][]apiRoute{
	// Edge endpoints served by the gateway itself.
	"/healthz": {
		{Method: "GET", OperationID: "getHealthz", Summary: "Gateway liveness probe"},
	},
	"/readyz": {
		{Method: "GET", OperationID: "getReadyz", Summary: "Gateway readiness probe"},
	},
	"/metrics": {
		{Method: "GET", OperationID: "getMetrics", Summary: "Prometheus latency and SLA telemetry"},
	},
	"/openapi.json": {
		{Method: "GET", OperationID: "getOpenAPISpec", Summary: "OpenAPI v3.0 contract of the platform"},
	},

	// clinical-agenda · identity.
	"/api/v1/auth/login": {
		{Method: "POST", OperationID: "authLogin", Summary: "Authenticate credentials and issue the pre-MFA token"},
	},
	"/api/v1/auth/mfa/verify": {
		{Method: "POST", OperationID: "authVerifyMFA", Summary: "Verify the MFA challenge and upgrade the session"},
	},
	"/api/v1/auth/heartbeat": {
		{Method: "POST", OperationID: "authHeartbeat", Summary: "Refresh the sliding idle session"},
	},
	"/api/v1/auth/logout": {
		{Method: "POST", OperationID: "authLogout", Summary: "Revoke the current session"},
	},
	"/api/v1/users": {
		{Method: "GET", OperationID: "listUsers", Summary: "List the clinic staff"},
	},

	// clinical-agenda · patients.
	"/api/v1/patients": {
		{Method: "GET", OperationID: "listPatients", Summary: "Search unified patient records"},
		{Method: "POST", OperationID: "createPatient", Summary: "Register a new patient"},
	},
	"/api/v1/patients/{id}": {
		{Method: "GET", OperationID: "getPatient", Summary: "Read the unified patient record"},
		{Method: "PUT", OperationID: "updatePatient", Summary: "Update the unified patient record"},
	},
	"/api/v1/patients/{id}/header": {
		{Method: "GET", OperationID: "getPatientHeader", Summary: "Read the sticky patient header with critical alerts"},
	},
	"/api/v1/patients/{id}/summary": {
		{Method: "GET", OperationID: "getPatientSummary", Summary: "Read the clinical summary of a patient"},
	},
	"/api/v1/patients/{id}/anamnesis": {
		{Method: "PUT", OperationID: "upsertAnamnesis", Summary: "Create or update the anamnesis and medical alerts"},
	},

	// clinical-agenda · odontogram and periodontogram.
	"/api/v1/patients/{id}/odontogram": {
		{Method: "GET", OperationID: "getOdontogram", Summary: "Read the 2D odontogram"},
	},
	"/api/v1/patients/{id}/odontogram/findings": {
		{Method: "POST", OperationID: "recordOdontogramFinding", Summary: "Record a clinical finding on the odontogram"},
	},
	"/api/v1/patients/{id}/odontogram/findings/{findingId}": {
		{Method: "PUT", OperationID: "updateOdontogramFinding", Summary: "Update a clinical finding"},
		{Method: "DELETE", OperationID: "deleteOdontogramFinding", Summary: "Delete a clinical finding before its lock window"},
	},
	"/api/v1/patients/{id}/odontogram/findings/{findingId}/addendum": {
		{Method: "POST", OperationID: "addOdontogramFindingAddendum", Summary: "Add a signed addendum to a locked finding"},
	},
	"/api/v1/patients/{id}/periodontograms": {
		{Method: "GET", OperationID: "listPeriodontograms", Summary: "List the digital periodontograms"},
		{Method: "POST", OperationID: "recordPeriodontogram", Summary: "Record a digital periodontogram"},
	},

	// clinical-agenda · SOEP evolution and informed consent.
	"/api/v1/patients/{id}/soep-notes": {
		{Method: "GET", OperationID: "listSOEPNotes", Summary: "List the SOEP evolution notes"},
		{Method: "POST", OperationID: "createSOEPNote", Summary: "Create a SOEP evolution note"},
	},
	"/api/v1/patients/{id}/soep-notes/{noteId}": {
		{Method: "PUT", OperationID: "updateSOEPNote", Summary: "Update a SOEP note before its lock window"},
		{Method: "DELETE", OperationID: "deleteSOEPNote", Summary: "Delete a SOEP note before its lock window"},
	},
	"/api/v1/patients/{id}/soep-notes/{noteId}/addenda": {
		{Method: "POST", OperationID: "createSOEPAddendum", Summary: "Add a signed addendum to a locked SOEP note"},
	},
	"/api/v1/patients/{id}/consents": {
		{Method: "GET", OperationID: "listPatientConsents", Summary: "List the consents of a patient"},
		{Method: "POST", OperationID: "createConsent", Summary: "Create a digital informed consent"},
	},
	"/api/v1/patients/{id}/consents/{consentId}/sign": {
		{Method: "POST", OperationID: "signConsent", Summary: "Sign a digital informed consent"},
	},

	// clinical-agenda · treatment plans and budgets.
	"/api/v1/patients/{id}/treatment-plans": {
		{Method: "GET", OperationID: "listPatientTreatmentPlans", Summary: "List the treatment plans of a patient"},
		{Method: "POST", OperationID: "createTreatmentPlan", Summary: "Create a multi-phase treatment plan and budget"},
	},
	"/api/v1/patients/{id}/schedulable-tasks": {
		{Method: "GET", OperationID: "listSchedulableTasks", Summary: "List the approved plan items ready to be scheduled"},
	},
	"/api/v1/treatment-plans": {
		{Method: "GET", OperationID: "listTreatmentPlans", Summary: "List the treatment plans"},
	},
	"/api/v1/treatment-plans/{planId}/approve": {
		{Method: "POST", OperationID: "approveTreatmentPlan", Summary: "Approve a treatment plan and its budget"},
	},
	"/api/v1/treatment-plans/{planId}/items/{itemId}/complete": {
		{Method: "POST", OperationID: "completeTreatmentPlanItem", Summary: "Complete an item of a treatment plan"},
	},
	"/api/v1/treatment-plans/{planId}/items/{itemId}": {
		{Method: "DELETE", OperationID: "deleteTreatmentPlanItem", Summary: "Delete an item of a treatment plan"},
	},
	"/api/v1/consents": {
		{Method: "GET", OperationID: "listConsents", Summary: "List the digital informed consents"},
	},

	// clinical-agenda · scheduling and audit.
	"/api/v1/scheduling/appointments": {
		{Method: "GET", OperationID: "listAppointments", Summary: "Query the multi-chair calendar"},
		{Method: "POST", OperationID: "createAppointment", Summary: "Schedule an appointment"},
	},
	"/api/v1/scheduling/appointments/{id}/reschedule": {
		{Method: "PATCH", OperationID: "rescheduleAppointment", Summary: "Reschedule an appointment"},
	},
	"/api/v1/scheduling/appointments/{id}/transition": {
		{Method: "POST", OperationID: "transitionAppointment", Summary: "Move an appointment to the next state"},
	},
	"/api/v1/scheduling/webhooks/patient-reply": {
		{Method: "POST", OperationID: "schedulingWebhookPatientReply", Summary: "Receive a patient reply webhook"},
	},
	"/api/v1/scheduling/webhooks/confirmation": {
		{Method: "POST", OperationID: "schedulingWebhookConfirmation", Summary: "Receive an appointment confirmation webhook"},
	},
	"/api/v1/audit/logs": {
		{Method: "GET", OperationID: "listAuditLogs", Summary: "Query the immutable clinical CRUD audit trail"},
	},

	// billing-stock · billing and blind cash close.
	"/api/v1/billing/invoices": {
		{Method: "GET", OperationID: "listInvoices", Summary: "List the electronic invoices and POS receipts"},
		{Method: "POST", OperationID: "issueInvoice", Summary: "Issue an electronic invoice or POS payment"},
	},
	"/api/v1/billing/cash-registers": {
		{Method: "GET", OperationID: "listCashRegisters", Summary: "List the cash registers"},
		{Method: "POST", OperationID: "openCashRegister", Summary: "Open a cash register"},
	},
	"/api/v1/billing/cash-registers/{id}": {
		{Method: "GET", OperationID: "getCashRegister", Summary: "Read a cash register"},
	},
	"/api/v1/billing/cash-registers/{id}/blind-close": {
		{Method: "POST", OperationID: "blindCloseCashRegister", Summary: "Close a cash register with a blind cash count"},
	},
	"/api/v1/billing/cash-registers/{id}/audit-report": {
		{Method: "GET", OperationID: "getCashRegisterAuditReport", Summary: "Read the cash close audit report"},
	},

	// billing-stock · settlements and commissions.
	"/api/v1/settlements": {
		{Method: "GET", OperationID: "listSettlements", Summary: "List the settlements of the professionals"},
	},
	"/api/v1/settlements/rules": {
		{Method: "GET", OperationID: "listCommissionRules", Summary: "List the commission rules"},
		{Method: "POST", OperationID: "configureCommissionRule", Summary: "Configure a commission rule"},
	},
	"/api/v1/settlements/generate": {
		{Method: "POST", OperationID: "generateSettlement", Summary: "Generate the settlements of a period"},
	},

	// billing-stock · clinical inventory.
	"/api/v1/inventory/items": {
		{Method: "GET", OperationID: "listInventoryItems", Summary: "List the clinical supplies"},
		{Method: "POST", OperationID: "upsertInventoryItem", Summary: "Register or update a clinical supply"},
	},
	"/api/v1/inventory/recipes": {
		{Method: "GET", OperationID: "listInventoryRecipes", Summary: "List the standard supply recipes"},
	},
	"/api/v1/inventory/recipes/{procedureCode}": {
		{Method: "PUT", OperationID: "saveInventoryRecipe", Summary: "Save the recipe of a procedure"},
	},
	"/api/v1/inventory/deductions/procedure-completed": {
		{Method: "POST", OperationID: "deductInventoryForProcedure", Summary: "Deduct the stock of a completed procedure"},
	},
}

// defaultOpenAPIJSON builds the contract published by the gateway from the
// routes the three services really expose.
func defaultOpenAPIJSON() string {
	paths := make(map[string]any, len(apiRoutes))
	for path, routes := range apiRoutes {
		operations := make(map[string]any, len(routes))
		for _, route := range routes {
			operations[strings.ToLower(route.Method)] = map[string]any{
				"operationId": route.OperationID,
				"summary":     route.Summary,
			}
		}
		paths[path] = operations
	}

	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "SIGO-Dental Edge API Gateway",
			"version": "1.0.0",
		},
		"paths": paths,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return `{"openapi":"3.0.3","info":{"title":"SIGO-Dental Edge API Gateway","version":"1.0.0"},"paths":{}}`
	}
	return string(b)
}

// buildOpenAPIJSONFromYAML converts an OpenAPI 3.0 YAML artifact into the JSON
// document served by /openapi.json. The artifact is served only when it is a
// valid OpenAPI 3.0 document that describes routes the platform really
// exposes; otherwise the gateway falls back to the built-in contract, so the
// edge never publishes a route that no service implements.
func buildOpenAPIJSONFromYAML(yamlRaw string) string {
	if strings.TrimSpace(yamlRaw) == "" {
		return defaultOpenAPIJSON()
	}

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(yamlRaw), &doc); err != nil {
		return defaultOpenAPIJSON()
	}
	if !isPublishableOpenAPI(doc) {
		return defaultOpenAPIJSON()
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return defaultOpenAPIJSON()
	}
	return string(b)
}

// isPublishableOpenAPI reports whether the document is an OpenAPI 3.0 contract
// whose operations all exist in the platform.
func isPublishableOpenAPI(doc map[string]any) bool {
	version, ok := doc["openapi"].(string)
	if !ok || !strings.HasPrefix(strings.TrimSpace(version), "3.0") {
		return false
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		return false
	}
	for path, item := range paths {
		operations, ok := item.(map[string]any)
		if !ok {
			return false
		}
		for method, operation := range operations {
			if !isHTTPMethod(method) {
				// Path-level members (parameters, summary, ...) are not operations.
				continue
			}
			if _, ok := operation.(map[string]any); !ok {
				return false
			}
			if !platformExposes(path, method) {
				return false
			}
		}
	}
	return true
}

// platformExposes reports whether the platform really serves the operation.
func platformExposes(path, method string) bool {
	normalized := normalizePathTemplate(path)
	for knownPath, routes := range apiRoutes {
		if normalizePathTemplate(knownPath) != normalized {
			continue
		}
		for _, route := range routes {
			if strings.EqualFold(route.Method, method) {
				return true
			}
		}
	}
	return false
}

// normalizePathTemplate replaces every {parameter} segment by a placeholder so
// that the same route matches regardless of the parameter name used by the
// document being validated.
func normalizePathTemplate(path string) string {
	var builder strings.Builder
	for i := 0; i < len(path); {
		if path[i] == '{' {
			end := strings.IndexByte(path[i:], '}')
			if end < 0 {
				builder.WriteString(path[i:])
				break
			}
			builder.WriteString("{}")
			i += end + 1
			continue
		}
		builder.WriteByte(path[i])
		i++
	}
	return builder.String()
}

// isHTTPMethod reports whether the key of a path item is an HTTP method.
func isHTTPMethod(key string) bool {
	switch strings.ToLower(key) {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	default:
		return false
	}
}
