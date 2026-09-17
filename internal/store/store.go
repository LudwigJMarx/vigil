// Package store keeps vigil's data in a single SQLite file. One file is the
// whole point: an operator who wants to move, back up or throw away their
// instance copies or deletes one path, with no server process to coordinate.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure Go, so the binary needs no cgo and no system libsqlite
)

// Store is a handle on the database. It is safe for concurrent use.
type Store struct {
	db *sql.DB
}

// ErrNotFound is returned instead of sql.ErrNoRows so callers outside this
// package never have to import database/sql to tell "absent" from "broken".
var ErrNotFound = errors.New("not found")

// Open opens (and creates, if needed) the database at path and brings its
// schema up to date. Pass ":memory:" for a throwaway database.
func Open(path string) (*Store, error) {
	dsn := path
	if path == ":memory:" {
		// A shared cache keeps every pooled connection on the same in-memory
		// database. Without it the pool hands out empty databases at random and
		// the failure looks like data loss rather than a configuration mistake.
		dsn = "file::memory:?cache=shared"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for tests and for the health check. Nothing outside
// this package writes through it.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > len(migrations) {
		return fmt.Errorf(
			"database is at schema version %d, this binary knows %d: it was written by a newer vigil",
			version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		// PRAGMA takes no bind parameters; i+1 is an int from a range, not input.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: record version: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d: commit: %w", i+1, err)
		}
	}
	return nil
}

// SchemaVersion reports how many migrations have been applied.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&v)
	return v, err
}

// Ping fails loudly when the file behind the handle has gone away. A health
// check that only reports "the process is up" answers a question nobody asked.
func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

const timeLayout = time.RFC3339Nano

func newID(prefix string) string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		// crypto/rand does not fail on any platform vigil supports, and a
		// non-random id would silently weaken token lookup. Stop instead.
		panic("vigil: no entropy available: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(raw)
}

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(timeLayout, s)
}
