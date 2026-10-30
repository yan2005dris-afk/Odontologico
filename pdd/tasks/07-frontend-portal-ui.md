# Feature 07 — Frontend Portal & Chairside Clinical Workspace (Next.js App Router)

- **Feature ID:** `07-frontend-portal-ui`
- **SRS References:** `RF-01`..`RF-14`, `RN-01`..`RN-03`, `RNF-01`, `RNF-05`, `RNF-07`, `RNF-08`, Interface Architecture (`AGENTS.md §3`)
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-10-frontend-shell-patients-agenda`](../wi-10-frontend-shell-patients-agenda/plan.md) (`wt/frontend` — App Shell, Auth/MFA/Idle Timeout, Patients `RF-01`/`RF-02`, Multi-chair Agenda `RF-09`/`RF-10`/`RN-03`)
  - [`wi-11-frontend-clinical-workspace`](../wi-11-frontend-clinical-workspace/plan.md) (`wt/frontend` — 2D Odontogram `RF-03`, Periodontogram `RF-04`, SOEP `RF-05`/`RN-01`, Consents `RF-06`, Budgets `RF-07`/`RF-08`)
  - [`wi-12-frontend-billing-inventory`](../wi-12-frontend-billing-inventory/plan.md) (`wt/frontend` — Mixed-Payment POS `RF-12`, Blind Cash Close `RN-02`, Settlements `RF-13`, Inventory `RF-14`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Next.js App Router (TypeScript, Tailwind CSS) responsive web application and PWA shell optimized for desktop ($1920 \times 1080$), laptop ($1366 \times 768$), and chairside tablet ($\ge 10.5''$ iPad/Android) (`RNF-07`, `RNF-08`).
- Interface Architecture Invariants (`AGENTS.md §3`):
  - **App Shell & Navigation:** Collapsible left sidebar for primary peer modules (`Agenda`, `Pacientes`, `Caja e Inventario`, `Reportes`) and contextual **Slide-over Sheets / Drawers** for focused transactional flows (scheduling an appointment, recording a payment, signing digital consent) without losing clinical context.
  - **Progressive Disclosure in Clinical Record:**
    - *Level 1 (Sticky Patient Header):* Always-visible patient identity, age, insurance/convenio, and high-visibility semantic alert badges (`RF-02`: e.g., Penicillin Allergy, Anticoagulants).
    - *Level 2 (Scoped Clinical Tabs):* Separate tabs for `Odontograma 2D`, `Periodontograma`, `Evolución SOEP`, `Presupuestos`, and `Consentimientos`.
    - *Level 3 (Focused Detail):* Tooth/surface inspector and collapsible completed treatment phases to highlight *what's next*.
  - **Visual Language & Typography:** Cool off-white background (`bg-slate-50`) with pure white (`#FFFFFF`) active clinical canvases and `1px solid border-slate-200`; high-saturation colors strictly reserved for `RF-03` clinical findings (`#DC2626` red = caries, `#2563EB` blue = existing restorations) and `RF-02` critical medical alerts; mandatory `tabular-nums` on all 6-point periodontal probing inputs (`RF-04`), FDI/Universal tooth numbers, and POS/cash-close financial tables.
- Client-side 15-minute idle activity watcher (`RNF-05`) that locks/terminates the clinical session on inactivity.

### Out of Scope
- Native iOS/Android mobile apps for patients and direct 3D DICOM rendering (explicitly out of Phase 1 scope per `SRS §2.3`).

---

## 2. Numbered Specifications (`S1..S9`)

