import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import {
  cleanTitle,
  displayName,
  draftFrom,
  parseRelativeTime,
  profileFromURL,
  suggestKind,
  toRFC3339,
} from "../src/guess.ts";
import { describe, normaliseServer } from "../src/client.ts";

const tableURL = new URL("../../testdata/profile-normalisation.json", import.meta.url);

test("the URL rule agrees with the table the Go server reads", () => {
  // Two implementations of one rule drift, and the drift is invisible: the
  // server simply starts holding two accounts for one company. The Go side
  // reads this same file in internal/core/shared_table_test.go.
  const table = JSON.parse(readFileSync(fileURLToPath(tableURL), "utf8")) as {
    cases: { input: string; want: string }[];
  };
  assert.ok(table.cases.length > 0, "the shared table holds no cases");

  for (const { input, want } of table.cases) {
    const got = profileFromURL(input)?.profile ?? "";
    assert.equal(got, want, `profileFromURL(${JSON.stringify(input)})`);
  }
});

test("a person URL is told apart from a company URL", () => {
  assert.equal(profileFromURL("https://www.linkedin.com/in/jane-doe")?.kind, "person");
  assert.equal(profileFromURL("https://www.linkedin.com/company/acme")?.kind, "company");
});

test("the suggested kind follows the kind of page, not the content", () => {
  assert.equal(suggestKind("https://www.linkedin.com/jobs/view/4001"), "hiring");
  assert.equal(suggestKind("https://www.linkedin.com/feed/update/urn:li:activity:7100"), "post");
  assert.equal(suggestKind("https://www.linkedin.com/posts/acme_hiring-activity-7100"), "post");
  assert.equal(suggestKind("https://www.linkedin.com/in/jane-doe"), "note");
  assert.equal(suggestKind("not a url"), "note");
});

test("relative times are turned into instants", () => {
  const now = new Date("2026-09-17T12:00:00Z");
  const cases: [string, string][] = [
    ["2h", "2026-09-17T10:00:00Z"],
    ["3d", "2026-09-14T12:00:00Z"],
    ["1w", "2026-09-10T12:00:00Z"],
    ["2mo", "2026-07-19T12:00:00Z"],
    ["3 Tage", "2026-09-14T12:00:00Z"],
    ["2 Wo.", "2026-09-03T12:00:00Z"],
    ["5 Std.", "2026-09-17T07:00:00Z"],
  ];
  for (const [text, want] of cases) {
    const got = parseRelativeTime(text, now);
    assert.ok(got, `${text} was not recognised`);
    assert.equal(toRFC3339(got), want, text);
  }
});

test("an unrecognised time is null, never 'now'", () => {
  // Falling back to now would date a three-month-old post to today, and the
  // coldest account on the list would come out hottest.
  const now = new Date("2026-09-17T12:00:00Z");
  for (const text of ["", "yesterday", "vor kurzem", "0d", "-3d", "12 parsecs",
                      "raised 12 m EUR", "3d ago on a Tuesday"]) {
    assert.equal(parseRelativeTime(text, now), null, JSON.stringify(text));
  }
});

test("the LinkedIn boilerplate is stripped from the title", () => {
  assert.equal(cleanTitle("(3) Acme GmbH | LinkedIn"), "Acme GmbH");
  assert.equal(cleanTitle("Jane Doe - Head of Ops - Acme | LinkedIn"), "Jane Doe - Head of Ops - Acme");
  assert.equal(cleanTitle("  plain  "), "plain");
});

test("a slug becomes a name without LinkedIn's trailing hash", () => {
  assert.equal(displayName("jane-doe"), "Jane Doe");
  assert.equal(displayName("jane-doe-8a3f21b9"), "Jane Doe");
  assert.equal(displayName("acme-gmbh"), "Acme Gmbh");
});

test("a company page fills the account and leaves the person empty", () => {
  const now = new Date("2026-09-17T12:00:00Z");
  const draft = draftFrom(
    {
      url: "https://www.linkedin.com/company/acme-gmbh/",
      title: "(1) Acme GmbH | LinkedIn",
      selection: "",
    },
    now,
  );
  assert.equal(draft.accountProfile, "linkedin.com/company/acme-gmbh");
  assert.equal(draft.accountName, "Acme GmbH");
  assert.equal(draft.personName, "");
  assert.deepEqual(draft.missing, []);
});

test("a person page leaves the account to the human instead of inventing one", () => {
  // An invented account name creates a record that looks real and matches
  // nothing. The form says what it does not know and refuses to send.
  const now = new Date("2026-09-17T12:00:00Z");
  const draft = draftFrom(
    {
      url: "https://www.linkedin.com/in/jane-doe/",
      title: "Jane Doe - Head of Ops - Acme GmbH | LinkedIn",
      selection: "",
    },
    now,
  );
  assert.equal(draft.personProfile, "linkedin.com/in/jane-doe");
  assert.equal(draft.personName, "Jane Doe");
  assert.equal(draft.accountName, "");
  assert.deepEqual(draft.missing, ["account"]);
});

test("a relative time inside the selection sets the date", () => {
  const now = new Date("2026-09-17T12:00:00Z");
  const draft = draftFrom(
    {
      url: "https://www.linkedin.com/company/acme-gmbh/",
      title: "Acme GmbH | LinkedIn",
      selection: "We are moving off SAP.\n3d",
    },
    now,
  );
  assert.equal(toRFC3339(draft.occurredAt), "2026-09-14T12:00:00Z");
  assert.equal(draft.body, "We are moving off SAP.\n3d");
});

test("a number inside prose does not move the date", () => {
  // "raised 12 m EUR" used to be read as twelve minutes ago. A wrong date is
  // worse than no date, because nothing about it looks wrong afterwards.
  const now = new Date("2026-09-17T12:00:00Z");
  const draft = draftFrom(
    {
      url: "https://www.linkedin.com/company/acme-gmbh/",
      title: "Acme GmbH | LinkedIn",
      selection: "Acme raised 12 m EUR in a Series B",
    },
    now,
  );
  assert.equal(toRFC3339(draft.occurredAt), toRFC3339(now));
});

test("a page that is not LinkedIn still produces a usable draft", () => {
  const now = new Date("2026-09-17T12:00:00Z");
  const draft = draftFrom(
    { url: "https://acme.example/news/funding", title: "Acme raises 12M", selection: "" },
    now,
  );
  assert.equal(draft.kind, "note");
  assert.equal(draft.title, "Acme raises 12M");
  assert.deepEqual(draft.missing, ["account"]);
});

test("the server URL loses its trailing slash", () => {
  assert.equal(normaliseServer("http://127.0.0.1:8099/"), "http://127.0.0.1:8099");
  assert.equal(normaliseServer("  http://host:8099///  "), "http://host:8099");
  assert.equal(normaliseServer(""), "");
});

test("the result line says what happened, not that something was sent", () => {
  assert.equal(
    describe({ account_id: "a", received: 2, inserted: 1, duplicate: 1, rejected: [], signal_ids: [] }),
    "1 new, 1 already known",
  );
  assert.equal(
    describe({
      account_id: "a", received: 1, inserted: 0, duplicate: 0,
      rejected: [{ index: 0, reason: "occurred_at is missing" }], signal_ids: [],
    }),
    "1 rejected: occurred_at is missing",
  );
  assert.equal(
    describe({ account_id: "a", received: 0, inserted: 0, duplicate: 0, rejected: [], signal_ids: [] }),
    "nothing was sent",
  );
});
