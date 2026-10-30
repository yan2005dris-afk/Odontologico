import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  isSoepNoteLockedByWindow,
  validateSoepAddendumInput,
  renderSoepEvolutionTimelineHtml,
  type SoepNoteDTO,
} from "./SoepEvolutionTimeline.tsx";

describe("wi-11 S2 — Immutable SOEP Evolution Notes (24h lock window & Signed Addenda RF-05, RN-01)", () => {
  it("locks SOEP notes automatically once 24 hours have elapsed or when isLocked flag is set (RN-01)", () => {
    const createdIso = "2026-10-29T08:00:00Z";
    const within24hIso = "2026-10-29T20:00:00Z"; // 12h later
    const past24hIso = "2026-10-30T08:00:01Z"; // 24h + 1s later

    assert.equal(isSoepNoteLockedByWindow(createdIso, within24hIso, false), false);
    assert.equal(isSoepNoteLockedByWindow(createdIso, past24hIso, false), true);
    assert.equal(isSoepNoteLockedByWindow(createdIso, within24hIso, true), true);
  });

  it("disables edit/delete buttons on locked SOEP notes and enables only 'Agregar Adenda Firmada' (S2)", () => {
    const lockedNote: SoepNoteDTO = {
      id: "soep-001",
      patientId: "pat-001",
      authorName: "Dr. Alejandro Vega",
      createdAtIso: "2026-10-28T10:00:00Z",
      isLocked: true,
      subjective: "Paciente refiere sensibilidad al frío en cuadrante superior derecho.",
      objective: "Caries oclusal profunda en pieza 16 sin exposición pulpar.",
      assessment: "Pulpitis reversible pieza 16.",
      plan: "Restauración con resina compuesta fotopolimerizable.",
      addenda: [
        {
          id: "add-01",
          authorName: "Dr. Alejandro Vega",
          signedAtIso: "2026-10-29T11:30:00Z",
          content: "Rectificación: se aplicó recubrimiento pulpar indirecto con hidróxido de calcio.",
        },
      ],
    };

    const html = renderSoepEvolutionTimelineHtml({
      patientId: "pat-001",
      notes: [lockedNote],
      activeAddendumNoteId: "soep-001",
    });

    assert.match(html, /Bloqueada \(RN-01 · &gt;24h\)/);
    assert.match(html, /Agregar Adenda Firmada/);
    assert.equal(html.includes('data-action="delete-soep"'), false);
    assert.match(html, /role="dialog"/);
  });

  it("validates signed addendum input requiring non-empty clinical text and professional signature confirmation", () => {
    assert.equal(
      validateSoepAddendumInput({ noteId: "soep-001", content: "   ", confirmSignature: true }).valid,
      false
    );
    assert.equal(
      validateSoepAddendumInput({
        noteId: "soep-001",
        content: "Adenda aclaratoria sobre anestesia utilizada.",
        confirmSignature: false,
      }).valid,
      false
    );
    assert.equal(
      validateSoepAddendumInput({
        noteId: "soep-001",
        content: "Adenda aclaratoria sobre anestesia utilizada.",
        confirmSignature: true,
      }).valid,
      true
    );
  });
});
