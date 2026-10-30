# AGENTS.md — SIGO-Dental

Quick reference for any coding agent (pandorx Orchestrator, Planner, Implementer, Reviewer) working on the **SIGO-Dental** repository. Read this before touching anything.

---

## 1. What is SIGO-Dental

**SIGO-Dental** (*Sistema Integral de Gestión Odontológica y Expediente Clínico Digital*) is a clinical and administrative platform for dental clinics.
- **Reference Specification:** [`Documento de Requerimientos de Software - Sistema Odontológico.md`](./Documento%20de%20Requerimientos%20de%20Software%20-%20Sistema%20Odontol%C3%B3gico.md)
- **Project Context (8 stages of `/init`):** [`pdd/context/project-context.md`](./pdd/context/project-context.md)

---

## 2. Tech Stack & Topology

- **Topology (SRS §7):**
  - **Client Layer (`frontend/`):** Responsive Web Portal / PWA (Next.js App Router, TypeScript, Tailwind CSS) communicating over `HTTPS / WSS / JWT`.
  - **API Gateway / Ingress (`services/api-gateway/`):** Edge routing, TLS termination, JWT/MFA session validation (`RNF-05`), rate limiting, and request forwarding to downstream backend services.
  - **Servicio Clínico & Agenda (`services/clinical-agenda/`):** Go 1.24+ service owning the clinical and scheduling bounded contexts:
    - `identity`: RBAC (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`), MFA verification, 15-min idle session timeout (`RNF-05`).
    - `patients`: unified patient record (`RF-01`), anamnesis & high-visibility critical medical alerts (`RF-02`).
    - `clinical`: 2D interactive odontogram with FDI & Universal notation (`RF-03`), digital periodontogram (`RF-04`), immutable SOEP evolution notes locked after 24h (`RF-05`, `RN-01`), digital informed consent (`RF-06`).
    - `treatments`: multi-phase treatment plans & budgets (`RF-07`), partial/total approval & transition to schedulable tasks (`RF-08`).
    - `scheduling`: multi-chair/multi-specialist calendar (`RF-09`), appointment state machine (`RF-10`), dental chair overlap prevention (`RN-03`), 48h/24h automated notifications (`RF-11`).
    - `audit`: immutable CRUD audit trail for clinical records with ISO-8601 timestamp and source IP (`RNF-06`).
  - **Servicio Facturación / Stock (`services/billing-stock/`):** Go 1.24+ service owning the financial and inventory bounded contexts:
    - `billing`: POS & electronic invoicing (`RF-12`), blind cash close (`RN-02`).
    - `settlements`: configurable dentist fee/commission calculation (`RF-13`).
    - `inventory`: clinical supplies catalog and automatic stock deduction upon procedure completion (`RF-14`).
  - **Internal Service Architecture:** Both Go services follow Pattern B / Hexagonal (`internal/<context>/domain`, `internal/<context>/application`, `internal/<context>/adapters`), with ports declared alongside the consuming use case (`.port.go` next to `.service.go`).
- **Database & Storage:**
  - **Primary DB:** PostgreSQL (incremental versioned SQL migrations `00001_*.sql`, AES-256 at rest, TLS 1.3 in transit).
  - **Cache & Queues:** Redis (session/idle tracking, async notification workers).
  - **Object Storage:** S3-compatible blob storage (radiographs, photos, signed consent PDFs).
- **Frontend:** Next.js (App Router, TypeScript, Tailwind CSS)
  -Optimized for desktop ($1920 \times 1080$), laptops ($1366 \times 768$), and chairside tablets (iPad / Android $\ge 10.5''$) (`RNF-07`).

---

## 3. Interface Architecture & UX Invariants (`frontend/`)

1. **Platform Defaults & Navigation:**
   - Collapsible sidebar for primary peer modules (`Agenda`, `Pacientes`, `Caja e Inventario`, `Reportes`).
   - **Sheets / Drawers over page pushes** for short, focused tasks (scheduling an appointment, recording a payment, signing consent) to keep the user anchored in context.
2. **Progressive Disclosure in Clinical Record:**
   - **Sticky Patient Header:** Always displays critical medical alerts (`RF-02`, e.g., penicillin allergy, anticoagulants) as high-visibility semantic badges.
   - **Scoped Clinical Workspaces:** Separate tabs for `Odontograma 2D`, `Periodontograma`, `Evolución SOEP`, and `Presupuestos` to prevent cognitive overload.
   - Collapse completed treatment phases to emphasize *what's next*.
3. **Visual Language:**
   - **Guiding Attributes:** *Trust, Clinical Precision, Calm*.
   - **Surface & Color:** Cool off-white background (`slate-50`) so pure white (`#FFFFFF`) signals the active clinical canvas (odontogram/chart). Reserve high-saturation semantic colors strictly for clinical findings (`RF-03`: red = caries, blue = existing restorations) and critical medical alerts (`RF-02`).
   - **Typography:** System/geometric sans for UI clarity; tabular numerals (`tabular-nums`) for periodontal probing depths (`RF-04`), tooth numbers (FDI/Universal), and financial/cash-close tables.

---

## 4. Non-Negotiable Business & Security Rules

- **RN-01 (Clinical History Immutability):** No procedure marked as `"Realizado"` or SOEP note past its lock window (24h default) can be deleted or mutated. Corrections require a signed addendum note.
- **RN-02 (Blind Cash Close):** Receptionists must NEVER receive or see the system's theoretical cash total before submitting the physical cash count during daily close.
- **RN-03 (Dental Chair Overlap Prevention):** Never allow two simultaneous appointments on the same physical dental chair (`sillón`), regardless of whether the dentists are different.
- **RNF-01 (Performance):** Clinical record load + odontogram render must complete in $< 1.5\text{ s}$ on $\ge 15\text{ Mbps}$.
- **RNF-06 (Audit Trail):** Every CRUD action on clinical records must append an immutable audit log entry (`actor_id`, `action`, `resource_id`, `timestamp_iso8601`, `source_ip`).

---

## 5. Canonical Commands

```bash
# API Gateway / Ingress (Go)
cd services/api-gateway && go test ./...
cd services/api-gateway && go vet ./...
cd services/api-gateway && go build -o bin/api-gateway ./cmd/gateway

# Servicio Clínico & Agenda (Go)
cd services/clinical-agenda && go test ./...
cd services/clinical-agenda && go vet ./...
cd services/clinical-agenda && go build -o bin/clinical-agenda ./cmd/api

# Servicio Facturación / Stock (Go)
cd services/billing-stock && go test ./...
cd services/billing-stock && go vet ./...
cd services/billing-stock && go build -o bin/billing-stock ./cmd/api

# Frontend (Next.js)
cd frontend && pnpm test
cd frontend && pnpm lint
cd frontend && pnpm build
```

---

## 6. How to Ship Work (PDD — Plan-Driven Development)

1. **No production code without a human-approved plan.**
2. **Feature Spec:** `pdd/tasks/<feature>.md` with numbered specs (`S1..Sn`).
3. **Work Item Plan:** `pdd/<work-item>/plan.md` with all 7 sections (`docs/pdd/roles/planner.md` §4). Wait for explicit human approval.
4. **Implementation (`apply`):** TDD (RED → GREEN → refactor). Record executed commands and outputs in `pdd/<work-item>/apply.md`.
5. **Review (`review`):** 4-lens audit recorded in `pdd/<work-item>/review.md` citing `file:line`. Bounded correction (`MaxCorrectionChangedLines = 200`).
6. **Context by reference:** Pass file paths (`pdd/...`) between roles, never paste full file contents into prompts.
