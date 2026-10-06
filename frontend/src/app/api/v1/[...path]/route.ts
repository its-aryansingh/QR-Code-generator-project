/**
 * Same-origin proxy: /api/v1/*  ->  ${BACKEND_INTERNAL_URL}/api/v1/*
 *
 * The backend address is read per request at runtime, so one image works in
 * every environment. On Railway set, on the frontend service:
 *   BACKEND_INTERNAL_URL=http://${{backend.RAILWAY_PRIVATE_DOMAIN}}:${{backend.PORT}}
 *
 * Redirects are passed through untouched (the OAuth start/callback endpoints
 * answer with 302s that the browser must follow), every Set-Cookie header is
 * forwarded, and X-Forwarded-Proto/For/Host tell Django the original scheme,
 * client IP and host.
 */
import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const HOP_BY_HOP = new Set([
  "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "proxy-connection",
  "te", "trailer", "transfer-encoding", "upgrade",
]);
// Never forwarded upstream: the backend gets its own Host, and the body length
// is recomputed. Never forwarded downstream: fetch already decoded the body.
const DROP_REQUEST = new Set(["host", "content-length", "accept-encoding"]);
const DROP_RESPONSE = new Set(["content-encoding", "content-length", "set-cookie"]);

function backendBase(): string {
  const raw =
    process.env.BACKEND_INTERNAL_URL ||
    process.env.BACKEND_URL ||
    "http://127.0.0.1:8084";
  return raw.replace(/\/+$/, "").replace(/\/api\/v1$/, "");
}

async function proxy(request: NextRequest): Promise<Response> {
  const incoming = request.nextUrl;
  const target = `${backendBase()}${incoming.pathname}${incoming.search}`;

  const headers = new Headers();
  request.headers.forEach((value, key) => {
    const k = key.toLowerCase();
    if (!HOP_BY_HOP.has(k) && !DROP_REQUEST.has(k)) headers.set(key, value);
  });
  // Railway's edge terminates TLS and sets these on the request we receive;
  // fall back to what this server saw for local development.
  const proto = request.headers.get("x-forwarded-proto") || incoming.protocol.replace(":", "");
  headers.set("x-forwarded-proto", proto.split(",")[0].trim());
  headers.set("x-forwarded-host", request.headers.get("x-forwarded-host") || request.headers.get("host") || incoming.host);
  headers.set("accept-encoding", "identity");

  const method = request.method.toUpperCase();
  const hasBody = !["GET", "HEAD"].includes(method);

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method,
      headers,
      body: hasBody ? await request.arrayBuffer() : undefined,
      redirect: "manual",
      cache: "no-store",
      signal: AbortSignal.timeout(120_000),
    });
  } catch (error) {
    console.error(`[api-proxy] ${method} ${incoming.pathname} -> ${target} failed:`, error);
    return Response.json(
      {
        success: false,
        error: "The API is unreachable right now. Please try again in a moment.",
        code: "bad_gateway",
      },
      { status: 502 },
    );
  }

  const out = new Headers();
  upstream.headers.forEach((value, key) => {
    const k = key.toLowerCase();
    if (!HOP_BY_HOP.has(k) && !DROP_RESPONSE.has(k)) out.set(key, value);
  });
  for (const cookie of upstream.headers.getSetCookie()) out.append("set-cookie", cookie);

  const bodyless = method === "HEAD" || [101, 204, 205, 304].includes(upstream.status);
  return new Response(bodyless ? null : upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: out,
  });
}

export const GET = proxy;
export const HEAD = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const OPTIONS = proxy;
