# Contexto del proyecto — SIGO-Dental

Las ocho etapas de `init` resueltas, con su procedencia. Este archivo es el
artefacto legible de [`pdd/context/project-context.md`](./project-context.md);
la copia que el agente lee vive en `.pandorx/pandorx.db` y su espejo en
`.pandorx/MEMORIES.md`.

| Procedencia | Significado |
|---|---|
| `user` | La persona lo define o confirma como decisión de negocio/producto (`SRS / BRD v1.0` + entrevista `/init`). |
| `confirmed` | Propuesto a partir de la especificación técnica del proyecto y confirmado por la persona. |
| `derived` | Leído directamente de la evidencia del repositorio sin confirmación humana. |

---

## Etapa 1 — Naturaleza y dominio del sistema (`system_nature`)

- **Pregunta:** What type of system are you building?
- **Respuesta:** **C) Full-stack application (backend + user-facing interface).**
- **Procedencia:** `user`
- **Detalle acordado (Topología SRS §7):**
  - **Capa Cliente (`frontend/`):** Web App / PWA en Next.js (App Router, TypeScript, Tailwind CSS) comunicándose vía `HTTPS / WSS / JWT`, diseñada bajo principios de *Interface Architecture* para operación clínica en sillón odontológico (tablets $\ge 10.5''$, laptops y escritorio).
  - **API Gateway / Ingress (`services/api-gateway/`):** Puerta de entrada única que gestiona ruteo, validación de tokens JWT/MFA, control de inactividad (15 min) y reenvío hacia los servicios de dominio.
  - **Servicio Clínico & Agenda (`services/clinical-agenda/`):** Microservicio / servicio modular en Go con arquitectura hexagonal (`internal/<context>/domain`, `application`, `adapters`) para Pacientes, Anamnesis, Odontograma 2D, Periodontograma, Evolución SOEP, Consentimientos, Presupuestos, Agenda Multirrecinto y Auditoría Clínica.
  - **Servicio Facturación / Stock (`services/billing-stock/`):** Microservicio / servicio modular en Go con arquitectura hexagonal (`internal/<context>/domain`, `application`, `adapters`) para Punto de Venta, Facturación Electrónica, Cierre de Caja Ciego (`RN-02`), Liquidación de Honorarios (`RF-13`) y Descargo Automático de Insumos (`RF-14`).

## Etapa 2 — Alcance y límites del MVP (`scope_mvp`)

- **Pregunta:** What is the scope of this first version?
- **Respuesta:** **D) Enterprise / institutional system with compliance requirements.**
- **Procedencia:** `user`
- **Follow-up (`scope_mvp.features`):**
  - **Dentro del alcance (Fase 1 — Core Administrativo y Turnos, Sprints 1–4):**
    1. Gestión de pacientes, ficha única y alertas médicas de alta visibilidad (`RF-01`, `RF-02`).
    2. Agenda multirrecinto por sillón/especialista, máquina de estados de cita y bloqueo de sobreposición de sillón (`RF-09`, `RF-10`, `RN-03`).
    3. Catálogo médico, planes de tratamiento/presupuestos multinivel (`RF-07`, `RF-08`).
    4. Punto de venta, facturación electrónica y cierre de caja ciego (`RF-12`, `RN-02`).
    5. Seguimiento en Fases 2 y 3: Odontograma 2D interactivo FDI/Universal (`RF-03`), Periodontograma (`RF-04`), Notas SOEP inmutables (`RF-05`, `RN-01`), Consentimiento digital (`RF-06`), Liquidación de honorarios (`RF-13`) y Descargo de insumos (`RF-14`).
  - **Fuera del alcance en Fase 1:**
    1. App móvil nativa iOS/Android para pacientes (se abordará vía PWA en Fase 2).
    2. Procesamiento directo de imágenes 3D DICOM (se usará almacenamiento de archivos planos / PACS externo en esta etapa).

## Etapa 3 — Concurrencia, carga y performance (`concurrency_load`)

- **Pregunta:** What is the expected usage volume and performance profile?
- **Respuesta:** **B) Moderate concurrency (< 1,000 req/min), standard web traffic.**
- **Procedencia:** `user`
- **Restricciones de SLA y latencia (`RNF-01`, `RNF-02`, `RNF-03`, `RF-11`):**
  - Tiempo de carga de expediente clínico y renderizado de odontograma $< 1.5\text{ segundos}$ en conexiones $\ge 15\text{ Mbps}$.
  - Disponibilidad (SLA) $\ge 99.9\%$ en horario operativo clínico (07:00 a 21:00 hora local).
  - Soporte base para 150 usuarios concurrentes sin degradación de latencia en Fase 1.
  - Procesamiento asíncrono con Redis/workers para recordatorios multicanal 48h y 24h antes del turno (WhatsApp Business / SMS / Email).

## Etapa 4 — Arquitectura de datos y persistencia (`data_architecture`)

