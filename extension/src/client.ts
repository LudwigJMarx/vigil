import type { IngestRequest, IngestResponse, Settings } from "./types.ts";

export class VigilError extends Error {}

/** normaliseServer trims the trailing slash so "http://host:8099/" and
 *  "http://host:8099" do not produce "//api/v1/ingest". */
export function normaliseServer(raw: string): string {
  return raw.trim().replace(/\/+$/, "");
}

/**
 * send posts one capture. It reports what the server said it did rather than
 * "sent": a capture that was entirely duplicate and one that was entirely new
 * are both 200, and the person clicking the button wants to know which.
 */
export async function send(settings: Settings, request: IngestRequest): Promise<IngestResponse> {
  const server = normaliseServer(settings.server);
  if (server === "") throw new VigilError("No server configured. Open the options page.");
  if (settings.token.trim() === "") throw new VigilError("No token configured. Open the options page.");

  let response: Response;
  try {
    response = await fetch(`${server}/api/v1/ingest`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${settings.token.trim()}`,
      },
      body: JSON.stringify(request),
    });
  } catch (cause) {
    // A refused connection and a wrong token look the same in the popup
    // otherwise, and they need different fixes.
    throw new VigilError(`Cannot reach ${server}. Is vigil running?`, { cause });
  }

  if (response.status === 401) throw new VigilError("The server refused this token.");

  const body = (await response.json().catch(() => null)) as IngestResponse | { error?: string } | null;
  if (!response.ok) {
    const message = body && "error" in body && body.error ? body.error : `HTTP ${response.status}`;
    throw new VigilError(message);
  }
  if (!body || !("inserted" in body)) {
    throw new VigilError("The server answered something vigil does not understand.");
  }
  return body;
}

/** describe turns a result into the one line the popup shows. */
export function describe(result: IngestResponse): string {
  const parts: string[] = [];
  if (result.inserted > 0) parts.push(`${result.inserted} new`);
  if (result.duplicate > 0) parts.push(`${result.duplicate} already known`);
  if (result.rejected.length > 0) {
    parts.push(`${result.rejected.length} rejected: ${result.rejected.map((r) => r.reason).join("; ")}`);
  }
  if (parts.length === 0) parts.push("nothing was sent");
  return parts.join(", ");
}
