/**
 * Portal Home (wi-13 S2).
 *
 * The portal has no dashboard yet: the entry point sends the operator to the
 * patient directory, which is where the clinical work starts. The portal layout
 * already resolved the session, so an operator without a session was sent to
 * the login screen before reaching this page.
 */

import { redirect } from "next/navigation";

export default function PortalHomePage() {
  redirect("/patients");
}
