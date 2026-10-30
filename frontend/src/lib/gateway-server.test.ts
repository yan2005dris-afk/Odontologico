import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  DEFAULT_GATEWAY_INTERNAL_URL,
  GatewayError,
  createGatewayServerClient,
  getGatewayInternalUrl,
  mapAppointment,
  mapFindingCode,
  mapOdontogramFindings,
  mapPatientSummary,
  mapPeriodontogramTeeth,
  mapSoepNotes,
} from "./gateway-server.ts";

describe("wi-13 S4/S5 — server-side gateway client (same-origin data flow)", () => {
  it("resolves the internal gateway URL and defaults to the compose service", () => {
    assert.equal(getGatewayInternalUrl({}), DEFAULT_GATEWAY_INTERNAL_URL);
    assert.equal(
      getGatewayInternalUrl({ GATEWAY_INTERNAL_URL: "http://api-gateway:8080/" }),
      "http://api-gateway:8080"
    );
    assert.equal(
      getGatewayInternalUrl({ GATEWAY_INTERNAL_URL: "http://gateway.internal:9090" }),
      "http://gateway.internal:9090"
    );
  });

  it("maps a patient search item onto the portal DTO", () => {
    const patient = mapPatientSummary({
      id: "pat-1",
      hc_number: "HC-2026-000001",
      first_name: "María",
      last_name: "Fernández Gómez",
      age: 64,
      is_minor: false,
      insurance_plan: { provider_name: "OSDE", plan_name: "Plan 310" },
      critical_alerts: [
        { code: "PENICILLIN_ALLERGY", label: "Alergia a penicilina", severity: "CRITICAL" },
        { code: "ANTICOAGULANTS", label: "Anticoagulado", severity: "HIGH" },
      ],
    });

    assert.equal(patient.id, "pat-1");
    assert.equal(patient.hcNumber, "HC-2026-000001");
    assert.equal(patient.fullName, "María Fernández Gómez");
    assert.equal(patient.age, 64);
    assert.equal(patient.isMinor, false);
    assert.equal(patient.insurancePlan?.providerName, "OSDE");
    assert.equal(patient.criticalAlerts.length, 2);
    assert.equal(patient.criticalAlerts[0].severity, "CRITICAL");
    assert.equal(patient.criticalAlerts[1].severity, "HIGH");
  });

  it("maps the clinical header payload that carries full_name", () => {
    const patient = mapPatientSummary({
      id: "pat-2",
      hc_number: "HC-2026-000002",
      full_name: "Lucas Mateo Díaz",
      age: 12,
      is_minor: true,
      critical_alerts: [],
    });

    assert.equal(patient.fullName, "Lucas Mateo Díaz");
    assert.equal(patient.isMinor, true);
    assert.equal(patient.insurancePlan, undefined);
    assert.deepEqual(patient.criticalAlerts, []);
  });

  it("maps the odontogram finding catalog and reports unmapped codes", () => {
    const mapped = mapOdontogramFindings([
      { id: "f-1", tooth_fdi: "16", surface: "OCLUSAL_INCISAL", finding_type: "CARIES", status: "DIAGNOSED" },
      { id: "f-2", tooth_fdi: "26", surface: "MESIAL", finding_type: "EXISTING_RESTORATION", status: "REALIZADO" },
      { id: "f-3", tooth_fdi: "36", surface: "WHOLE_TOOTH", finding_type: "ENDODONTICS_DONE", status: "PLANNED" },
      { id: "f-4", tooth_fdi: "46", surface: "VESTIBULAR", finding_type: "NOT_A_CATALOG_CODE", status: "DIAGNOSED" },
    ]);

    assert.equal(mapped.unmappedCount, 1);
    assert.equal(mapped.findings.length, 3);
    assert.equal(mapped.findings[0].findingCode, "CARIES");
    assert.equal(mapped.findings[0].status, "PLANIFICADO");
    assert.equal(mapped.findings[1].findingCode, "EXISTING_RESTORATION");
    assert.equal(mapped.findings[1].status, "REALIZADO");
    assert.equal(mapped.findings[2].findingCode, "ENDODONTICS");
    assert.equal(mapFindingCode("FIXED_PROSTHESIS"), "PROSTHESIS_FIXED");
    assert.equal(mapFindingCode("UNKNOWN"), null);
  });

  it("maps the periodontogram teeth into six-site tuples", () => {
    const teeth = mapPeriodontogramTeeth([
      {
        tooth_fdi: "16",
        probing_depth_mm: [3, 4, 3, 2, 3, 2],
        gingival_margin_mm: [0, -1, 0, 0, 0, 0],
        bleeding_on_probing: [true, false, false, false, false, true],
        furcation_grade: 1,
        mobility_grade: 0,
      },
    ]);

    assert.equal(teeth.length, 1);
    assert.equal(teeth[0].fdiTooth, "16");
    assert.deepEqual(teeth[0].probingDepthMm, [3, 4, 3, 2, 3, 2]);
    assert.deepEqual(teeth[0].bleedingOnProbing, [true, false, false, false, false, true]);
    assert.equal(teeth[0].furcationGrade, 1);
  });

  it("maps SOEP notes and their signed addenda", () => {
    const notes = mapSoepNotes([
      {
        id: "n-1",
        patient_id: "pat-1",
        author_dentist_id: "dent-1",
        subjective: "S",
        objective: "O",
        assessment: "A",
        plan: "P",
        created_at: "2026-10-30T10:00:00Z",
        is_locked: false,
        addenda: [
          {
            id: "add-1",
            author_dentist_id: "dent-2",
            correction_text: "Corrección firmada",
            created_at: "2026-10-30T12:00:00Z",
          },
        ],
      },
    ]);

    assert.equal(notes.length, 1);
    assert.equal(notes[0].authorName, "dent-1");
    assert.equal(notes[0].addenda[0].content, "Corrección firmada");
    assert.equal(notes[0].addenda[0].signedAtIso, "2026-10-30T12:00:00Z");
  });

  it("maps a scheduling appointment onto the calendar DTO", () => {
    const appointment = mapAppointment(
      {
        id: "apt-1",
        patient_id: "pat-1",
        dentist_id: "dent-1",
        chair_id: "SILLON-1",
        clinic_id: "clinic-1",
        start_at: "2026-10-30T14:00:00Z",
        end_at: "2026-10-30T15:00:00Z",
        status: "CONFIRMADO",
        reason: "Control de ortodoncia",
      },
      { "pat-1": "María Fernández Gómez" }
    );

    assert.equal(appointment.date, "2026-10-30");
    assert.equal(appointment.startTime, "14:00");
    assert.equal(appointment.endTime, "15:00");
    assert.equal(appointment.patientName, "María Fernández Gómez");
    assert.equal(appointment.chairLabel, "Sillón SILLON-1");
    assert.equal(appointment.status, "CONFIRMADO");
    assert.equal(appointment.procedureSummary, "Control de ortodoncia");
  });

  it("sends the session bearer token and maps the patient list response", async () => {
    const calls: { url: string; init: RequestInit }[] = [];
    const client = createGatewayServerClient({
      baseUrl: "http://api-gateway:8080",
      accessToken: "jwt-token",
      fetchImpl: (async (url: string, init: RequestInit) => {
        calls.push({ url, init });
        return new Response(
          JSON.stringify({
            items: [
              {
                id: "pat-1",
                hc_number: "HC-1",
                first_name: "Ana",
                last_name: "Pérez",
                age: 30,
                is_minor: false,
                critical_alerts: [],
              },
            ],
            total: 1,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }) as unknown as typeof fetch,
    });

    const result = await client.listPatients();

    assert.equal(result.total, 1);
    assert.equal(result.items[0].fullName, "Ana Pérez");
    assert.equal(calls[0].url, "http://api-gateway:8080/api/v1/patients");
    const headers = calls[0].init.headers as Record<string, string>;
    assert.equal(headers.Authorization, "Bearer jwt-token");
  });

  it("raises a GatewayError carrying the upstream error code", async () => {
    const client = createGatewayServerClient({
      baseUrl: "http://api-gateway:8080",
      accessToken: "jwt-token",
      fetchImpl: (async () =>
        new Response(JSON.stringify({ code: "SESSION_IDLE_TIMEOUT", message: "Sesión expirada" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        })) as unknown as typeof fetch,
    });

    await assert.rejects(
      () => client.listPatients(),
      (error: unknown) => {
        assert.ok(error instanceof GatewayError);
        assert.equal(error.code, "SESSION_IDLE_TIMEOUT");
        assert.equal(error.status, 401);
        assert.equal(error.isUnauthorized, true);
        return true;
      }
    );
  });

  it("reports an unreachable gateway instead of a fabricated success", async () => {
    const client = createGatewayServerClient({
      baseUrl: "http://api-gateway:8080",
      fetchImpl: (async () => {
        throw new Error("connect ECONNREFUSED");
      }) as unknown as typeof fetch,
    });

    await assert.rejects(
      () => client.listPatients(),
      (error: unknown) => {
        assert.ok(error instanceof GatewayError);
        assert.equal(error.code, "GATEWAY_UNREACHABLE");
        assert.equal(error.status, 0);
        return true;
      }
    );
  });
});
