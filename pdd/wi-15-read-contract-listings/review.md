# Review — `wi-15-read-contract-listings`

Revisión 4-lentes sobre el cambio de `wi-15` contra la base `c865d15`.
Lentes: `review-risk`, `review-resilience`, `review-readability`, `review-reliability`.
Alcance: 60 archivos bajo `services/`, +5 242 / −147.
Estado: **aprobado**. Los 5 defectos que blocking (4 del plan + 1 de `RN-02` encontrado en la
revisión) están corregidos y verificados; las 5 observaciones abiertas de §5 no invalidan el
work item.

---

## 1. Lente `review-risk` — superficie de riesgo y cumplimiento normativo

### 1.1 La matriz RBAC y el rol `auditor` — hallazgo cerrado

La pregunta que `wi-14` dejó abierta ("¿debe el `auditor` leer las fichas clínicas?") tiene
respuesta documental: el **SRS §3** define al rol como «**Auditor / Contador**», con permisos de
*conciliación financiera e inventarios* (cortes de caja, liquidación de honorarios, inventario) y
**no** de historia clínica. El usuario confirmó esa lectura y el plan lo fijó.

La matriz final (`services/api-gateway/internal/ingress/domain/rbac_policy.go:166`, `:181`,
`:192`):

| Ruta | admin | dentist | receptionist | assistant | auditor |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `/api/v1/users` | pleno | — | — | — | — |
| `/api/v1/treatment-plans` | pleno | pleno | sólo GET | sólo GET | — |
| `/api/v1/patients/{id}/treatment-plans` | pleno | pleno | sólo GET | sólo GET | — |
| `/api/v1/consents` | pleno | pleno | pleno | sólo GET | — |
| `/api/v1/patients/{id}/consents` | pleno | pleno | pleno | sólo GET | — |
| `/api/v1/patients` (sin cambios) | pleno | pleno | pleno | sólo GET | — (403) |

Verificado end-to-end contra el stack real: el `auditor` recibe **403** en `/api/v1/patients`,
`/api/v1/consents`, `/api/v1/treatment-plans` y `/api/v1/users`, y **200** en `/api/v1/audit/logs`,
`/api/v1/settlements` e `/api/v1/inventory/items`. El `dentist` recibe **403** en `/api/v1/users`.

### 1.2 El solapamiento de prefijos — hallazgo cerrado

El riesgo real de este diseño no era la matriz, sino el **solapamiento de prefijos**: el caso
`/api/v1/patients` (`rbac_policy.go:203`) captura por `HasPrefix` todo lo que empiece por
`/api/v1/patients/`, y las dos rutas nuevas son precisamente
`/api/v1/patients/{id}/treatment-plans` y `/api/v1/patients/{id}/consents`. Sin resolverlo, un
`assistant` habría podido **crear** un plan (sólo lectura) o un `auditor` habría leído
consentimientos clínicos.

La resolución es explícita: `patientSubResource` (`rbac_policy.go:118`) extrae el sub-recurso y los
casos de planes y consentimientos (`rbac_policy.go:181`, `:192`) se evalúan **antes** que la
política de fichas clínicas. Fijado con tests en `rbac_policy_test.go`.

### 1.3 `RN-02` — el hallazgo más grave del work item, cerrado

La primera implementación del listado de cajas exponía `theoretical_cents` y `discrepancy_cents`
a **cualquier rol** en cuanto la caja estaba `CLOSED`. Tres cosas lo hacen inaceptable:

1. Contradice el contrato **preexistente** del dominio: `GetAuditReport`
   (`billing.service.go:315`) restringe la conciliación a `auditor`/`admin`, y el propio comentario
   de `CashRegisterAuditReportView` lo dice («accessible exclusively to auditor and admin after
   close»).
2. Contradice el `S6` del plan: «un `receptionist` no ve el monto teórico en ningún caso».
3. **Fuga indirecta**: la discrepancia es `declarado − teórico`, de modo que publicar la
   discrepancia al receptor le permite recuperar el total teórico **por resta**. El control de
   `RN-02` quedaba violado sin tocar una sola vez el campo que prohíbe.

La corrección (`cash_register.go:245`, `billing.service.go:360`) hace que `ToListItemView` reciba
`canSeeReconciliation` y que el servicio lo calcule con el rol del actor: el receptor ve su
propio conteo declarado —que es su propia declaración— pero ni el teórico ni la discrepancia;
el auditor ve los tres. Verificado con caja real: flotante 50 000, conteo declarado 48 000,
discrepancia −2 000 (15/15 comprobaciones).

**Confianza en el rol:** el servicio se apoya en `X-Actor-Role`, que inyecta el gateway
(`proxy_router.go:164`). Los dos servicios backend **no publican puertos** (`deploy/compose.yaml`):
sólo son alcanzables por el gateway, así que un cliente externo no puede forjar el rol. Es un
requisito de despliegue, no una suposición: si alguien publica el puerto de `billing-stock`, este
control se vuelve hueco. Queda anotado en §5.

