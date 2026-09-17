# HTTP API

Version `v1`. Every route under `/api/v1` requires
`Authorization: Bearer <token>`. `/healthz` does not.

Every failure is `{"error": "..."}` with a matching status. Authentication
failures are all the same 401 with the same body: whether a token was unknown,
revoked or malformed is exactly the hint a guesser wants.

CORS allows every origin. That is safe only because vigil authenticates with a
header and never with a cookie, so a page that has not been given a token gets a
401 regardless of where it calls from. A test fails the build if any response
ever sets a cookie.

## POST /api/v1/ingest

Store one capture. The account is resolved by normalised profile URL when one is
given, so the same company captured from `de.linkedin.com` with a tracking
parameter lands on the account that already exists.

```json
{
  "account": {"name": "Acme GmbH", "profile": "https://www.linkedin.com/company/acme-gmbh/", "stage": "acquire"},
  "person":  {"name": "Jane Doe", "profile": "https://www.linkedin.com/in/jane-doe/", "headline": "Head of Operations"},
  "signals": [
    {"kind": "job_change", "source": "linkedin",
     "title": "Jane Doe started as Head of Operations",
     "body": "",
     "url": "https://www.linkedin.com/in/jane-doe/",
     "occurred_at": "2026-09-10T09:00:00Z"}
  ]
}
```

`account_id` may be sent instead of `account`. `person` is optional. At most
200 signals per request.

```json
{
  "account_id": "acc_1d05…",
  "person_id": "per_e99d…",
  "received": 1,
  "inserted": 1,
  "duplicate": 0,
  "rejected": [],
  "signal_ids": ["sig_c868…"]
}
```

| Field | Meaning |
|---|---|
| `received` | How many signals were in the request |
| `inserted` | How many were new |
| `duplicate` | How many were already stored under the same fingerprint |
| `rejected` | One entry per signal that was not stored, with its index and the reason |

A rejected entry is named, never dropped. The status is 200 while anything at
all was stored, and 400 when nothing was, so a permanently broken capture cannot
look healthy to a caller who does not read the body.

Rejection reasons currently in use:

| Reason | Cause |
|---|---|
| `occurred_at is missing` | No time. vigil does not substitute `now` |
| `occurred_at is more than a day in the future` | A clock or timezone error upstream |

## GET /api/v1/accounts

The book, highest score first.

```json
{
  "accounts": [
    {"id": "acc_1d05…", "name": "Acme GmbH", "profile": "linkedin.com/company/acme-gmbh",
     "stage": "acquire", "created_at": "…", "updated_at": "…",
     "score": 23.02, "top_reason": "job_change", "signals_14d": 1}
  ],
  "scored_at": "2026-09-17T12:12:18Z",
  "signals_read": 1,
  "unscored_kinds": []
}
```

`signals_read` and `unscored_kinds` are the scope of the answer. An empty book
because nothing was captured and an empty book because no rule matched anything
would be the same response without them.

## GET /api/v1/accounts/{id}

Takes `?limit=` (default 200) for the timeline. The limit cuts the returned
list, never the scoring: the score always reads the whole timeline, or an
account would look colder the smaller the page you asked for.

```json
{
  "account": {"id": "acc_1d05…", "name": "Acme GmbH", "…": "…"},
  "people":  [{"id": "per_e99d…", "name": "Jane Doe", "headline": "Head of Operations"}],
  "signals": [{"id": "sig_c868…", "kind": "job_change", "occurred_at": "…", "…": "…"}],
  "score": {
    "account_id": "acc_1d05…",
    "total": 23.02,
    "contributions": [
      {"signal_id": "sig_c868…", "kind": "job_change", "weight": 25,
       "age_days": 7.13, "decay": 0.9209, "points": 23.02}
    ],
    "unscored": [],
    "at": "2026-09-17T12:12:18Z"
  },
  "signals_returned": 1,
  "signals_total": 1
}
```

## POST /api/v1/accounts

```json
{"name": "Acme GmbH", "profile": "https://www.linkedin.com/company/acme-gmbh/", "stage": "acquire"}
```

`stage` is `acquire`, `expand` or `retain`. Returns 201 and the stored account.

## POST /api/v1/accounts/{id}/note

```json
{"body": "Jane asked for a reference customer", "occurred_at": "2026-09-17T09:00:00Z"}
```

`occurred_at` defaults to now. The note lands on the timeline as a signal of
kind `note`, which is weighted zero by default.

## GET /api/v1/rules, PUT /api/v1/rules/{kind}, DELETE /api/v1/rules/{kind}

`GET` returns the model in force: the defaults, with stored overrides replacing
those of the same kind, plus kinds you added that have no default.

```json
{"kind": "post", "weight": 6, "half_life_days": 14, "enabled": true, "note": "what they chose to say this week"}
```

`DELETE` drops an override, and the default applies again. Deleting a kind that
was never overridden is not an error: you asked for "no override", and that is
the state either way.

Defaults are never written into the database. A default copied into a table
stops being a default and freezes at the value of the release that copied it.

## GET, POST, DELETE /api/v1/tokens

`POST {"name": "browser"}` returns 201 with the token and the secret. The secret
appears in that one response and nowhere else; the database holds a SHA-256 of
it. `DELETE /api/v1/tokens/{id}` revokes, and the token stops working on the
next request.

## GET /healthz

No credential. Returns 503 with `{"status": "database unreachable"}` when the
file behind the handle has gone away.

```json
{"status":"ok","version":"0.1.0","schema_version":1,"accounts":0,"signals":0,"active_tokens":1}
```
