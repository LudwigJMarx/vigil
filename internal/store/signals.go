package store

import (
	"context"
	"fmt"
	"time"

	"github.com/LudwigJMarx/vigil/internal/core"
)

// IngestResult says what an ingest call actually did. It exists because
// "accepted" is not an outcome: a batch where every entry was already known and
// a batch where every entry was new look the same from the outside, and the
// operator checking whether their extension works needs to tell them apart.
type IngestResult struct {
	Inserted  int      `json:"inserted"`
	Duplicate int      `json:"duplicate"`
	SignalIDs []string `json:"signal_ids"`
}

// SaveSignal stores one signal unless its fingerprint is already present.
// It reports whether the row was new.
func (s *Store) SaveSignal(ctx context.Context, sig core.Signal) (core.Signal, bool, error) {
	if sig.ObservedAt.IsZero() {
		sig.ObservedAt = time.Now().UTC()
	}
	if err := sig.Validate(); err != nil {
		return core.Signal{}, false, err
	}
	if sig.Fingerprint == "" {
		sig.Fingerprint = core.Fingerprint(sig)
	}
	if sig.ID == "" {
		sig.ID = newID("sig")
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO signals
			(id, account_id, person_id, kind, source, title, body, url, occurred_at, observed_at, fingerprint)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO NOTHING`,
		sig.ID, sig.AccountID, sig.PersonID, sig.Kind, sig.Source, sig.Title, sig.Body, sig.URL,
		formatTime(sig.OccurredAt), formatTime(sig.ObservedAt), sig.Fingerprint)
	if err != nil {
		return core.Signal{}, false, fmt.Errorf("save signal %s: %w", sig.Kind, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return core.Signal{}, false, err
	}
	if affected == 0 {
		// Already known. Hand back the row that is actually stored, so the
		// caller never gets an id that no table row answers to.
		existing, err := s.signalByFingerprint(ctx, sig.Fingerprint)
		if err != nil {
			return core.Signal{}, false, err
		}
		return existing, false, nil
	}
	return sig, true, nil
}

const signalColumns = `id, account_id, person_id, kind, source, title, body, url, occurred_at, observed_at, fingerprint`

func scanSignal(row interface{ Scan(...any) error }) (core.Signal, error) {
	var sig core.Signal
	var occurred, observed string
	if err := row.Scan(&sig.ID, &sig.AccountID, &sig.PersonID, &sig.Kind, &sig.Source,
		&sig.Title, &sig.Body, &sig.URL, &occurred, &observed, &sig.Fingerprint); err != nil {
		return core.Signal{}, err
	}
	var err error
	if sig.OccurredAt, err = parseTime(occurred); err != nil {
		return core.Signal{}, fmt.Errorf("signal %s: occurred_at: %w", sig.ID, err)
	}
	if sig.ObservedAt, err = parseTime(observed); err != nil {
		return core.Signal{}, fmt.Errorf("signal %s: observed_at: %w", sig.ID, err)
	}
	return sig, nil
}

func (s *Store) signalByFingerprint(ctx context.Context, fp string) (core.Signal, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+signalColumns+` FROM signals WHERE fingerprint = ?`, fp)
	return scanSignal(row)
}

// Signals returns an account's timeline, most recent event first. A limit of
// zero or less means no limit.
func (s *Store) Signals(ctx context.Context, accountID string, limit int) ([]core.Signal, error) {
	query := `SELECT ` + signalColumns + ` FROM signals WHERE account_id = ? ORDER BY occurred_at DESC`
	args := []any{accountID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []core.Signal{}
	for rows.Next() {
		sig, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, rows.Err()
}

// SignalsSince returns every signal across all accounts that occurred at or
// after `since`, oldest first. The scoring pass over the whole book uses it so
// it makes one query instead of one per account.
func (s *Store) SignalsSince(ctx context.Context, since time.Time) (map[string][]core.Signal, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+signalColumns+` FROM signals WHERE occurred_at >= ? ORDER BY occurred_at ASC`,
		formatTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]core.Signal{}
	for rows.Next() {
		sig, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		out[sig.AccountID] = append(out[sig.AccountID], sig)
	}
	return out, rows.Err()
}

// CountSignals reports how many signals are stored, in total and for one
// account. Used by the health endpoint and by the ingest check, so an empty
// answer can be told apart from a broken query.
func (s *Store) CountSignals(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM signals`).Scan(&n)
	return n, err
}
