# Apply Report — `wi-01-gateway-ingress`

- **Work Item:** `wi-01-gateway-ingress`
- **Parent Feature:** [`pdd/tasks/01-platform-gateway-auth.md`](../tasks/01-platform-gateway-auth.md)
- **Plan Reference:** [`pdd/wi-01-gateway-ingress/plan.md`](./plan.md)
- **Worktree / Branch:** `wt/gateway`
- **Status:** `DONE`

---

## 1. Spec Coverage (`S1..S6`)

| Spec | Requirement Summary | Verification & Test Coverage | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Docker Compose multi-service topology (`postgres`, `redis`, `minio`, `api-gateway`, `clinical-agenda`, `billing-stock`, `frontend`) with healthchecks, isolated internal network, and TLS 1.3 (`SRS §7`, `RNF-02`, `RNF-04`). | `deploy/compose.yaml`, `httpadapter.NewTLS13Server`, verified by `TestSpecS1_ComposeTopologyAndTLS13`. | Covered |
| **S2** | Context-aware Reverse Proxy routing to `clinical-agenda` and `billing-stock` preserving method/query and injecting verified `X-Request-ID`, `X-Actor-ID`, `X-Actor-Role`, `X-Session-ID`, `X-MFA-Verified`, `X-Source-IP` while stripping spoofed client headers (`SRS §7`, `RNF-06`). | `application.ResolveUpstream`, `httpadapter.ProxyRouter`, verified by `TestResolveUpstreamContext` and `TestProxyRouter_SpecsS2ToS6/S2`. | Covered |
| **S3** | 15-minute (`900s`) sliding idle session timeout in Redis (`session:idle:<session_id>`); expired/missing sessions return `401 Unauthorized` (`SESSION_IDLE_TIMEOUT`) (`RNF-05`). | `redis.InMemorySessionStore`, `redis.RedisSessionStore`, `application.GatewayService.AuthorizeRequest`, verified by `TestGatewayService_AuthorizeRequest/S3`, `TestProxyRouter_SpecsS2ToS6/S3`, and `TestRedisSessionStore_RESPProtocol`. | Covered |
| **S4** | Mandatory `mfa_verified == true` enforcement on `/api/v1/clinical/*`, `/api/v1/patients/*/anamnesis`, `/api/v1/treatments/*`, `/api/v1/billing/*`, and `/api/v1/settlements/*` returning `403 Forbidden` (`MFA_REQUIRED`) (`RNF-05`). | `domain.RequiresMFA`, `application.GatewayService.AuthorizeRequest`, verified by `TestRequiresMFA`, `TestGatewayService_AuthorizeRequest/S4`, and `TestProxyRouter_SpecsS2ToS6/S4`. | Covered |
| **S5** | 5-role edge RBAC matrix (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`) returning `403 Forbidden` (`INSUFFICIENT_ROLE_PERMISSION`) (`SRS §3`, `RNF-05`). | `domain.RBACPolicy.IsAllowed`, verified by `TestRBACPolicy_IsAllowed`, `TestGatewayService_AuthorizeRequest/S5`, and `TestProxyRouter_SpecsS2ToS6/S5`. | Covered |
| **S6** | Observability endpoints (`GET /healthz`, `GET /metrics` exposing `gateway_request_duration_ms` and `< 1500ms` SLA counters, and `GET /openapi.json` serving OpenAPI 3.0.3) (`RNF-01`, `SRS §8.2`). | `httpadapter.LatencyMetricsCollector`, `openapi/sigo-dental.v1.yaml`, verified by `TestProxyRouter_SpecsS2ToS6/S6`. | Covered |

---

## 2. Files Created / Modified

- `deploy/compose.yaml`
- `deploy/.env.example`
- `openapi/sigo-dental.v1.yaml`
- `services/api-gateway/go.mod`
- `services/api-gateway/cmd/gateway/main.go`
- `services/api-gateway/internal/ingress/domain/claims.go`
- `services/api-gateway/internal/ingress/domain/rbac_policy.go`
- `services/api-gateway/internal/ingress/domain/rbac_policy_test.go`
- `services/api-gateway/internal/ingress/application/gateway.port.go`
- `services/api-gateway/internal/ingress/application/gateway.service.go`
- `services/api-gateway/internal/ingress/application/gateway.service_test.go`
- `services/api-gateway/internal/ingress/adapters/redis/session_store.go`
- `services/api-gateway/internal/ingress/adapters/http/middleware.go`
- `services/api-gateway/internal/ingress/adapters/http/proxy_router.go`
- `services/api-gateway/internal/ingress/adapters/http/proxy_router_test.go`

---

## 3. Executed Commands & Observed Output

```bash
cd services/api-gateway && go test -v -cover ./... && go vet ./... && go build -o bin/api-gateway ./cmd/gateway
```

```text
=== RUN   TestSpecS1_ComposeTopologyAndTLS13
--- PASS: TestSpecS1_ComposeTopologyAndTLS13 (0.00s)
=== RUN   TestProxyRouter_SpecsS2ToS6
=== RUN   TestProxyRouter_SpecsS2ToS6/S2:_routes_clinical_and_billing_contexts_and_strips_spoofed_X-Actor_headers
=== RUN   TestProxyRouter_SpecsS2ToS6/S3:_session_idle_>_900s_returns_401_SESSION_IDLE_TIMEOUT
=== RUN   TestProxyRouter_SpecsS2ToS6/S4:_mfa_verified=false_on_clinical/financial_routes_returns_403_MFA_REQUIRED
=== RUN   TestProxyRouter_SpecsS2ToS6/S5:_receptionist_on_odontograms_and_assistant_on_billing_return_403_INSUFFICIENT_ROLE_PERMISSION
=== RUN   TestProxyRouter_SpecsS2ToS6/S6:_healthz,_metrics_(gateway_request_duration_ms),_and_openapi.json
--- PASS: TestProxyRouter_SpecsS2ToS6 (0.00s)
=== RUN   TestRedisSessionStore_RESPProtocol
--- PASS: TestRedisSessionStore_RESPProtocol (0.00s)
=== RUN   TestEdgeCases_OptionsLoginAndRateLimiter
--- PASS: TestEdgeCases_OptionsLoginAndRateLimiter (0.00s)
PASS
coverage: 89.7% of statements
ok  	github.com/sigo-dental/api-gateway/internal/ingress/adapters/http	0.010s	coverage: 89.7% of statements
=== RUN   TestGatewayService_AuthorizeRequest
--- PASS: TestGatewayService_AuthorizeRequest (0.00s)
=== RUN   TestResolveUpstreamContext
--- PASS: TestResolveUpstreamContext (0.00s)
PASS
coverage: 85.7% of statements
ok  	github.com/sigo-dental/api-gateway/internal/ingress/application	0.004s	coverage: 85.7% of statements
=== RUN   TestClaimsValidation
--- PASS: TestClaimsValidation (0.00s)
=== RUN   TestRequiresMFA
--- PASS: TestRequiresMFA (0.00s)
=== RUN   TestRBACPolicy_IsAllowed
--- PASS: TestRBACPolicy_IsAllowed (0.00s)
=== RUN   TestHS256Verifier_VerifyToken
--- PASS: TestHS256Verifier_VerifyToken (0.00s)
PASS
coverage: 90.4% of statements
ok  	github.com/sigo-dental/api-gateway/internal/ingress/domain	0.004s	coverage: 90.4% of statements
```

---

## 4. Architectural Decisions & Boundaries Respected

- **Hexagonal Pattern B:** Domain entities/policies (`domain/claims.go`, `domain/rbac_policy.go`) have zero outer dependencies; application ports (`application/gateway.port.go`) sit next to the orchestrating service (`application/gateway.service.go`); HTTP and Redis adapters (`adapters/http`, `adapters/redis`) implement the driven/driving ports.
- **Zero External Dependency Bloat:** Implemented constant-time HMAC-SHA256 JWT verification (`crypto/hmac`, `crypto/sha256`), Redis RESP `EXPIRE session:idle:<sid> 900` protocol client (`net`), token-bucket rate limiting, and reverse proxying (`net/http/httputil`) using the Go 1.24 standard library for deterministic offline execution.
- **Scope Boundaries:** Only files listed in Section 7 of `pdd/wi-01-gateway-ingress/plan.md` plus `pdd/wi-01-gateway-ingress/apply.md` were created or modified.