### 1.4 MFA de las escrituras — hallazgo cerrado

`RequiresMFA` no distinguía el verbo, de modo que un `GET` de listado y un `POST` de aprobación
compartían la misma respuesta. `RequiresMFAForMethod(método, ruta)` (`rbac_policy.go:73`) separa
ambos: MFA en `POST /api/v1/treatment-plans/{planId}/approve`,
`.../items/{itemId}/complete`, `DELETE .../items/{itemId}`, `POST /api/v1/patients/{id}/treatment-plans`,
`POST /api/v1/patients/{id}/consents` y `POST /api/v1/patients/{id}/consents/{consentId}/sign`;
sin MFA adicional en los `GET` de listado.

**Riesgo residual, asumido:** la regla en blanco de `/api/v1/billing` (preexistente) sigue
exigiendo MFA también en `GET /api/v1/billing/invoices`. **No se relajó** para acomodar el
listado: debilitar una regla de seguridad preexistente para hacer más cómodo un listado nuevo
sería el intercambio equivocado.

### 1.5 El OpenAPI ya no publica rutas inexistentes — hallazgo cerrado

El gateway publicaba 4 rutas que la plataforma no expone (`/api/v1/clinical/odontograms`,
`/api/v1/treatments/plans`, `/api/v1/settlements/commissions`, `/api/v1/inventory/supplies`). Ahora
`defaultOpenAPIJSON` (`openapi_spec.go:205`) se genera desde el registro real (49 rutas / 62
operaciones) e `isPublishableOpenAPI` (`openapi_spec.go:260`) valida el YAML contra ese registro
antes de publicarlo. Verificado: las 11 rutas nuevas están y las 4 ficticias no.

---

## 2. Lente `review-resilience` — degradación y fallo controlado

### 2.1 Un fallo de auditoría no tumba el listado — correcto y verificado

`RNF-06` pide traza, pero una traza que tumba la lectura convierte la auditoría en un punto
único de fallo. El patrón es correcto en los dos servicios: el error del `Recorder` se registra
con `log.Printf` (`billing.service.go:109`) y el listado responde `200` igual. Hay tests con un
`Recorder` que falla siempre, en ambos servicios.

### 2.2 Techo duro de paginación — correcto

`NormalizePage` (`billing.service.go:72`) fija `limit=50` por defecto, techo de 200 y `offset ≥ 0`.
Verificado end-to-end: `limit=9999` devuelve como máximo 200 items. Es lo que impide que un
cliente agote la memoria de un repositorio en memoria.

### 2.3 El seed es idempotente y sobrevive a un arranque en caliente — correcto

`seedCanonicalUsers` (`main.go:117`) comprueba `GetByEmail` antes de crear y no falla si la
cuenta ya existe. El test `cmd/api/main_test.go` lo ejecuta dos veces y comprueba que siguen
siendo 5. Importante porque el servicio **siembra en cada arranque**: un seed no idempotente
habría acumulado usuarios duplicados en cada despliegue.

### 2.4 El gateway cae al contrato embebido si el YAML no es publicable — correcto

Un YAML inválido o que describa rutas inexistentes degrada al contrato embebido en vez de
publicar un contrato roto. `buildOpenAPIJSONFromYAML` (`openapi_spec.go:238`) + `isPublishableOpenAPI`
(`openapi_spec.go:260`).

### 2.5 La persistencia sigue siendo en memoria — riesgo conocido, no introducido

Cada reinicio del contenedor borra pacientes, turnos, facturas, bitácora y usuarios, y vuelve a
sembrar los 5 roles. Es el límite ya documentado en el plan (`S0`) y no se introduce aquí, pero
significa que **las listas del portal arrancan siempre vacías en un entorno recién levantado**, y que
la verificación end-to-end de este work item tiene que sembrar sus propios datos. Queda en §5.

---

## 3. Lente `review-readability` — claridad del código y del contrato

### 3.1 El vocabulario de estados de plan — hallazgo cerrado

El plan especificaba `DRAFT | PARTIALLY_APPROVED | APPROVED | COMPLETED`; el **dominio** usa
`DRAFT | APPROVED_PARTIAL | APPROVED_TOTAL | COMPLETED` (`treatment_plan.go:51`). El plan tenía el
vocabulario equivocado. `normalizePlanStatus` (`treatment.service.go:328`) acepta ambos y mapea al
dominio, con el vocabulario público del plan como forma canónica de la API. Es un puente de
compatibilidad deliberado y documentado, no una ambigüedad: el consumidor (`wi-16`) tiene una
forma estable.

### 3.2 Código muerto y homonimia — hallazgo cerrado

