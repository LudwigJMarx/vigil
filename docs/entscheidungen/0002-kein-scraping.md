# 0002: vigil makes no request to LinkedIn

Decided 2026-09-17. Status: accepted.

## Problem

The obvious product reads the user's feed on a timer and surfaces what matters.
That is what the commercial tools in this category do, and it is what the first
issue asking for a feature will ask for.

## Decision

vigil never makes a request to LinkedIn. The server does not, and the extension
does not. The extension reads `location.href`, `document.title` and the current
text selection of the tab the user is on, at the moment the user clicks the
icon, under `activeTab` with no host permissions.

## Consequences

The product is a memory and a ranking, not a crawler. Capture costs a click.
That is a real reduction in what vigil can promise, and
[docs/scope.md](../scope.md) says so in the user's own words rather than hiding
it behind a roadmap.

In exchange: an operator cannot get their LinkedIn account restricted by
running vigil, there is no rate limiter to evade, no session cookie to store and
no selector to maintain against markup that changes without notice.

## Enforcement

A decision that lives only in a document is a decision until the next pull
request. `scripts/pruefe-keine-hintergrundabfrage.py` runs in CI and fails on a
host permission, an `alarms` permission, a `background` section, a
`content_scripts` entry with `matches`, a `setInterval`, a `chrome.alarms` call
or a `fetch` against a linkedin.com URL. Its own tests build trees containing
each of those and insist they are found.

The checker reads text, not behaviour. Someone determined to get past it can,
through a computed string. It stops the change made out of convenience, which is
the one that actually happens.

## What was rejected

**Reading the feed from the content script while the user has it open.** It
looks like a small step from "read the page the user opened". It is not: it
collects items the user did not look at, which is automated collection with an
extra click in front of it.

**Bundling feed connectors.** RSS, job boards and funding databases are fine to
poll, but the licence situation differs per feed and vigil cannot know the
operator's. `POST /api/v1/ingest` is open for a cron job the operator writes.
