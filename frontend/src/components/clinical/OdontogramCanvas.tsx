/**
 * OdontogramCanvas Component (wi-11 S1 & S2, RF-03, RN-01, RNF-01, RNF-07, AGENTS.md §3).
 * High-performance 2D interactive SVG odontogram supporting 52 teeth (32 permanent + 20 deciduous),
 * instant FDI <-> Universal notation toggle, 5 dental surfaces per tooth, semantic color catalog
 * (#DC2626 caries, #2563EB existing restorations), and RN-01 immutability lock on "REALIZADO".
 */

import { escapeHtml } from "../../lib/session-idle-guard.ts";

export type DentalNotation = "FDI" | "UNIVERSAL";
export type DentitionMode = "PERMANENT" | "DECIDUOUS" | "MIXED";

export type DentalSurface =
  | "MESIAL"
  | "DISTAL"
  | "OCLUSAL_INCISAL"
  | "VESTIBULAR"
  | "LINGUAL_PALATINO";

export const DENTAL_SURFACES: DentalSurface[] = [
  "MESIAL",
  "DISTAL",
  "OCLUSAL_INCISAL",
  "VESTIBULAR",
  "LINGUAL_PALATINO",
];

export type OdontogramFindingCode =
  | "CARIES"
  | "EXISTING_RESTORATION"
  | "ENDODONTICS"
  | "IMPLANT"
  | "PROSTHESIS_FIXED"
  | "EXTRACTION"
  | "SEALANT";

export const FINDING_CATALOG_COLORS: Record<OdontogramFindingCode, string> = {
  CARIES: "#DC2626", // Red per RF-03 & AGENTS.md §3
  EXISTING_RESTORATION: "#2563EB", // Blue per RF-03 & AGENTS.md §3
  ENDODONTICS: "#7C3AED",
  IMPLANT: "#0D9488",
  PROSTHESIS_FIXED: "#475569",
  EXTRACTION: "#1E293B",
  SEALANT: "#16A34A",
};

export interface OdontogramFindingDTO {
  id: string;
  fdiTooth: string;
  surface: DentalSurface | "WHOLE_TOOTH";
  findingCode: OdontogramFindingCode;
  status: "PLANIFICADO" | "REALIZADO";
}

export const PERMANENT_FDI_TEETH: string[] = [
  // Upper Right (18..11)
  "18", "17", "16", "15", "14", "13", "12", "11",
  // Upper Left (21..28)
  "21", "22", "23", "24", "25", "26", "27", "28",
  // Lower Left (38..31)
  "38", "37", "36", "35", "34", "33", "32", "31",
  // Lower Right (41..48)
  "41", "42", "43", "44", "45", "46", "47", "48",
];

export const DECIDUOUS_FDI_TEETH: string[] = [
  // Upper Right (55..51)
  "55", "54", "53", "52", "51",
  // Upper Left (61..65)
  "61", "62", "63", "64", "65",
  // Lower Left (75..71)
  "75", "74", "73", "72", "71",
  // Lower Right (81..85)
  "81", "82", "83", "84", "85",
];

export const ALL_52_FDI_TEETH: string[] = [...PERMANENT_FDI_TEETH, ...DECIDUOUS_FDI_TEETH];

const FDI_TO_UNIVERSAL_MAP: Record<string, string> = {
  // Permanent Upper Right 18..11 -> 1..8
  "18": "1", "17": "2", "16": "3", "15": "4", "14": "5", "13": "6", "12": "7", "11": "8",
  // Permanent Upper Left 21..28 -> 9..16
  "21": "9", "22": "10", "23": "11", "24": "12", "25": "13", "26": "14", "27": "15", "28": "16",
  // Permanent Lower Left 38..31 -> 17..24
  "38": "17", "37": "18", "36": "19", "35": "20", "34": "21", "33": "22", "32": "23", "31": "24",
  // Permanent Lower Right 41..48 -> 25..32
  "41": "25", "42": "26", "43": "27", "44": "28", "45": "29", "46": "30", "47": "31", "48": "32",
  // Deciduous Upper Right 55..51 -> A..E
  "55": "A", "54": "B", "53": "C", "52": "D", "51": "E",
  // Deciduous Upper Left 61..65 -> F..J
  "61": "F", "62": "G", "63": "H", "64": "I", "65": "J",
  // Deciduous Lower Left 75..71 -> K..O
  "75": "K", "74": "L", "73": "M", "72": "N", "71": "O",
  // Deciduous Lower Right 81..85 -> P..T
  "81": "P", "82": "Q", "83": "R", "84": "S", "85": "T",
};

