package store

// migrations are applied in order, once each, and never edited afterwards.
// Editing a migration that has already run somewhere means two databases with
// the same version number and different shapes, which is the one thing a
// version number is supposed to rule out. A correction is a new migration.
var migrations = []string{
	`CREATE TABLE accounts (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		domain      TEXT NOT NULL DEFAULT '',
		profile     TEXT NOT NULL DEFAULT '',
		stage       TEXT NOT NULL,
		note        TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);
	-- A partial index, because '' is not a profile and several accounts are
	-- allowed to have none.
	CREATE UNIQUE INDEX accounts_profile ON accounts(profile) WHERE profile <> '';

	CREATE TABLE people (
		id          TEXT PRIMARY KEY,
		account_id  TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		headline    TEXT NOT NULL DEFAULT '',
		profile     TEXT NOT NULL DEFAULT '',
		role        TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	);
	CREATE UNIQUE INDEX people_profile ON people(profile) WHERE profile <> '';
	CREATE INDEX people_account ON people(account_id);

	CREATE TABLE signals (
		id           TEXT PRIMARY KEY,
		account_id   TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		person_id    TEXT NOT NULL DEFAULT '',
		kind         TEXT NOT NULL,
		source       TEXT NOT NULL,
		title        TEXT NOT NULL DEFAULT '',
		body         TEXT NOT NULL DEFAULT '',
		url          TEXT NOT NULL DEFAULT '',
		occurred_at  TEXT NOT NULL,
		observed_at  TEXT NOT NULL,
		fingerprint  TEXT NOT NULL UNIQUE
	);
	CREATE INDEX signals_account_time ON signals(account_id, occurred_at DESC);

	CREATE TABLE rules (
		kind           TEXT PRIMARY KEY,
		weight         REAL NOT NULL,
		half_life_days REAL NOT NULL,
		enabled        INTEGER NOT NULL,
		note           TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE tokens (
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		hash         TEXT NOT NULL UNIQUE,
		created_at   TEXT NOT NULL,
		last_used_at TEXT NOT NULL DEFAULT '',
		revoked_at   TEXT NOT NULL DEFAULT ''
	);`,
}
