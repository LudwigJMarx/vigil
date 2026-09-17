package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LudwigJMarx/vigil/internal/core"
)

// SaveAccount inserts a new account or updates the one that already owns the
// same normalised profile URL. Ingest calls it for accounts it has never seen,
// and an account matched by raw URL string would split into several accounts
// within weeks, so the profile is normalised before it is used as the key.
func (s *Store) SaveAccount(ctx context.Context, a core.Account) (core.Account, error) {
	if a.Profile != "" {
		if n := core.NormalizeProfile(a.Profile); n != "" {
			a.Profile = n
		}
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Stage == "" {
		a.Stage = core.StageAcquire
	}
	if err := a.Validate(); err != nil {
		return core.Account{}, err
	}

	now := time.Now().UTC()
	if a.ID == "" && a.Profile != "" {
		if existing, err := s.AccountByProfile(ctx, a.Profile); err == nil {
			a.ID = existing.ID
			a.CreatedAt = existing.CreatedAt
			if a.Note == "" {
				a.Note = existing.Note
			}
		} else if !errors.Is(err, ErrNotFound) {
			return core.Account{}, err
		}
	}
	if a.ID == "" {
		a.ID = newID("acc")
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO accounts (id, name, domain, profile, stage, note, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, domain = excluded.domain, profile = excluded.profile,
			stage = excluded.stage, note = excluded.note, updated_at = excluded.updated_at`,
		a.ID, a.Name, a.Domain, a.Profile, string(a.Stage), a.Note,
		formatTime(a.CreatedAt), formatTime(a.UpdatedAt))
	if err != nil {
		return core.Account{}, fmt.Errorf("save account %q: %w", a.Name, err)
	}
	return a, nil
}

const accountColumns = `id, name, domain, profile, stage, note, created_at, updated_at`

func scanAccount(row interface{ Scan(...any) error }) (core.Account, error) {
	var a core.Account
	var stage, created, updated string
	if err := row.Scan(&a.ID, &a.Name, &a.Domain, &a.Profile, &stage, &a.Note, &created, &updated); err != nil {
		return core.Account{}, err
	}
	a.Stage = core.Stage(stage)
	var err error
	if a.CreatedAt, err = parseTime(created); err != nil {
		return core.Account{}, fmt.Errorf("account %s: created_at: %w", a.ID, err)
	}
	if a.UpdatedAt, err = parseTime(updated); err != nil {
		return core.Account{}, fmt.Errorf("account %s: updated_at: %w", a.ID, err)
	}
	return a, nil
}

func (s *Store) Account(ctx context.Context, id string) (core.Account, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE id = ?`, id)
	a, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Account{}, ErrNotFound
	}
	return a, err
}

func (s *Store) AccountByProfile(ctx context.Context, profile string) (core.Account, error) {
	profile = core.NormalizeProfile(profile)
	if profile == "" {
		return core.Account{}, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE profile = ?`, profile)
	a, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Account{}, ErrNotFound
	}
	return a, err
}

// Accounts lists every account, newest change first.
func (s *Store) Accounts(ctx context.Context) ([]core.Account, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+accountColumns+` FROM accounts ORDER BY updated_at DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []core.Account{}
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SavePerson inserts or updates a person, keyed on the normalised profile URL
// when there is one.
func (s *Store) SavePerson(ctx context.Context, p core.Person) (core.Person, error) {
	if p.Profile != "" {
		if n := core.NormalizeProfile(p.Profile); n != "" {
			p.Profile = n
		}
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return core.Person{}, core.ErrEmptyName
	}
	if p.AccountID == "" {
		return core.Person{}, core.ErrNoAccount
	}

	now := time.Now().UTC()
	if p.ID == "" && p.Profile != "" {
		var id, created string
		err := s.db.QueryRowContext(ctx,
			`SELECT id, created_at FROM people WHERE profile = ?`, p.Profile).Scan(&id, &created)
		switch {
		case err == nil:
			p.ID = id
			if p.CreatedAt, err = parseTime(created); err != nil {
				return core.Person{}, err
			}
		case errors.Is(err, sql.ErrNoRows):
		default:
			return core.Person{}, err
		}
	}
	if p.ID == "" {
		p.ID = newID("per")
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO people (id, account_id, name, headline, profile, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			account_id = excluded.account_id, name = excluded.name,
			headline = excluded.headline, profile = excluded.profile,
			role = excluded.role, updated_at = excluded.updated_at`,
		p.ID, p.AccountID, p.Name, p.Headline, p.Profile, p.Role,
		formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		return core.Person{}, fmt.Errorf("save person %q: %w", p.Name, err)
	}
	return p, nil
}

// People lists everyone attached to an account.
func (s *Store) People(ctx context.Context, accountID string) ([]core.Person, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, name, headline, profile, role, created_at, updated_at
		FROM people WHERE account_id = ? ORDER BY name ASC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []core.Person{}
	for rows.Next() {
		var p core.Person
		var created, updated string
		if err := rows.Scan(&p.ID, &p.AccountID, &p.Name, &p.Headline, &p.Profile, &p.Role,
			&created, &updated); err != nil {
			return nil, err
		}
		if p.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if p.UpdatedAt, err = parseTime(updated); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