const UNIVERSAL_TO_FDI_MAP: Record<string, string> = Object.fromEntries(
  Object.entries(FDI_TO_UNIVERSAL_MAP).map(([fdi, univ]) => [univ, fdi])
);

export function fdiToUniversal(fdi: string): string {
  return FDI_TO_UNIVERSAL_MAP[fdi] ?? fdi;
}

export function universalToFdi(universal: string): string {
  return UNIVERSAL_TO_FDI_MAP[universal] ?? universal;
}

/**
 * Enforces RN-01: Any finding marked as "REALIZADO" is immutable and cannot be deleted or mutated.
 */
export function canMutateOdontogramFinding(finding: OdontogramFindingDTO): boolean {
  return finding.status !== "REALIZADO";
}

export function renderOdontogramCanvasHtml(props: {
  patientId: string;
  notation: DentalNotation;
  dentitionMode?: DentitionMode;
  findings: OdontogramFindingDTO[];
}): string {
  const mode = props.dentitionMode ?? "MIXED";
  const activeTeeth =
    mode === "PERMANENT"
      ? PERMANENT_FDI_TEETH
      : mode === "DECIDUOUS"
        ? DECIDUOUS_FDI_TEETH
        : ALL_52_FDI_TEETH;

  const findingsByTooth = new Map<string, OdontogramFindingDTO[]>();
  for (const f of props.findings) {
    const list = findingsByTooth.get(f.fdiTooth) ?? [];
    list.push(f);
    findingsByTooth.set(f.fdiTooth, list);
  }

  const teethSvgItems = activeTeeth
    .map((fdi) => {
      const displayLabel = props.notation === "FDI" ? fdi : fdiToUniversal(fdi);
      const toothFindings = findingsByTooth.get(fdi) ?? [];
      const hasImmutableFinding = toothFindings.some((f) => !canMutateOdontogramFinding(f));

      const surfacesSvg = DENTAL_SURFACES.map((surface) => {
        const match = toothFindings.find((f) => f.surface === surface);
        const fill = match ? FINDING_CATALOG_COLORS[match.findingCode] : "#FFFFFF";
        return `<rect data-surface="${surface}" width="44" height="44" fill="${fill}" stroke="#CBD5E1" stroke-width="1.5" />`;
      }).join("");

      return `<div class="p-2 rounded border border-slate-200 bg-white flex flex-col items-center" data-fdi="${fdi}" data-tooth-label="${displayLabel}">
        <span class="text-xs font-mono font-bold tabular-nums text-slate-900 mb-1">${displayLabel}</span>
        <svg viewBox="0 0 100 100" class="w-12 h-12" role="img" aria-label="Pieza dental ${displayLabel}">
          ${surfacesSvg}
        </svg>
        ${
          hasImmutableFinding
            ? `<div class="mt-1 flex flex-col items-center gap-0.5">
                <span class="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-white font-semibold">RN-01 Inmutable</span>
                <button type="button" class="text-[10px] text-blue-700 underline font-medium">Agregar Adenda Firmada</button>
              </div>`
            : ""
        }
      </div>`;
    })
    .join("");

  return `<section class="bg-white rounded-xl border border-slate-200 p-6 space-y-4" style="background-color: #FFFFFF" aria-label="Odontograma Gráfico Interactivo 2D">
    <header class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 pb-3">
      <div>
        <h2 class="text-base font-bold text-slate-900">Odontograma 2D Vectorial (${activeTeeth.length} piezas · ${props.notation})</h2>
        <p class="text-xs text-slate-600">Catálogo oficial: Caries (<span style="color:#DC2626">#DC2626</span>) · Restauración existente (<span style="color:#2563EB">#2563EB</span>) · Bloqueo RN-01 activo</p>
      </div>
      <div class="flex items-center gap-2">
        <button type="button" class="min-h-[44px] px-3 py-1.5 rounded border border-slate-300 text-xs font-semibold tabular-nums">
          Alternar Nomenclatura (FDI / Universal)
        </button>
      </div>
    </header>
    <div class="grid grid-cols-4 sm:grid-cols-8 lg:grid-cols-16 gap-2 tabular-nums">
      ${teethSvgItems}
    </div>
  </section>`;
}

export default renderOdontogramCanvasHtml;