- **Pregunta:** How will the system store and manage data?
- **Respuesta:** **A) Relational database (PostgreSQL, MySQL, SQLite).**
- **Procedencia:** `user`
- **Follow-up (`data_architecture.orm`):** **C) Other (specify):** `PostgreSQL` como base transaccional con consultas tipadas (`sqlc` / `pgx`) y migraciones SQL incrementales por caso de uso (`00001_*.sql`), acompañado de `Redis` para caché/mensajería y `S3 / Blob Storage` para radiografías, fotos y PDFs firmados.

## Etapa 5 — Autenticación y seguridad (`auth_security`)

- **Pregunta:** Does the system require user authentication?
- **Respuesta:** **E) Multi-tenant with role-based access control (RBAC/ABAC).**
- **Procedencia:** `user`
- **Follow-up (`auth_security.compliance`):** **C) HIPAA (healthcare data)** + Ley de Protección de Datos Personales y normativas sanitarias locales.
- **Controles obligatorios (`RNF-04`, `RNF-05`):**
  - 5 perfiles RBAC: `Administrador TI`, `Recepcionista / Cajero`, `Odontólogo General / Especialista`, `Asistente Dental`, `Auditor / Contador`.
  - Autenticación Multifactor (MFA) obligatoria para acceder a expedientes clínicos e información financiera.
  - Cierre automático de sesión tras 15 minutos de inactividad en estaciones clínicas.
  - Cifrado TLS 1.3 en tránsito y AES-256 en reposo para base de datos y blobs.

## Etapa 6 — Estrategia de testing y calidad (`testing_strategy`)

- **Pregunta:** What testing approach do you want from the start?
- **Respuesta:** **B) Unit + integration tests with CI automation.**
- **Procedencia:** `user`
- **Criterios de aceptación técnica (`SRS §8.2`):**
  - Cobertura mínima del $80\%$ en pruebas unitarias y de integración sobre endpoints críticos del backend.
  - Escaneo de seguridad SAST/DAST sin vulnerabilidades de severidad Alta o Crítica (OWASP Top 10).
  - Disciplina TDD en PDD: tests unitarios y de aceptación de fase escritos en RED por el `Implementer`; casos adversariales y de borde auditados por el `Reviewer`.

## Etapa 7 — Infraestructura y despliegue (`infrastructure_deployment`)

- **Pregunta:** How will this system be deployed and run?
- **Respuesta:** **C) Container orchestration (Docker Compose / Podman Quadlet).**
- **Procedencia:** `user`
- **Follow-up (`infrastructure_deployment.cicd`):** **A) Yes, GitHub Actions.**
- **Entregables asociados:** Contenedores reproducibles para `frontend` (Next.js), `api-gateway` (Go/Ingress), `clinical-agenda` (Go), `billing-stock` (Go), `postgres`, `redis` y `rustfs/s3` local, junto con especificación OpenAPI / Swagger v3.0.

## Etapa 8 — Observabilidad y monitoreo (`observability`)

- **Pregunta:** How will you monitor the system in production?
- **Respuesta:** **D) Full observability: logs + metrics + traces (OpenTelemetry).**
- **Procedencia:** `user`
- **Requerimiento normativo (`RNF-06`, `RN-01`):**
  - Bitácora inmutable de auditoría para toda operación CRUD sobre historias clínicas (quién consultó, modificó o exportó qué registro, con timestamp `ISO-8601` y dirección IP de origen).
  - Telemetría de tiempos de respuesta para garantizar el umbral de $< 1.5\text{ s}$ en expediente y odontograma.

---

## Arquitectura de Interfaz Acordada (`/interface-architecture`)

1. **Separación de Preocupaciones y Fidelidad:**
   - Estructura de navegación tipo *App Shell* clínica: Sidebar izquierda colapsable para destinos principales y paneles deslizables (*Sheets*) para acciones transaccionales rápidas (agendar cita, registrar cobro, firmar consentimiento) manteniendo siempre el contexto.
2. **Arquitectura de Información y Divulgación Progresiva:**
   - **Nivel 1 (Cabecera Fija del Paciente):** Identidad, edad, convenio y *badges* críticos de anamnesis (`RF-02`) siempre visibles.
   - **Nivel 2 (Workspaces Clínicos por Pestaña):** `Odontograma 2D`, `Periodontograma`, `Evolución SOEP`, `Planes y Presupuestos`, `Consentimientos`.
   - **Nivel 3 (Detalle / Acción):** Selección de diente/superficie en el odontograma o fase del presupuesto. Las fases completadas se colapsan automáticamente para destacar el siguiente paso clínico.
3. **Lenguaje Visual:**
   - **Atributos Guía:** *Confianza, Precisión Clínica, Calma*.
   - **Paleta y Profundidad:** Superficie base neutra fría (`slate-50`) con tarjetas blancas puras (`#FFFFFF`) delimitadas por bordes sutiles (`1px solid slate-200`) para destacar el área de trabajo activa sin fatiga visual en consultorio. Colores saturados reservados exclusivamente para el catálogo estándar del odontograma (`RF-03`: rojo caries, azul restauración) y alertas médicas (`RF-02`).
   - **Tipografía:** Sans-serif del sistema/geométrica con `tabular-nums` obligatorio en sondaje periodontal (6 puntos por diente), numeración dental FDI/Universal y montos de caja.