- **S1 — Clinical App Shell, Collapsible Sidebar & 15-Min Idle Session Guard (`RNF-05`, `RNF-07`):** Responsive layout (`bg-slate-50`), role-filtered collapsible sidebar, MFA login challenge UI, and client+server 15-minute inactivity timer that auto-locks the workstation.
- **S2 — Unified Patient Directory, Minor Guardian Validation & Sticky Alert Header (`RF-01`, `RF-02`):** Patient creation/edit form enforcing legal guardian fields when age $< 18$, and sticky patient header rendering high-visibility `RF-02` critical medical alert badges across all clinical tabs.
- **S3 — Multi-Chair / Multi-Specialist Calendar & Slide-Over Booking Sheet (`RF-09`, `RF-10`, `RN-03`):** Day/week calendar grid filterable by physical dental chair (`sillón`) and specialist, real-time chair overlap error feedback (`RN-03`), and 7-state appointment status badge & transition drawer (`RF-10`).
- **S4 — Interactive 2D SVG Odontogram with FDI/Universal Toggle (`RF-03`, `RNF-01`, `RN-01`):** Pure-white (`#FFFFFF`) vector canvas rendering 32 permanent and 20 deciduous teeth with clickable 5-surface polygons (`M`, `D`, `O/I`, `V`, `L/P`), instant FDI $\leftrightarrow$ Universal notation switch (`tabular-nums`), standardized color catalog (red caries, blue restorations, endodontics, implants, prostheses, extractions, sealants), and read-only lock on `"Realizado"` findings (`RN-01`). Initial load + render $< 1.5\text{ s}$ (`RNF-01`).
- **S5 — 6-Point Digital Periodontogram Grid (`RF-04`, `RNF-07`):** Chairside keyboard/touch-friendly 6-point per tooth table (`MB, B, DB, ML, L, DL`) using `tabular-nums`, live client/server calculation of Clinical Attachment Level ($\text{NIC} = \text{PD} - \text{GM}$), furcation, mobility, and Bleeding on Probing (`BOP`) toggles.
- **S6 — Chronological SOEP Evolution Notes & Addendum Drawer (`RF-05`, `RN-01`):** Structured `S-O-E-P` editor with countdown to the 24h immutability lock, read-only badge once locked, and slide-over sheet to append a cryptographically signed Addendum note (`RN-01`).
- **S7 — Touch Biometric / OTP Informed Consent Sheet (`RF-06`):** Slide-over drawer presenting the procedure's legal consent text with an HTML5 Canvas touch signature pad (for $\ge 10.5''$ chairside tablets) and OTP verification fallback.
- **S8 — Multi-Phase Treatment Builder & Partial Approval (`RF-07`, `RF-08`):** Multi-phase budget workspace where completed phases auto-collapse, allowing partial or total item selection and single-click conversion of approved items into schedulable tasks.
- **S9 — Mixed-Payment POS, Blind Cash Close (`RN-02`), Dentist Settlements & Inventory UI (`RF-12`, `RF-13`, `RF-14`):** POS checkout drawer supporting mixed payment tenders, Blind Cash Close screen that strictly hides theoretical system totals until the receptionist submits the physical cash count (`RN-02`), dentist commission settlement views (`RF-13`), and consultorio stock + procedure recipe management (`RF-14`).

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-10-frontend-shell-patients-agenda` | `wt/frontend` | `frontend/src/app/(auth)/**`, `frontend/src/app/(portal)/layout.tsx`, `frontend/src/app/(portal)/patients/**`, `frontend/src/app/(portal)/agenda/**`, `frontend/src/components/shell/**`, `frontend/src/components/patients/**`, `frontend/src/components/agenda/**`, `frontend/src/lib/**` | `wi-01`, `wi-02`, `wi-03`, `wi-07` (API contracts) |
| `wi-11-frontend-clinical-workspace` | `wt/frontend` | `frontend/src/app/(portal)/patients/[id]/clinical/**`, `frontend/src/components/clinical/**`, `frontend/src/components/treatments/**` | `wi-10-frontend-shell-patients-agenda` |
| `wi-12-frontend-billing-inventory` | `wt/frontend` | `frontend/src/app/(portal)/billing/**`, `frontend/src/app/(portal)/inventory/**`, `frontend/src/app/(portal)/settlements/**`, `frontend/src/components/billing/**`, `frontend/src/components/inventory/**` | `wi-10-frontend-shell-patients-agenda` |