`ListConsentsByPatient` estaba en el puerto `ConsentRepository` **sin un solo llamador** desde
antes de `wi-15` (verificado con `git grep` sobre `c865d15`). Al servir el listado por paciente
con el método paginado, quedaba además un **homónimo** con otra firma y otro contrato
(`{items,total}` con `limit`/`offset` frente a una lista simple). Se retiró del puerto y del
repositorio: una sola vía de lectura, un solo contrato.

### 3.3 Dónde vive la autorización de rol — observación, no defecto

`clinical-agenda` **aplica el rol en el servicio** (`ListUsers` exige `admin`;
`requireConsentReadRole` para consentimientos), mientras que `billing-stock` lo **delega en el
gateway** (`requireListingActor` sólo exige un actor con id y rol). La asimetría es real, y la
justificación también: los servicios backend no tienen puertos publicados (§1.3), el gateway es
el único borde, y el plan `S4` reserva ahí la resolución de los 403. Unificarlo sería una decisión
de arquitectura, no una corrección de este work item; queda anotada para `wi-17`.

La excepción —y es deliberada— es el campo `theoretical_cents`, que necesita el rol **en el
servicio** porque es una regla a nivel de campo, no de ruta (`RN-02`).

### 3.4 La duplicación de `ActorMeta`/`ActorContext` — observación

Cada servicio define su propio tipo de actor (`clinical-agenda`: `ActorMeta` con `MFAVerified`;
`billing-stock`: `ActorContext` con `MFAVerified`). Son equivalentes pero no compartidos: los
servicios son módulos Go independientes y no comparten biblioteca. Es la forma correcta de mantener
la frontera; no es un defecto.

---

## 4. Lente `review-reliability` — contrato, determinismo y cobertura

### 4.1 El contrato `{items, total}` es uniforme en los 11 endpoints — verificado

Los 11 responden `200` con `{"items": [...], "total": N}`, y `total` es el total **antes** de
`limit`/`offset` (verificado con 201 facturas sembradas y una ventana `limit=2&offset=1`).

### 4.2 Orden determinista y paginación sin huecos — correcto

Los repositorios en memoria ordenan por `created_at` descendente con `id` como desempate, y por
`id`/`procedure_code` en las entidades sin `created_at`. Es lo que garantiza que la paginación no
duplique ni omita registros entre páginas.

### 4.3 La cobertura de tests sube en los tres servicios

| Servicio | Funciones de test | Paquetes |
| :--- | ---: | ---: |
| `clinical-agenda` | 40 | 20 |
| `billing-stock` | 25 | 9 |
| `api-gateway` | 21 | 3 |

Cada endpoint nuevo tiene pruebas de handler que verifican status, forma, filtros, paginación y
techo; los listados clínicos y financieros tienen además prueba de traza `EXPORT` y de
tolerancia a un `Recorder` caído.

### 4.4 El `seed` no rompe la credencial del portal — verificado

La cuenta `dentist` conserva correo, contraseña y secreto TOTP. El login real por el portal
(Chromium, correo → MFA → `/patients`) siguió funcionando después del cambio, y el E2E de este
work item inicia sesión con los **5** roles a través del gateway real.

---

## 5. Observaciones abiertas (no bloquean)

1. La bitácora de `billing-stock` **no tiene endpoint de lectura**: las trazas financieras se
   escriben, pero no hay forma de consultarlas por HTTP. `RNF-06` exige la escritura; su lectura
   para el `auditor` es trabajo de `wi-17`.
2. `openapi/sigo-dental.v1.yaml` sigue conteniendo las 4 rutas ficticias. El gateway ya no las
   publica, pero el artefacto es la fuente que `wi-17` debe regenerar.
3. `/api/v1/patients/{id}/soep-notes` no exige MFA. Es preexistente y ajeno a `wi-15`, pero las
   notas SOEP son historia clínica: conviene revisarlo al aplicar `RNF-05` de forma integral.
4. La autorización de rol vive en el gateway (salvo el campo RN-02). Es correcto con los servicios
   internos, y se vuelve frágil si alguien publica sus puertos.
5. El repositorio de usuarios es en memoria y las credenciales de desarrollo están en
   `deploy/README.md`: exige una guardia de entorno antes de producción.

---

## 6. Veredicto

**Aprobado.** Los 11 endpoints `GET` responden el contrato pactado, `RF-08` deja de ser
inalcanzable, el MFA cubre las escrituras de presupuestos y consentimientos, la matriz RBAC
coincide con el SRS §3 y la decisión humana sobre el `auditor`, y `RN-02` queda demostrado en sus
dos capas (estado y rol) con verificación end-to-end sobre cajas reales. 66/66 comprobaciones
end-to-end y los tres servicios con `go build`, `go vet` y `go test` en verde.

Correcciones aplicadas dentro del presupuesto de `MaxCorrectionChangedLines = 200` (~55 líneas
reales): el defecto de `RN-02` por rol y la retirada del método huérfano.
