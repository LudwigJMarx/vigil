# Contributing

Thank you for looking. What follows is the bar a change has to clear. It is
higher than usual in one specific way, and lower than usual in every other.

## The bar

**A pull request has to let a stranger reproduce, rebuild and disprove the
change without asking you anything.** Concretely:

1. **The reproduction comes first.** A command or a test that shows the
   problem, before any diagnosis. No change is made without a reproduced
   failure, even when the cause looks obvious.
2. **The failing test is in the pull request.** It is red before the fix and
   green after. Both runs actually happened, and their output is in the
   description.
3. **No regression.** The rest of the suite ran before and after. If it was
   already red, say so rather than leaving it to be discovered.
4. **The smallest diff that solves it.** No reformatting, no tidying on the
   side, no style alignment. Those are their own pull requests, and they are
   welcome as their own pull requests.
5. **The reasoning is part of the delivery.** Why this way and not the obvious
   one. What was tried and discarded. That belongs in the commit body, where it
   survives; not in a review comment, where it does not.

Everything else is ordinary. Formatting is `gofmt` and whatever `tsc` accepts.
Commit messages are Conventional Commits, lowercase subject, under 72
characters.

## Before you open anything

```bash
make check
```

That runs exactly what CI runs, in the same order. It needs Go 1.26, Node 24 and
Python 3.

## The checkers

Three of the checks are unusual enough to explain.

`scripts/pruefer-verdrahtet.py` fails if a checker exists that no workflow
calls. A checker nobody runs produces the same silence as a satisfied one, and
one that has never run looks exactly like one that found nothing.

`scripts/pruefe-keine-hintergrundabfrage.py` fails if the extension gains a host
permission, an alarm, a background worker, a matching content script, a
`setInterval` or a fetch against linkedin.com. This is the one rule the product
is built around, and a rule that lives only in a document lasts until the next
pull request. See [docs/entscheidungen/0002-kein-scraping.md](docs/entscheidungen/0002-kein-scraping.md).

`scripts/pruefe-doku-befehle.py` fails if the README shows a command the binary
does not have. It gets the command list from `vigil help` at run time, not from
a second list in the script.

## Tests

Two rules beyond the usual, both learned the hard way:

**A mock moves the risk, it does not remove it.** If you inject a dependency to
make logic testable, the untested part is now the seam. The real implementation
needs its own test, or the mock will be right while the real thing is wrong and
nothing can fail.

**Test the wiring too.** A test that calls a function directly stays green after
nothing calls that function any more. `cmd/vigil/main_test.go` builds the actual
binary and drives it the way a user does, for exactly this reason.

## Adding a signal kind

A kind is a string. Adding one means a default rule in `core.DefaultRules()`
with a weight, a half-life and a one-line reason in the `Note` field. The reason
is not decoration: a weight without an argument is a number someone will change
back in six months.

Operators can already add their own kinds through `PUT /api/v1/rules/{kind}`
without touching the code, so a new default has to be one most users would want.

## Where changes are welcome

- Ingest connectors for sources that publish for machines, as separate programs
  posting to `/api/v1/ingest`
- Meeting preparation: a briefing per calendar entry from the stored timeline
- Buying-committee view: several people per account with roles and activity
- Export: CSV and a CRM-shaped JSON
- Anything in [docs/scope.md](docs/scope.md) marked as not written yet

## Where they are not

Anything that makes vigil send a message, and anything that makes it request a
page from LinkedIn. Those are not missing features. They are the two decisions
the rest of the design follows from, and they are written down with their
reasoning in `docs/entscheidungen/`. If you want to argue against one, argue
against the document; a pull request that quietly does it anyway will be closed.

## Security

Do not open an issue. See [SECURITY.md](SECURITY.md).
