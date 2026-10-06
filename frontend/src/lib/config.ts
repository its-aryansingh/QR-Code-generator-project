/**
 * Where the browser sends API calls.
 *
 * By default this is the same-origin proxy at `/api/v1/*`
 * (src/app/api/v1/[...path]/route.ts), which forwards to the backend named by
 * the runtime variable BACKEND_INTERNAL_URL. Nothing about the backend is baked
 * into the bundle, so a Railway build can never ship with `localhost` in it,
 * no CORS preflight is needed, and the OAuth state cookie and callback live on
 * the site the user is actually on.
 *
 * NEXT_PUBLIC_API_URL still overrides this for setups that serve the API from
 * another origin, but it is read at build time, so leave it unset on Railway.
 */
export const API_URL = (process.env.NEXT_PUBLIC_API_URL || "/api/v1").replace(/\/$/, "");

/** OAuth always goes through the same-origin proxy, whatever API_URL is. */
export const OAUTH_URL = "/api/v1/auth/oauth";
