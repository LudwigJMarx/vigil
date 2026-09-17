# vigil

Watch the accounts you sell to. Score what you captured. Write the message
yourself.

vigil is an open source, self-hosted signal layer for long-cycle B2B sales. It
keeps a timeline per account, weighs each observation by kind and age, and
ranks your book by who is worth a message this week. It is one binary and one
SQLite file. It sends nothing on your behalf and it crawls nothing.

The problem it addresses: in a deal that takes months, the moment to write is
visible days before it passes, and it is visible in things you already saw and
forgot. vigil is the place you put them, and the ranking that brings them back
at the right time.

## What it does not do

The absences are the design, not a backlog.

| Not built | Reason |
|---|---|
| Automated outreach | The message that closes a six-figure deal is written by the person who understood the moment. vigil has no send path at all. |
| Crawling LinkedIn | LinkedIn's terms forbid automated collection, and the account that gets restricted is the user's. The extension holds no host permissions, registers no alarms and can read only the tab you are on, in the moment you click. See [docs/scope.md](docs/scope.md). |
| A hosted service | Your pipeline is your pipeline. There is no account to create and no server of ours to trust. |
| Scraping page markup | LinkedIn's markup is generated and changes without notice. A selector that silently stops matching makes an account look quiet, which is worse than making it look broken. v1 reads the URL, the title and your selection. |
| Telemetry | vigil makes no outbound request that you did not trigger. |

## Install

Three ways, in order of how little you have to have installed.

