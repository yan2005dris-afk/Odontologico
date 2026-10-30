/**
 * Clinical Supplies Inventory Page (wi-13 S9, RF-14).
 *
 * The billing service does not expose a read endpoint for the supplies
 * catalog, so the page shows the explicit empty state that names the missing
 * endpoint. No stock level is invented.
 */

import { buildMissingEndpointEmptyState } from "../../../lib/portal-views.ts";
import { requirePortalSession } from "../../../lib/session.ts";

export default async function InventoryPage() {
  await requirePortalSession();
  const state = buildMissingEndpointEmptyState("inventory");

  return (
    <div className="p-6 space-y-4">
      <header>
        <h1 className="text-lg font-bold text-slate-900">{state.title}</h1>
        <p className="text-xs text-slate-600">
          Catálogo de insumos clínicos y descuento automático de stock (RF-14).
        </p>
      </header>

      <section
        role="status"
        data-state="empty"
        data-missing-endpoint={state.missingEndpoint}
        className="p-6 rounded-lg border border-dashed border-slate-300 bg-white space-y-3"
      >
        <h2 className="text-sm font-bold text-slate-900">Sin datos para mostrar</h2>
        <p className="text-xs text-slate-600">{state.description}</p>
        <dl className="text-xs text-slate-700 space-y-1">
          <div>
            <dt className="inline font-semibold">Endpoint de lectura pendiente: </dt>
            <dd className="inline font-mono">{state.missingEndpoint}</dd>
          </div>
          <div>
            <dt className="inline font-semibold">Operaciones disponibles: </dt>
            <dd className="inline font-mono">
              {state.availableOperations.join(", ")}
            </dd>
          </div>
        </dl>
        <p className="text-xs text-slate-500">
          No hay insumos ni existencias que consultar hasta que exista el endpoint de
          listado.
        </p>
      </section>
    </div>
  );
}
