import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  filterAppointments,
  detectChairOverlapConflict,
  getAllowedStatusTransitions,
  canTransitionAppointmentStatus,
  renderMultiChairCalendarHtml,
  type CalendarAppointmentDTO,
} from "./MultiChairCalendar.tsx";

describe("wi-10 S4 & S5 — Multi-Chair Calendar, Chair Overlap Prevention (RN-03) & 7-State Machine (RF-09, RF-10)", () => {
  const existingAppointments: CalendarAppointmentDTO[] = [
    {
      id: "apt-101",
      patientId: "pat-001",
      patientName: "María Fernanda Gómez",
      chairId: "chair-1",
      chairLabel: "Sillón 1 - Cirugía",
      dentistId: "dent-1",
      dentistName: "Dr. Alejandro Vega",
      date: "2026-10-30",
      startTime: "09:00",
      endTime: "10:00",
      status: "CONFIRMADO",
      procedureSummary: "Implante Pieza 16",
    },
    {
      id: "apt-102",
      patientId: "pat-002",
      patientName: "Lucas Mateo Díaz",
      chairId: "chair-2",
      chairLabel: "Sillón 2 - Odontopediatría",
      dentistId: "dent-2",
      dentistName: "Dra. Valeria Solís",
      date: "2026-10-30",
      startTime: "09:30",
      endTime: "10:15",
      status: "PROGRAMADO",
      procedureSummary: "Sellante Pieza 55",
    },
  ];

  it("filters appointments by physical dental chair (chairId) and specialist (dentistId) (RF-09)", () => {
    const chair1Only = filterAppointments(existingAppointments, {
      view: "day",
      date: "2026-10-30",
      chairId: "chair-1",
    });
    assert.equal(chair1Only.length, 1);
    assert.equal(chair1Only[0].id, "apt-101");

    const dent2Only = filterAppointments(existingAppointments, {
      view: "day",
      date: "2026-10-30",
      dentistId: "dent-2",
    });
    assert.equal(dent2Only.length, 1);
    assert.equal(dent2Only[0].id, "apt-102");
  });

  it("blocks booking on the same physical chair during an overlapping time window even with a different dentist (RN-03)", () => {
    const conflictCheck = detectChairOverlapConflict(existingAppointments, {
      chairId: "chair-1",
      dentistId: "dent-2", // Different dentist! Still same physical chair-1
      date: "2026-10-30",
      startTime: "09:30",
      endTime: "10:30",
    });

    assert.equal(conflictCheck.hasConflict, true);
    assert.equal(conflictCheck.errorCode, "CHAIR_OVERLAP_CONFLICT");
    assert.equal(conflictCheck.conflictingAppointmentId, "apt-101");
  });

  it("ignores CANCELADO or NO_SHOW appointments when evaluating physical chair availability (RN-03)", () => {
    const cancelledList: CalendarAppointmentDTO[] = [
      {
        ...existingAppointments[0],
        status: "CANCELADO",
      },
    ];

    const check = detectChairOverlapConflict(cancelledList, {
      chairId: "chair-1",
      dentistId: "dent-2",
      date: "2026-10-30",
      startTime: "09:15",
      endTime: "09:45",
    });
    assert.equal(check.hasConflict, false);
  });

  it("enforces the 7-state appointment lifecycle graph strictly (RF-10)", () => {
    assert.deepEqual(getAllowedStatusTransitions("PROGRAMADO"), [
      "CONFIRMADO",
      "EN_SALA_DE_ESPERA",
      "CANCELADO",
      "NO_SHOW",
    ]);
    assert.deepEqual(getAllowedStatusTransitions("CONFIRMADO"), [
      "EN_SALA_DE_ESPERA",
      "CANCELADO",
      "NO_SHOW",
    ]);
    assert.deepEqual(getAllowedStatusTransitions("EN_SALA_DE_ESPERA"), [
      "EN_ATENCION",
      "CANCELADO",
      "NO_SHOW",
    ]);
    assert.deepEqual(getAllowedStatusTransitions("EN_ATENCION"), ["ATENDIDO"]);
    assert.deepEqual(getAllowedStatusTransitions("ATENDIDO"), []);

    assert.equal(canTransitionAppointmentStatus("PROGRAMADO", "ATENDIDO"), false);
    assert.equal(canTransitionAppointmentStatus("EN_SALA_DE_ESPERA", "EN_ATENCION"), true);
  });

  it("renders calendar surface with tabular-nums and RN-03 alert banner when overlap is present", () => {
    const html = renderMultiChairCalendarHtml({
      appointments: existingAppointments,
      filter: { view: "day", date: "2026-10-30" },
      overlapError: "CHAIR_OVERLAP_CONFLICT: Sillón 1 ocupado entre 09:00 y 10:00",
    });
    assert.match(html, /tabular-nums/);
    assert.match(html, /CHAIR_OVERLAP_CONFLICT/);
    assert.match(html, /role="alert"/);
  });
});

describe("wi-13 S8 — real appointment state transition control (RF-10, RN-03)", () => {
  const appointments: CalendarAppointmentDTO[] = [
    {
      id: "apt-101",
      patientId: "pat-001",
      patientName: "María Fernanda Gómez",
      chairId: "chair-1",
      chairLabel: "Sillón 1 - Cirugía",
      dentistId: "dent-1",
      dentistName: "Dr. Alejandro Vega",
      date: "2026-10-30",
      startTime: "09:00",
      endTime: "10:00",
      status: "CONFIRMADO",
      procedureSummary: "Implante Pieza 16",
    },
  ];

  it("keeps the decorative select when the transition hook is omitted", () => {
    const html = renderMultiChairCalendarHtml({
      appointments,
      filter: { view: "day", date: "2026-10-30" },
    });

    assert.match(html, /<select aria-label="Cambiar estado de cita"/);
    assert.match(html, /<option value="EN_SALA_DE_ESPERA">EN_SALA_DE_ESPERA<\/option>/);
    assert.equal(html.includes('action="/agenda/transition"'), false);
  });

  it("replaces the decorative select with the markup of the transition hook", () => {
    const html = renderMultiChairCalendarHtml({
      appointments,
      filter: { view: "day", date: "2026-10-30" },
      transitionControlHtml: (appointment) =>
        `<form method="post" action="/agenda/transition">
          <input type="hidden" name="appointmentId" value="${appointment.id}" />
        </form>`,
    });

    assert.equal(html.includes('action="/agenda/transition"'), true);
    assert.equal(html.includes('name="appointmentId" value="apt-101"'), true);
    assert.equal(html.includes('aria-label="Cambiar estado de cita"'), false);
  });

  it("gives the hook access to the allowed transitions of the appointment", () => {
    const seen: string[][] = [];

    renderMultiChairCalendarHtml({
      appointments,
      filter: { view: "day", date: "2026-10-30" },
      transitionControlHtml: (appointment) => {
        seen.push(getAllowedStatusTransitions(appointment.status));
        return "<form></form>";
      },
    });

    assert.deepEqual(seen, [["EN_SALA_DE_ESPERA", "CANCELADO", "NO_SHOW"]]);
  });
});
