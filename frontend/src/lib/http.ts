/**
 * Reads a fetch response as JSON without throwing.
 *
 * An error page from a proxy or server is HTML, and `response.json()` on it
 * throws "Unexpected token '<'", which crashed whole dashboard pages. This
 * returns the dashboard's `{success: false, error}` envelope instead, so code
 * that already checks `success` treats it like any other failed request.
 *
 * Typed exactly like `Response.json()` so it is a drop-in replacement; new
 * code should use the typed client in `lib/enterprise.ts` instead.
 */
export async function safeJson(response: Response): ReturnType<Response["json"]> {
  const text = await response.text();
  if (!text) return response.ok ? {} : failure(response);
  try {
    return JSON.parse(text);
  } catch {
    return failure(response);
  }
}

function failure(response: Response) {
  return {
    success: false,
    status: response.status,
    error: response.ok
      ? "The server sent a response the app couldn't read."
      : `Request failed (${response.status})`,
  };
}
