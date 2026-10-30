/**
 * Multi-Chair Clinical Agenda Page (wi-10 S4 & S5, RF-09, RF-10, RN-03).
 */

import {
  renderMultiChairCalendarHtml,
  type CalendarAppointmentDTO,
} from "../../../components/agenda/MultiChairCalendar.tsx";
import type { AgendaFilterState } from "../../../lib/session-idle-guard.ts";

export function renderAgendaPortalPageHtml(props: {
  appointments: CalendarAppointmentDTO[];
  filter: AgendaFilterState;
  overlapError?: string;
}): string {
  return `<div class="p-6">
    ${renderMultiChairCalendarHtml(props)}
  </div>`;
}

export default renderAgendaPortalPageHtml;
