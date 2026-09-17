package store

import (
	"context"
	"fmt"
	"sort"

	"github.com/LudwigJMarx/vigil/internal/core"
)

// Rules returns the scoring model in force: the defaults, with any stored
// override replacing the default of the same kind, plus kinds the operator
// added that have no default.
//
// The defaults are not written into the database at install time on purpose.
// A default that has been copied into a table stops being a default: it freezes
// at the value of the version that created it, and an operator who never
// touched a rule silently keeps a model that later releases have moved on from.
func (s *Store) Rules(ctx context.Context) ([]core.Rule, error) {
	byKind := map[string]core.Rule{}
	for _, r := range core.DefaultRules() {
		byKind[r.Kind] = r
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT kind, weight, half_life_days, enabled, note FROM rules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r core.Rule
		var enabled int
		if err := rows.Scan(&r.Kind, &r.Weight, &r.HalfLifeDays, &enabled, &r.Note); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		byKind[r.Kind] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]core.Rule, 0, len(byKind))
	for _, r := range byKind {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out, nil
}

// SaveRule stores an override for one signal kind.
func (s *Store) SaveRule(ctx context.Context, r core.Rule) error {
	if r.Kind == "" {
		return core.ErrEmptyKind
	}
	if r.HalfLifeDays < 0 {
		return fmt.Errorf("rule %q: half_life_days is negative", r.Kind)
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rules (kind, weight, half_life_days, enabled, note)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(kind) DO UPDATE SET
			weight = excluded.weight, half_life_days = excluded.half_life_days,
			enabled = excluded.enabled, note = excluded.note`,
		r.Kind, r.Weight, r.HalfLifeDays, enabled, r.Note)
	return err
}

// DeleteRule drops an override so the default for that kind applies again.
// It is not an error to delete a kind that was never overridden: the caller
// asked for "no override", and that is the state afterwards either way.
func (s *Store) DeleteRule(ctx context.Context, kind string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rules WHERE kind = ?`, kind)
	return err
}
