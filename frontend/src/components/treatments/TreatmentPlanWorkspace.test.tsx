import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  buildTreatmentPlanViewModel,
  renderTreatmentPlanWorkspaceHtml,
  type TreatmentPlanDTO,
} from "./TreatmentPlanWorkspace.tsx";

describe("wi-11 S5 — Multi-Phase Treatment Plans, Auto-Collapsed Completed Phases & Partial/Total Approval (RF-07, RF-08)", () => {
  const samplePlan: TreatmentPlanDTO = {
    id: "plan-2026-01",
    patientId: "pat-001",
    title: "Rehabilitación Integral Superior",
    phases: [
      {
        id: "phase-1",
        name: "Fase 1: Urgencia e Higiene Periodontal",
        items: [
          {
            id: "item-1",
            procedureCode: "PER-01",
            description: "Tartrectomía y Profilaxis",
            toothFdi: undefined,
            priceCents: 4500_00,
            status: "REALIZADO",
          },
          {
            id: "item-2",
            procedureCode: "URG-02",
            description: "Inactivación de Caries Pieza 16",
            toothFdi: "16",
            priceCents: 3000_00,
            status: "REALIZADO",
          },
        ],
      },
      {
        id: "phase-2",
        name: "Fase 2: Restauradora y Endodoncia",
        items: [
          {
            id: "item-3",
            procedureCode: "END-16",
            description: "Endodoncia Birradicular Pieza 14",
            toothFdi: "14",
            priceCents: 18000_00,
            status: "PENDIENTE",
          },
          {
            id: "item-4",
            procedureCode: "RES-16",
            description: "Restauración Resina Compuesta Pieza 16",
            toothFdi: "16",
            priceCents: 9500_00,
            status: "PENDIENTE",
          },
        ],
      },
    ],
  };

  it("auto-collapses phases where all items are REALIZADO to emphasize what's next (AGENTS.md §3, S5)", () => {
    const vm = buildTreatmentPlanViewModel(samplePlan, ["item-3"]);
    assert.equal(vm.phases[0].isCompleted, true);
    assert.equal(vm.phases[0].isCollapsedByDefault, true);
    assert.equal(vm.phases[1].isCompleted, false);
    assert.equal(vm.phases[1].isCollapsedByDefault, false);
  });

  it("recalculates partial approval subtotal and transitions selected items into schedulable tasks (RF-08, S5)", () => {
    const vmPartial = buildTreatmentPlanViewModel(samplePlan, ["item-3"]);
    assert.equal(vmPartial.totalPlanCents, 35000_00);
    assert.equal(vmPartial.selectedSubtotalCents, 18000_00);
    assert.equal(vmPartial.approvalMode, "PARTIAL");
    assert.deepEqual(vmPartial.schedulableTaskIds, ["item-3"]);

    const vmTotal = buildTreatmentPlanViewModel(samplePlan, ["item-3", "item-4"]);
    assert.equal(vmTotal.selectedSubtotalCents, 27500_00);
    assert.equal(vmTotal.approvalMode, "TOTAL");
    assert.deepEqual(vmTotal.schedulableTaskIds, ["item-3", "item-4"]);

    const html = renderTreatmentPlanWorkspaceHtml({
      patientId: "pat-001",
      plan: samplePlan,
      selectedItemIds: ["item-3"],
    });
    assert.match(html, /tabular-nums/);
    assert.match(html, /Fase Completada \(Colapsada\)/);
    assert.match(html, /Aprobar Selección \(PARCIAL\)/);
  });
});
