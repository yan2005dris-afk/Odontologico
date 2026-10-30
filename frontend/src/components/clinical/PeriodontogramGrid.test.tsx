import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  PERIODONTAL_SITES_6,
  calculateClinicalAttachmentLevel,
  computeToothPeriodontalMetrics,
  renderPeriodontogramGridHtml,
  type PeriodontogramToothDTO,
} from "./PeriodontogramGrid.tsx";

describe("wi-11 S3 — Digital Periodontogram (6 sites/tooth, NIC = PD - GM calculation & tabular-nums RF-04)", () => {
  it("defines the 6 standard probing sites per tooth (MB, B, DB, ML, L, DL)", () => {
    assert.deepEqual(PERIODONTAL_SITES_6, ["MB", "B", "DB", "ML", "L", "DL"]);
  });

  it("calculates Clinical Attachment Level (NIC = PD - GM) accurately across all 6 sites (S3)", () => {
    const probingDepthMm: [number, number, number, number, number, number] = [3, 2, 4, 3, 2, 3];
    const gingivalMarginMm: [number, number, number, number, number, number] = [0, -1, -1, 0, 0, -1];

    const nic = calculateClinicalAttachmentLevel(probingDepthMm, gingivalMarginMm);
    assert.deepEqual(nic, [3, 3, 5, 3, 2, 4]);
  });

  it("flags pathological pockets (PD >= 4mm) and renders tabular-nums on all numeric cells (S3, AGENTS.md §3)", () => {
    const tooth16: PeriodontogramToothDTO = {
      fdiTooth: "16",
      probingDepthMm: [3, 2, 4, 3, 2, 3],
      gingivalMarginMm: [0, -1, -1, 0, 0, -1],
      bleedingOnProbing: [false, false, true, false, false, false],
      furcationGrade: 1,
      mobilityGrade: 0,
    };

    const metrics = computeToothPeriodontalMetrics(tooth16);
    assert.deepEqual(metrics.clinicalAttachmentLevelMm, [3, 3, 5, 3, 2, 4]);
    assert.equal(metrics.hasPathologicalPocket, true);
    assert.equal(metrics.bopCount, 1);

    const html = renderPeriodontogramGridHtml({
      patientId: "pat-001",
      teethData: [tooth16],
    });

    assert.match(html, /tabular-nums/);
    assert.match(html, /NIC: 3, 3, 5, 3, 2, 4/);
  });
});
