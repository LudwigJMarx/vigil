# What vigil collects, and what it refuses to

This file exists because the honest answer to "why does it not just read my
feed for me" is long enough that it does not fit in a README table.

## The rule the product is built around

LinkedIn's User Agreement forbids using bots, scrapers or other automated means
to access the service or copy data from it. That restriction binds the person
whose session makes the request, not the author of the tool. A self-hosted
scraper therefore puts the operator's own LinkedIn account at risk of
restriction, and hands the operator the legal exposure as well.

So vigil does not make requests to LinkedIn. Not from the server, not from a
background worker, not from the extension.

## What the extension is allowed to see

| | |
|---|---|
| When | Only while the popup is open, which requires a click on the icon |
| What | `location.href`, `document.title`, and the current text selection |
| How | `chrome.scripting.executeScript` under `activeTab` |
| Host permissions | None. Not for LinkedIn, not for anything |
| Background work | None. No service worker, no `chrome.alarms`, no `setInterval` |

`activeTab` grants access to one tab, at the moment of the click, and nothing
afterwards. Without that click the extension cannot see any page at all.

Three fields is not an accident of effort. LinkedIn's markup is generated and
its class names change without notice. A selector that quietly stops matching
turns a busy account into a quiet one, and a quiet account is exactly what
nobody investigates. The URL, the title and the user's own selection do not
move.

## What that costs, stated plainly

vigil does not watch fifty accounts while you focus on one conversation. It
remembers, ranks and explains what you fed it. The capture is a click, not a
cron job. If the promise you want is "the tool finds signals for you without
you being there", vigil is not that tool, and a tool that does that with
LinkedIn data is making a promise with your account as the collateral.

## Where automated collection is fine

Sources that publish for machines are a different matter, and the ingest API is
the same for all of them:

- RSS and Atom feeds from company blogs and newsrooms
- Job board feeds, where the terms allow it
- Press release and funding databases with an API and a licence
- Your own CRM and your own mailbox
- Anything you have a contract for

`POST /api/v1/ingest` takes a signal from any source. `source` is a free string
and `kind` is a free string with a scoring rule attached. A cron job on your own
machine that reads a company's Atom feed and posts `funding` signals is a
supported and intended use. It is also not written yet: it is left to the
operator, because the licence situation differs per feed and vigil has no way
to know yours.

## The fingerprint

Anything captured twice has to land once, or the score doubles for an event
that happened once. vigil derives a fingerprint per signal:

The timestamp never enters the identity while a URL is present, because
LinkedIn renders "2d", which resolves to a different instant on every capture.
What the URL is worth depends on what it points at:

- **A permalink** such as `/feed/update/urn:li:activity:7100` names one item, so
  `source`, `kind` and the URL are the whole identity. Two people selecting
  different halves of the same post still captured one post.
- **A profile or company page** names a place, not an observation. Everything
  captured while standing on it carries the same URL, so the title and the body
  join the identity there. Two observations stay two; pressing Send twice still
  stays one.
- **No URL**: `source`, `kind`, the account, the person, the exact timestamp and
  the text. Two notes a minute apart stay two notes, because nothing stable is
  left to key on and merging them would lose one.

The middle case was wrong until 17.09.2026: the URL alone decided, so the second
capture from a company page came back as "already known" and was dropped. It was
found by driving the extension's popup against a running instance, and it is the
worst shape a bug can take here, because silent loss and correct deduplication
look identical from the outside.

## Time

A capture with no time is rejected, by index and with a reason, not stored with
`now`. Dating a three-month-old post to today would make the coldest account on
the list come out hottest, and nothing about the result would look wrong
afterwards. A time more than a day in the future is rejected for the same
reason from the other side.

The extension reads a relative time such as `3d` or `2 Wo.` only when it is a
line of its own. An earlier version searched anywhere in the selected text and
read "raised 12 m EUR" as twelve minutes ago.
