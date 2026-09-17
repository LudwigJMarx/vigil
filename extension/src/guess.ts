// Everything in this file is a pure function of its arguments. That is what
// makes it the only part of the extension with real tests: it needs no browser,
// no network and no LinkedIn account to run.

import type { PageFacts, Stage } from "./types.ts";

export const SIGNAL_KINDS = [
  "post",
  "comment",
  "reaction",
  "job_change",
  "hiring",
  "funding",
  "competitor_interaction",
  "profile_view",
  "note",
] as const;

export type SignalKind = (typeof SIGNAL_KINDS)[number];

export interface ProfileRef {
  kind: "person" | "company" | "school" | "showcase";
  slug: string;
  /** The form vigil stores: "linkedin.com/in/jane-doe". */
  profile: string;
}

/**
 * profileFromURL mirrors NormalizeProfile in internal/core/identity.go. The two
 * have to agree, because the server keys accounts on the result: if the
 * extension sends a form the server normalises differently, one company becomes
 * two accounts. The Go tests and the tests here use the same table for that
 * reason.
 */
export function profileFromURL(raw: string): ProfileRef | null {
  let parsed: URL;
  try {
    parsed = new URL(raw.includes("//") ? raw : `https://${raw}`);
  } catch {
    return null;
  }

  const host = parsed.hostname.toLowerCase();
  if (host !== "linkedin.com") {
    if (!host.endsWith(".linkedin.com")) return null;
    const sub = host.slice(0, -".linkedin.com".length);
    if (sub === "" || sub.includes(".")) return null;
  }

  const parts = parsed.pathname.split("/").filter((p) => p !== "");
  if (parts.length < 2) return null;

  const segment = (parts[0] ?? "").toLowerCase();
  const kind = (
    { in: "person", company: "company", school: "school", showcase: "showcase" } as const
  )[segment];
  if (!kind) return null;

  const slug = (parts[1] ?? "").trim().toLowerCase();
  if (slug === "") return null;

  return { kind, slug, profile: `linkedin.com/${segment}/${slug}` };
}

/**
 * suggestKind proposes a signal kind from the page the user is on. It is a
 * suggestion and the form lets the user override it: the page tells you what
 * you are reading, not what it means.
 */
export function suggestKind(url: string): SignalKind {
  let path: string;
  try {
    path = new URL(url.includes("//") ? url : `https://${url}`).pathname.toLowerCase();
  } catch {
    return "note";
  }
  if (path.startsWith("/jobs/")) return "hiring";
  if (path.startsWith("/feed/update/") || path.startsWith("/posts/")) return "post";
  if (path.startsWith("/pulse/")) return "post";
  if (path.startsWith("/in/")) return "note";
  if (path.startsWith("/company/") || path.startsWith("/showcase/")) return "note";
  return "note";
}

/**
 * displayName turns a profile slug into something a human recognises, for the
 * moment where the page title is unusable. "jane-doe-1234b" becomes
 * "Jane Doe": the trailing hash LinkedIn appends is dropped, because it is not
 * part of anyone's name.
 */
export function displayName(slug: string): string {
  return slug
    .split("-")
    .filter((part) => part !== "" && !/^[0-9a-f]{5,}$/.test(part))
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

/**
 * cleanTitle strips the boilerplate LinkedIn appends to every document title,
 * so the captured title is the thing and not the site.
 */
export function cleanTitle(title: string): string {
  return title
    .replace(/\s*\|\s*LinkedIn\s*$/i, "")
    .replace(/^\(\d+\)\s*/, "") // the unread-notification counter
    .trim();
}

const RELATIVE = new Map<string, number>([
  // English, as LinkedIn abbreviates it.
  ["s", 1], ["m", 60], ["h", 3600], ["d", 86400], ["w", 604800],
  ["mo", 2592000], ["y", 31536000],
  // German, because the interface follows the account language and a German
  // account is the common case for the person this was built for.
  ["sek", 1], ["min", 60], ["std", 3600], ["t", 86400], ["tag", 86400],
  ["tage", 86400], ["tagen", 86400], ["wo", 604800], ["woche", 604800],
  ["mon", 2592000], ["j", 31536000],
]);

/**
 * parseRelativeTime turns "2d", "3 Wo." or "1 mo" into an absolute instant.
 *
 * It returns null for anything it does not recognise, and the caller keeps the
 * date the user typed instead. Guessing "now" for an unparsed string would date
 * a three-month-old post to today, which is the one error that makes a cold
 * account look like the hottest one on the list.
 */
export function parseRelativeTime(text: string, now: Date): Date | null {
  // Anchored on purpose. An unanchored search finds "12 m" in "raised 12 m
  // EUR" and dates the signal twelve minutes ago, which is a wrong date that
  // nothing flags. LinkedIn renders the timestamp as its own element, so the
  // caller passes it as its own string; draftFrom tries the lines of a
  // selection one by one rather than the whole blob.
  const match = /^(\d+)\s*([a-zA-ZäöüÄÖÜ]{1,6})\.?$/.exec(text.trim());
  if (!match) return null;

  const amount = Number(match[1] ?? "");
  if (!Number.isFinite(amount) || amount <= 0) return null;

  const unit = (match[2] ?? "").toLowerCase();
  const seconds = RELATIVE.get(unit);
  if (seconds === undefined) return null;

  return new Date(now.getTime() - amount * seconds * 1000);
}

export interface Draft {
  accountName: string;
  accountProfile: string;
  personName: string;
  personProfile: string;
  kind: SignalKind;
  title: string;
  body: string;
  url: string;
  occurredAt: Date;
  stage: Stage;
  /** What the extension could not work out and the human has to supply. */
  missing: string[];
}

/**
 * draftFrom builds the form the user sees. It never invents an account name:
 * when the page does not say which company this belongs to, `missing` says so
 * and the account field stays empty. The field is `required`, so the browser
 * refuses the submit; the button itself is not disabled. Checked in a browser
 * on 17.09.2026, because the comment here used to claim the button was.
 *
 * An invented name creates an account that looks real and matches nothing.
 */
export function draftFrom(facts: PageFacts, now: Date): Draft {
  const ref = profileFromURL(facts.url);
  const title = cleanTitle(facts.title);
  const selection = facts.selection.trim();

  const draft: Draft = {
    accountName: "",
    accountProfile: "",
    personName: "",
    personProfile: "",
    kind: suggestKind(facts.url),
    title,
    body: selection,
    url: facts.url,
    occurredAt: now,
    stage: "acquire",
    missing: [],
  };

  if (ref?.kind === "person") {
    draft.personProfile = ref.profile;
    draft.personName = title.split(" - ")[0]?.trim() || displayName(ref.slug);
  } else if (ref) {
    draft.accountProfile = ref.profile;
    draft.accountName = title || displayName(ref.slug);
  }

  for (const line of [selection, ...selection.split("\n")]) {
    const parsed = parseRelativeTime(line, now);
    if (parsed) {
      draft.occurredAt = parsed;
      break;
    }
  }

  if (draft.accountName === "") {
    draft.missing.push("account");
  }
  return draft;
}

/** toRFC3339 renders a date the way the API expects it: UTC, to the second. */
export function toRFC3339(date: Date): string {
  return date.toISOString().replace(/\.\d{3}Z$/, "Z");
}