**A released binary.** Download the archive for your platform from the
[releases page](https://github.com/LudwigJMarx/vigil/releases), check it
against `checksums.txt`, unpack, and move `vigil` onto your `PATH`. There is
nothing else to install: no runtime, no libraries, no database server.

**With Go 1.26 or newer:**

```bash
go install github.com/LudwigJMarx/vigil/cmd/vigil@latest
```

**From source:**

```bash
git clone https://github.com/LudwigJMarx/vigil
cd vigil
make build          # produces ./vigil
```

## Five minutes to a running instance

```bash
vigil token create --name browser
```

```
vgl_<43 base64url characters, redacted here>
issued token tok_… (browser). It is stored as a hash: this is the only time it is printed.
```

The value above is redacted; a real one is 47 characters. The token goes to
stdout on its own line, and the explanation to stderr, so
`vigil token create --name browser > token.txt` captures the credential and
nothing else. It is stored as a SHA-256 hash. Losing it means issuing another
one.

```bash
vigil serve
```

```
vigil 0.1.0 listening on http://127.0.0.1:8099
database: /home/you/.local/share/vigil/vigil.db
```

Open <http://127.0.0.1:8099>, paste the token, and you have the operator UI.
Check the instance without a credential at any time:

```bash
curl -s http://127.0.0.1:8099/healthz
```

```json
{"status":"ok","version":"0.1.0","schema_version":1,"accounts":0,"signals":0,"active_tokens":1}
```

`/healthz` names what it counted rather than answering "ok". A process that is
up with an unreadable database and one that is up with an empty database are
different problems.

Send a signal:

```bash
curl -s -X POST http://127.0.0.1:8099/api/v1/ingest \
  -H "Authorization: Bearer $VIGIL_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "account": {"name": "Acme GmbH", "profile": "https://www.linkedin.com/company/acme-gmbh/"},
    "person":  {"name": "Jane Doe", "profile": "https://www.linkedin.com/in/jane-doe/"},
    "signals": [{"kind": "job_change", "source": "linkedin",
                 "title": "Jane Doe started as Head of Operations",
                 "url": "https://www.linkedin.com/in/jane-doe/",
                 "occurred_at": "2026-09-10T09:00:00Z"}]
  }'
```

```json
{"account_id":"acc_1d05…","person_id":"per_e99d…","received":1,"inserted":1,"duplicate":0,"rejected":[],"signal_ids":["sig_c868…"]}
```

Send it again from a German locale URL with a tracking parameter and a
timestamp that drifted by an hour, and vigil recognises it:

```json
{"account_id":"acc_1d05…","received":1,"inserted":0,"duplicate":1,"rejected":[],"signal_ids":["sig_c868…"]}
```

That is the whole contract of the ingest endpoint: it tells you what it did.
"Accepted" would not be an outcome, because a batch that was entirely new and
one that was entirely duplicate would look the same.

## The browser extension

```bash
cd extension
npm install
npm run build
```

Load `extension/` as an unpacked extension in `chrome://extensions`, open its
options page, and enter the server URL and the token. Details and the list of
things it deliberately cannot do are in [extension/README.md](extension/README.md).

## How the score works

Each signal kind carries a weight and a half-life. A signal contributes
`weight * 0.5^(age_in_days / half_life)`. The account score is the sum.

| Kind | Weight | Half-life | Why |
|---|---:|---:|---|
| `job_change` | 25 | 60 d | New role, new budget, no incumbent loyalty yet |
| `funding` | 20 | 90 d | Money raised is money about to be spent |
| `competitor_interaction` | 18 | 21 d | The cheapest early warning there is |
| `hiring` | 12 | 45 d | A role opened is a problem stated in public |
| `profile_view` | 8 | 7 d | They looked at you: rarest signal, shortest lived |
| `post` | 6 | 14 d | What they chose to say this week |
| `comment` | 4 | 14 d | Weaker than a post: the topic was someone else's |
| `reaction` | 1 | 10 d | Near noise alone, useful in volume |
| `note` | 0 | 365 d | A human note belongs in the timeline, not in the score |

These are an opening position, not measured truth. Change them in the UI or
through `PUT /api/v1/rules/{kind}`; the defaults are never copied into the
database, so an instance you never touch keeps following later releases.

Every score comes back with the contributions it is made of. A number nobody
can take apart is a number nobody can argue with, and a salesperson deciding
whether to write today needs to argue with it.

A kind with no enabled rule is still stored and appears under
`unscored_kinds`. Without that, "this account is quiet" and "the scoring model
understood none of what you captured" would be the same empty list.

## Configuration

| Flag | Environment | Default | Meaning |
|---|---|---|---|
| `--addr` | `VIGIL_ADDR` | `127.0.0.1:8099` | Address to listen on |
| `--db` | `VIGIL_DB` | `$XDG_DATA_HOME/vigil/vigil.db`, else `~/.local/share/vigil/vigil.db` | Database file |
| `--log` | `VIGIL_LOG` | `text` | `text` or `json` |

vigil refuses to start on a non-loopback address while no token exists. An
instance on a public address with no credential is an open database, and a
warning in a log scrolls past.

## Deployment

[docs/deploy.md](docs/deploy.md) has a systemd unit, a reverse proxy
configuration and the backup procedure. The short version: one binary, one
file, one user, no dependencies to install on the host.

## API

Every route below `/api/v1` needs `Authorization: Bearer <token>`.
`/healthz` does not.

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/ingest` | Store a capture: account, optional person, signals |
| `GET` | `/api/v1/accounts` | The book, ranked, with what the ranking read |
| `POST` | `/api/v1/accounts` | Create an account by hand |
| `GET` | `/api/v1/accounts/{id}` | Account, people, timeline, score breakdown |
| `DELETE` | `/api/v1/accounts/{id}` | Delete an account and its signals |
| `POST` | `/api/v1/accounts/{id}/note` | Add a note to the timeline |
| `GET` | `/api/v1/rules` | The scoring model in force |
| `PUT` | `/api/v1/rules/{kind}` | Override one rule |
| `DELETE` | `/api/v1/rules/{kind}` | Drop an override, restoring the default |
| `GET` | `/api/v1/tokens` | Tokens, without their secrets |
| `POST` | `/api/v1/tokens` | Issue a token |
| `DELETE` | `/api/v1/tokens/{id}` | Revoke a token |
| `GET` | `/healthz` | Liveness with counts, no credential needed |

[docs/api.md](docs/api.md) has the request and response shapes.

## Your data

It is in one SQLite file, on your machine. Copy it to back it up, delete it to
be rid of it. vigil has no account system beyond the tokens you issue, sends no
telemetry, and makes no outbound request you did not trigger.

Tokens are stored as SHA-256 hashes. The browser extension keeps its token in
`chrome.storage.local`, never in `sync`, so the credential does not travel to
other browsers signed into the same Google account.

## Development

```bash
make check     # everything the CI runs, in the same order
make test      # Go tests plus the extension's tests
make build
```

The checks are listed in
[.github/workflows/pruefungen.yml](.github/workflows/pruefungen.yml), which is
the binding list. Among them:

- `scripts/pruefer-verdrahtet.py` fails the build if a checker exists that no
  workflow calls. A checker nobody runs reports the same silence as a happy one.
- `scripts/pruefe-keine-hintergrundabfrage.py` fails the build if a host
  permission, an alarm or a periodic fetch appears in the extension.
- `scripts/pruefe-doku-befehle.py` fails the build if this file shows a command
  the binary does not have.
- `scripts/pruefe-lizenzhinweise.py` fails the build if `THIRD-PARTY-NOTICES.md`
  is out of date. `make lizenzen` regenerates it.
- `scripts/pruefe-herkunftszeile.py` fails a pull request whose commits carry no
  `Signed-off-by` line.
- `scripts/pruefe-release-abgesichert.py` fails the build if a job in the release
  workflow can reach the publish step without going through these checks first.

[CONTRIBUTING.md](CONTRIBUTING.md) has the rest, including what a pull request
has to show before it can be reviewed.

## Legal

vigil is not affiliated with, endorsed by or connected to LinkedIn Corporation
or Microsoft Corporation. You are responsible for what you capture and for the
terms of the services you capture it from. [docs/scope.md](docs/scope.md)
explains what vigil does to keep that boundary, and what it cannot do for you.

## Licence

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

Contributions are covered by Apache-2.0 §5 and by the
[Developer Certificate of Origin](DCO): sign off your commits with
`git commit -s`. There is no CLA and no copyright assignment. Details in
[CONTRIBUTING.md](CONTRIBUTING.md).

The Go modules linked into the binary are MIT and BSD-3-Clause. Their copyright
notices and licence texts are in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md),
which is generated from the build and ships inside every release archive.
