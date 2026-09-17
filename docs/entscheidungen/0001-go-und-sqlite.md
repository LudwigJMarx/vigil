# 0001: A Go binary and a SQLite file

Decided 2026-09-17. Status: accepted.

## Problem

vigil has to be something a stranger can run. Most self-hosted sales tools ask
for a container runtime, a Postgres instance, a Redis, a `.env` with a dozen
entries and a migration command. Each of those is a place to fail before the
first screen, and the person evaluating the tool leaves at the first one.

## Decision

A single static Go binary, and SQLite in a single file, through the pure Go
driver `modernc.org/sqlite`. No cgo. The operator UI is embedded in the binary
with `go:embed`, with no CDN reference.

## Consequences

Installing is downloading a file. Backing up is copying a file. Uninstalling is
deleting two. `CGO_ENABLED=0` cross-compilation to linux, darwin and windows on
amd64 and arm64 was verified before this was written down.

The cost is single-writer concurrency and no cluster story. vigil watches one
salesperson's book, so that ceiling is far above the need. Moving to Postgres
later means replacing `internal/store`; nothing above it knows SQL.

## What was rejected

**Docker as the primary path.** It moves the dependency, it does not remove it,
and a Dockerfile the author cannot run is a Dockerfile nobody has tested. There
is no Docker on the development machine, so shipping one would have meant
shipping an untested install path as the recommended one.

**Postgres.** A second process to install, start, back up and upgrade, for a
dataset that fits in a few megabytes.

**`mattn/go-sqlite3`.** Faster, but it needs cgo, which means a C toolchain per
target and no simple static cross-compilation. That trade buys nothing at this
size and costs the entire "download one file" story.

**A Node or Python server.** Both would have meant a runtime on the host, a
lockfile to resolve and a version to pin. Go also closes a language gap in the
author's public work, which was a secondary reason, not the deciding one.
