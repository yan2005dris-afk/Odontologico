/**
 * Pure helpers of the portal form route handlers (wi-13 S6/S8).
 *
 * They live outside the `route.ts` files because Next.js only accepts the HTTP
 * method handlers (and the documented route config exports) there.
 */

import { GatewayError } from "./gateway-server.ts";

/** Splits the full name into first name and surnames (first token / rest). */
export function splitPatientFullName(fullName: string): {
  firstName: string;
  lastName: string;
} {
  const parts = fullName.split(/\s+/).filter((part) => part !== "");
  return {
    firstName: parts[0] ?? "",
    lastName: parts.slice(1).join(" "),
  };
}

/** Maps an upstream failure to the error code shown by the portal. */
export function mapCreatePatientErrorCode(error: unknown): string {
  if (error instanceof GatewayError && error.code !== "") {
    return error.code;
  }
  return "PATIENT_CREATION_FAILED";
}

/** Maps an upstream failure to the error code shown by the agenda. */
export function mapTransitionErrorCode(error: unknown): string {
  if (error instanceof GatewayError && error.code !== "") {
    return error.code;
  }
  return "APPOINTMENT_TRANSITION_FAILED";
}
