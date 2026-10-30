import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  PERMANENT_FDI_TEETH,
  DECIDUOUS_FDI_TEETH,
  ALL_52_FDI_TEETH,
  DENTAL_SURFACES,
  FINDING_CATALOG_COLORS,
  fdiToUniversal,
  universalToFdi,
  canMutateOdontogramFinding,
  renderOdontogramCanvasHtml,
  type OdontogramFindingDTO,
} from "./OdontogramCanvas.tsx";

describe("wi-11 S1 & S2 — 2D Interactive SVG Odontogram (52 teeth, FDI <-> Universal, 5 surfaces, RN-01 lock)", () => {
  it("exposes all 32 permanent teeth and 20 deciduous teeth (52 total) and 5 dental surfaces (RF-03)", () => {
    assert.equal(PERMANENT_FDI_TEETH.length, 32);
    assert.equal(DECIDUOUS_FDI_TEETH.length, 20);
    assert.equal(ALL_52_FDI_TEETH.length, 52);
    assert.deepEqual(DENTAL_SURFACES, [
      "MESIAL",
      "DISTAL",
      "OCLUSAL_INCISAL",
      "VESTIBULAR",
      "LINGUAL_PALATINO",
    ]);
  });

  it("converts bidirectionally between FDI (11..48, 51..85) and Universal (1..32, A..T) notation (S1)", () => {
    assert.equal(fdiToUniversal("18"), "1");
    assert.equal(fdiToUniversal("16"), "3");
    assert.equal(fdiToUniversal("28"), "16");
    assert.equal(fdiToUniversal("38"), "17");
    assert.equal(fdiToUniversal("48"), "32");
    assert.equal(fdiToUniversal("55"), "A");
    assert.equal(fdiToUniversal("65"), "J");
    assert.equal(fdiToUniversal("75"), "K");
    assert.equal(fdiToUniversal("85"), "T");

    assert.equal(universalToFdi("3"), "16");
    assert.equal(universalToFdi("A"), "55");
    assert.equal(universalToFdi("T"), "85");
  });

  it("assigns strict semantic colors: red (#DC2626) for CARIES and blue (#2563EB) for EXISTING_RESTORATION (AGENTS.md §3)", () => {
    assert.equal(FINDING_CATALOG_COLORS.CARIES, "#DC2626");
    assert.equal(FINDING_CATALOG_COLORS.EXISTING_RESTORATION, "#2563EB");
  });

  it("enforces RN-01 immutability: findings marked as REALIZADO cannot be edited or deleted (S2)", () => {
    const plannedFinding: OdontogramFindingDTO = {
      id: "find-1",
      fdiTooth: "16",
      surface: "OCLUSAL_INCISAL",
      findingCode: "CARIES",
      status: "PLANIFICADO",
    };
    const completedFinding: OdontogramFindingDTO = {
      id: "find-2",
      fdiTooth: "26",
      surface: "MESIAL",
      findingCode: "EXISTING_RESTORATION",
      status: "REALIZADO",
    };

    assert.equal(canMutateOdontogramFinding(plannedFinding), true);
    assert.equal(canMutateOdontogramFinding(completedFinding), false);
  });

  it("renders all 52 teeth in < 100ms on pure white (#FFFFFF) canvas with tabular-nums and RN-01 lock indicators (RNF-01, S1, S2)", () => {
    const findings: OdontogramFindingDTO[] = [
      {
        id: "f-100",
        fdiTooth: "16",
        surface: "OCLUSAL_INCISAL",
        findingCode: "CARIES",
        status: "PLANIFICADO",
      },
      {
        id: "f-101",
        fdiTooth: "55",
        surface: "MESIAL",
        findingCode: "EXISTING_RESTORATION",
        status: "REALIZADO",
      },
    ];

    const start = performance.now();
    const htmlFdi = renderOdontogramCanvasHtml({
      patientId: "pat-001",
      notation: "FDI",
      dentitionMode: "MIXED",
      findings,
    });
    const elapsedMs = performance.now() - start;

    assert.equal(elapsedMs < 100, true, `Expected render < 100ms, got ${elapsedMs}ms`);
    assert.match(htmlFdi, /#FFFFFF|bg-white/);
    assert.match(htmlFdi, /tabular-nums/);
    assert.match(htmlFdi, /#DC2626/);
    assert.match(htmlFdi, /#2563EB/);
    assert.match(htmlFdi, /RN-01 Inmutable/);
    assert.match(htmlFdi, /Agregar Adenda Firmada/);

    const htmlUniversal = renderOdontogramCanvasHtml({
      patientId: "pat-001",
      notation: "UNIVERSAL",
      dentitionMode: "MIXED",
      findings,
    });
    assert.match(htmlUniversal, /data-tooth-label="3"/);
    assert.match(htmlUniversal, /data-tooth-label="A"/);
  });
});
