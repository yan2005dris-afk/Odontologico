/**
 * Patient Intake Route Handler (wi-13 S6, RF-01).
 *
 * Receives the registration form of `/patients`, forwards it to the clinical
 * service through the API Gateway and redirects back to the directory with a
 * deterministic outcome. A success is never fabricated: any upstream failure
 * becomes an explicit error code in the redirect target.
 */

import { redirect } from "next/navigation";
import { createGatewayServerClient } from "../../../../lib/gateway-server.ts";
import {
  mapCreatePatientErrorCode,
  splitPatientFullName,
} from "../../../../lib/portal-forms.ts";
import { isSessionExpiredError } from "../../../../lib/portal-views.ts";
import { requirePortalSession } from "../../../../lib/session.ts";

const DOCUMENT_TYPES = new Set(["DNI", "PASSPORT", "FOREIGN_ID"]);
const SEXES = new Set(["F", "M", "X"]);

/** Reads a trimmed text field of the submitted form. */
function readField(formData: FormData, name: string): string {
  const value = formData.get(name);
  return typeof value === "string" ? value.trim() : "";
}

export async function POST(request: Request) {
  const session = await requirePortalSession();
  const formData = await request.formData();
  const client = createGatewayServerClient({ accessToken: session.accessToken });

  const documentType = readField(formData, "documentType").toUpperCase();
  const sex = readField(formData, "sex").toUpperCase();

  if (!DOCUMENT_TYPES.has(documentType) || !SEXES.has(sex)) {
    redirect(`/patients?error=${encodeURIComponent("VALIDATION_ERROR")}`);
  }

  const { firstName, lastName } = splitPatientFullName(readField(formData, "fullName"));

  const emergencyContact = {
    fullName: readField(formData, "emergencyContactFullName"),
    phone: readField(formData, "emergencyContactPhone"),
    relationship: readField(formData, "emergencyContactRelationship"),
  };

  const guardianFullName = readField(formData, "guardianFullName");
  const guardianDocumentNumber = readField(formData, "guardianDocumentNumber");
  const guardianPhone = readField(formData, "guardianPhone");
  const guardianRelationship = readField(formData, "guardianRelationship");
  const hasGuardian =
    guardianFullName !== "" ||
    guardianDocumentNumber !== "" ||
    guardianPhone !== "" ||
    guardianRelationship !== "";

  try {
    await client.createPatient({
      documentType,
      documentNumber: readField(formData, "documentNumber"),
      firstName,
      lastName,
      birthDate: readField(formData, "birthDate"),
      sex,
      phone: readField(formData, "phone"),
      email: readField(formData, "email"),
      emergencyContact,
      legalGuardian: hasGuardian
        ? {
            fullName: guardianFullName,
            documentNumber: guardianDocumentNumber,
            phone: guardianPhone,
            relationship: guardianRelationship,
          }
        : undefined,
    });
  } catch (error) {
    if (isSessionExpiredError(error)) {
      redirect("/login?reason=idle_timeout");
    }
    const code = mapCreatePatientErrorCode(error);
    redirect(`/patients?error=${encodeURIComponent(code)}`);
  }

  redirect("/patients?created=1");
}
