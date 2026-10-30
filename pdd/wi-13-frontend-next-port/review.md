# Review — `wi-13-frontend-next-port`

Revisión 4-lentes ejecutada con **pandorx** usando la configuración global de
la máquina (`~/.pandorx/config.json`), cuyo `role_assignments.reviewer_model`
es **MiniMax-M3**.

---

## 1. Configuración de la review

| Elemento | Valor |
| :--- | :--- |
| Configuración | `~/.pandorx/config.json` (global de la máquina) |
| Modelo de reviewer | `MiniMax-M3` (`role_assignments.reviewer_model`) |
| Provider | `minimax` (adaptador HTTP de pandorx → `https://api.minimaxi.chat/v1/text/chatcompletion_v2`) |
| Credencial | `api_keys.minimax` de la configuración global (nunca impresa) |
| Flow | [`pdd/wi-13-frontend-next-port/review-flow.yaml`](./review-flow.yaml) |
| Lentes | `review-risk`, `review-resilience`, `review-readability`, `review-reliability` |
| Evidencia | recibos `gentle-ai.review-receipt/v1` + `pandorx lineage show <id>` en `.pandorx/pandorx.db` |

### Ajustes al binario de pandorx (disclosed)

El binario instalado (`pandorx 0.1.0`) fija un timeout HTTP de **2 minutos**
por llamada al provider y dispara las cuatro lentes sin pausa. Con MiniMax-M3
—que es un modelo de razonamiento: 6.000–21.000 tokens de *reasoning* por
respuesta— esa ventana expira de forma intermitente y la review moría en la
tercera lente.

Para que la review fuera ejecutable se compiló **el mismo código fuente de
pandorx 0.1.0** (`~/Documentos/Github/personal/pandorx`) con dos cambios
mínimos y **sin tocar el binario instalado**:

1. `internal/orchestration/adapters/providers/minimax/minimax.go`: timeout del
   cliente HTTP `2 * time.Minute` → `15 * time.Minute`.
2. `internal/orchestration/application/run_flow/run_flow.service.go`: 8 s de
   pausa entre lentes de un mismo step (el API estrangula ráfagas de
   generaciones largas).

El modelo, la configuración, el flow y el contrato de hallazgos son los del
config global; lo único distinto es la ventana de espera del provider.

### Cubrimiento

La review corrió sobre **14 slices acotados** del cambio (infraestructura,
gateway, capa `lib` del frontend, layouts/login, páginas y renderers), con el
material de cada slice traveling en `--task`. Los pasos estructurales del flow
(orchestrator, implementer) corrieron con el provider `mock` in-proceso: sólo
la lente de reviewer llama al modelo.

---

## 2. Hallazgos

_(se completan con los recibos de cada slice)_

---

## 3. Correcciones aplicadas

_(se completan)_
