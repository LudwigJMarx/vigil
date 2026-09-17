// The shapes the vigil API speaks. Kept by hand rather than generated: the
// surface is small, and a generator is a build step that has to be right
// before anyone can read the code.

export type Stage = "acquire" | "expand" | "retain";

export interface SignalDraft {
  kind: string;
  source: string;
  title?: string;
  body?: string;
  url?: string;
  /** RFC 3339, always with an explicit offset. */
  occurred_at: string;
}

export interface IngestRequest {
  account_id?: string;
  account?: { name: string; profile?: string; domain?: string; stage?: Stage };
  person?: { name: string; profile?: string; headline?: string };
  signals: SignalDraft[];
}

export interface Rejection {
  index: number;
  kind?: string;
  reason: string;
}

export interface IngestResponse {
  account_id: string;
  person_id?: string;
  received: number;
  inserted: number;
  duplicate: number;
  rejected: Rejection[];
  signal_ids: string[];
}

/** What the content script reads off the page. Three fields that do not move. */
export interface PageFacts {
  url: string;
  title: string;
  selection: string;
}

export interface Settings {
  server: string;
  token: string;
}
