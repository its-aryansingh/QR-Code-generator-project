/**
 * Scans of dynamic QR codes: /r/<code> is the address printed in the code.
 * The API decides where it goes (and counts the scan); this only forwards.
 */
import { proxyToBackend } from "@/lib/backend-proxy";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export const GET = proxyToBackend;
export const HEAD = proxyToBackend;
// Password-protected codes post their form back to the same address.
export const POST = proxyToBackend;
