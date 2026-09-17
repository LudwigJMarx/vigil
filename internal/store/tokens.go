package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Token is what the API shows about a credential. The secret itself appears
// exactly once, in the return value of CreateToken, and is never stored.
type Token struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitzero"`
	RevokedAt  time.Time `json:"revoked_at,omitzero"`
}

// ErrBadToken is returned for every authentication failure: unknown, revoked,
// or malformed. Callers must not distinguish them to the client, because the
// difference tells an attacker which guesses were close.
var ErrBadToken = errors.New("token is not valid")

const tokenPrefix = "vgl_"

func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// CreateToken issues a credential and returns the secret in clear text. This
// is the only moment it exists outside the caller's hands; the database holds
// a SHA-256 of it. Losing it means issuing a new one, not recovering this one.
func (s *Store) CreateToken(ctx context.Context, name string) (Token, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unnamed"
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Token{}, "", err
	}
	secret := tokenPrefix + base64.RawURLEncoding.EncodeToString(raw)

	t := Token{ID: newID("tok"), Name: name, CreatedAt: time.Now().UTC()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tokens (id, name, hash, created_at) VALUES (?, ?, ?, ?)`,
		t.ID, t.Name, hashToken(secret), formatTime(t.CreatedAt))
	if err != nil {
		return Token{}, "", err
	}
	return t, secret, nil
}

// Authenticate resolves a secret to its token, or fails. It compares the
// stored hash with a constant-time comparison even though the lookup is by
// hash: the shape of the check is the thing that gets copied into the next
// function, and a fast-exit comparison here would be copied too.
func (s *Store) Authenticate(ctx context.Context, secret string) (Token, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Token{}, ErrBadToken
	}
	want := hashToken(secret)

	var t Token
	var stored, created, lastUsed, revoked string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, hash, created_at, last_used_at, revoked_at FROM tokens WHERE hash = ?`,
		want).Scan(&t.ID, &t.Name, &stored, &created, &lastUsed, &revoked)
	if err != nil {
		return Token{}, ErrBadToken
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(want)) != 1 {
		return Token{}, ErrBadToken
	}
	if revoked != "" {
		return Token{}, ErrBadToken
	}
	if t.CreatedAt, err = parseTime(created); err != nil {
		return Token{}, err
	}
	if t.LastUsedAt, err = parseTime(lastUsed); err != nil {
		return Token{}, err
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE tokens SET last_used_at = ? WHERE id = ?`,
		formatTime(time.Now().UTC()), t.ID); err != nil {
		return Token{}, err
	}
	return t, nil
}

// Tokens lists the credentials that exist, without their secrets.
func (s *Store) Tokens(ctx context.Context) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at, last_used_at, revoked_at FROM tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Token{}
	for rows.Next() {
		var t Token
		var created, lastUsed, revoked string
		if err := rows.Scan(&t.ID, &t.Name, &created, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		if t.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if t.LastUsedAt, err = parseTime(lastUsed); err != nil {
			return nil, err
		}
		if t.RevokedAt, err = parseTime(revoked); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeToken disables a credential by id.
func (s *Store) RevokeToken(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tokens SET revoked_at = ? WHERE id = ? AND revoked_at = ''`,
		formatTime(time.Now().UTC()), id)
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

// CountActiveTokens reports how many usable credentials exist. The server
// refuses a non-loopback bind when this is zero, so the number has to be real.
func (s *Store) CountActiveTokens(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tokens WHERE revoked_at = ''`).Scan(&n)
	return n, err
}
