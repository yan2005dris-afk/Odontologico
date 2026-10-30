/**
 * Appointment State Transition Route Handler (wi-13 S8, RF-10, RN-03).
 *
 * Receives the transition form of `/agenda`, forwards it to the scheduling
 * service through the API Gateway and redirects back to the agenda. A chair
 * overlap conflict (409 CHAIR_OVERLAP_CONFLICT) is reported as such so the
 * portal can show the banner without losing the transition form.
 */

import { redirect } from "next/navigation";
import { createGatewayServerClient } from "../../../../lib/gateway-server.ts";
import { mapTransitionErrorCode } from "../../../../lib/portal-forms.ts";
import { isSessionExpiredError } from "../../../../lib/portal-views.ts";
import { requirePortalSession } from "../../../../lib/session.ts";
import type { AppointmentStatus } from "../../../../lib/session-idle-guard.ts";

const APPOINTMENT_STATUSES: AppointmentStatus[] = [
  "PROGRAMADO",
  "CONFIRMADO",
  "EN_SALA_DE_ESPERA",
  "EN_ATENCION",
  "ATENDIDO",
  "CANCELADO",
  "NO_SHOW",
];

/** Reads a trimmed text field of the submitted form. */
function readField(formData: FormData, name: string): string {
  const value = formData.get(name);
  return typeof value === "string" ? value.trim() : "";
}

export async function POST(request: Request) {
  const session = await requirePortalSession();
  const formData = await request.formData();
  const client = createGatewayServerClient({ accessToken: session.accessToken });

  const appointmentId = readField(formData, "appointmentId");
  const targetStatus = readField(formData, "targetStatus");
  // The date and the view the operator was looking at are carried back to the
  // agenda, so a transition never loses the context of the operator.
  const date = readField(formData, "date");
  const view = readField(formData, "view") === "week" ? "week" : "day";
  const agendaQuery = `date=${encodeURIComponent(date)}&view=${encodeURIComponent(view)}`;

  if (
    appointmentId === "" ||
    !APPOINTMENT_STATUSES.includes(targetStatus as AppointmentStatus)
  ) {
    redirect(`/agenda?${agendaQuery}&error=${encodeURIComponent("INVALID_APPOINTMENT_TRANSITION")}`);
  }

  try {
    await client.transitionAppointment(appointmentId, targetStatus as AppointmentStatus);
  } catch (error) {
    const code = mapTransitionErrorCode(error);

    if (isSessionExpiredError(error)) {
      redirect("/login?reason=idle_timeout");
    }

    if (code === "CHAIR_OVERLAP_CONFLICT") {
      redirect(
        `/agenda?${agendaQuery}` +
          `&conflict=${encodeURIComponent("CHAIR_OVERLAP_CONFLICT")}` +
          `&appointment=${encodeURIComponent(appointmentId)}` +
          `&target=${encodeURIComponent(targetStatus)}`
      );
    }

    redirect(`/agenda?${agendaQuery}&error=${encodeURIComponent(code)}`);
  }

  redirect(`/agenda?${agendaQuery}&updated=1`);
}
