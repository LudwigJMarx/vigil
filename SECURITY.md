# Security

## Reporting

Use GitHub's private vulnerability reporting on this repository
("Security" tab, "Report a vulnerability"). Do not open a public issue.

Expect an acknowledgement within 72 hours and an assessment within 7 days. If
you have had no reply after 7 days, open a public issue saying only that you are
waiting on a private report, with no detail.

I am one person, not a team with a rota. Say so in the report if you have a
disclosure deadline, and it will be respected.

## Scope

In scope: authentication and token handling, the ingest path, SQL construction,
the operator UI, the browser extension, and anything that makes vigil issue a
request the operator did not trigger.

Out of scope: a vigil instance the operator has published to the internet
without a reverse proxy and without TLS. vigil warns about that at startup and
refuses it outright when no token exists.

## What vigil already does about the obvious things

| Concern | Measure |
|---|---|
| Token storage | SHA-256, never stored in clear. The secret is returned once, at creation |
| Token comparison | Constant-time, even though the lookup is by hash |
| Authentication errors | One 401 with one body for unknown, revoked and malformed |
| Default bind | `127.0.0.1:8099`. A non-loopback bind with no token issued is refused |
| SQL | Bound parameters throughout. The one formatted statement is `PRAGMA user_version = N` with an `int` from a loop counter, because PRAGMA takes no parameters |
| CORS | Wildcard origin, header authentication only. A test fails the build if any response ever sets a cookie |
| UI | `Content-Security-Policy: default-src 'self'` with no inline script, so an injected script cannot read the token out of `localStorage` and post it away |
| Request size | Bodies capped at 2 MiB, batches at 200 signals |
| Extension credentials | `chrome.storage.local`, never `sync`, so the token does not travel to other browsers on the same Google account |
| Extension reach | No host permissions, no background worker. It can read the active tab, at the moment of a click, and nothing else |

## Known limits, stated rather than implied

- The UI keeps the token in `localStorage`. Anything with script execution on
  that origin can read it. The policy above is what stands between those two
  facts.
- `scripts/pruefe-keine-hintergrundabfrage.py` reads text, not behaviour. A
  computed string gets past it. It stops the change made out of convenience.
- vigil speaks plain HTTP by design. TLS belongs to the reverse proxy, and
  [docs/deploy.md](docs/deploy.md) shows one.
