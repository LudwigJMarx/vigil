// Package core holds the domain types and the scoring rules. It performs no
// I/O: everything here is a value in, a value out. That is deliberate. The
// scoring decision is the part of vigil that must stay verifiable without a
// database, an HTTP server or a browser in the loop.
package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Stage is where an account sits in the cycle vigil watches.
type Stage string

const (
	StageAcquire Stage = "acquire" // not a customer yet
	StageExpand  Stage = "expand"  // customer, room to grow
	StageRetain  Stage = "retain"  // customer, worth defending
)

// ValidStages lists every stage in the order they occur.
var ValidStages = []Stage{StageAcquire, StageExpand, StageRetain}

func (s Stage) Valid() bool {
	for _, v := range ValidStages {
		if s == v {
			return true
		}
	}
	return false
}

// Account is an organisation being watched.
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Domain    string    `json:"domain,omitempty"`
	Profile   string    `json:"profile,omitempty"` // canonical LinkedIn company URL
	Stage     Stage     `json:"stage"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Person is an individual attached to an account.
type Person struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	Headline  string    `json:"headline,omitempty"`
	Profile   string    `json:"profile,omitempty"` // canonical LinkedIn member URL
	Role      string    `json:"role,omitempty"`    // free text: champion, economic buyer, ...
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Signal is one observation about an account or a person. A signal is a fact
// with a time, never an interpretation: "posted about migrating off SAP on
// 2026-09-14", not "is in market".
type Signal struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	PersonID    string    `json:"person_id,omitempty"`
	Kind        string    `json:"kind"`
	Source      string    `json:"source"`
	Title       string    `json:"title,omitempty"`
	Body        string    `json:"body,omitempty"`
	URL         string    `json:"url,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
	ObservedAt  time.Time `json:"observed_at"`
	Fingerprint string    `json:"fingerprint"`
}

// Rule turns a signal kind into a weight and a decay. A signal kind with no
// rule scores zero and is still stored: the timeline stays complete even when
// the scoring model does not know what to do with an entry yet.
type Rule struct {
	Kind         string  `json:"kind"`
	Weight       float64 `json:"weight"`
	HalfLifeDays float64 `json:"half_life_days"`
	Enabled      bool    `json:"enabled"`
	Note         string  `json:"note,omitempty"`
}

// Contribution is one signal's share of an account score, kept so the score
// can always be taken apart again. A number nobody can decompose is a number
// nobody can argue with, and a score you cannot argue with is not useful to a
// salesperson deciding whether to send a message.
type Contribution struct {
	SignalID string  `json:"signal_id"`
	Kind     string  `json:"kind"`
	Weight   float64 `json:"weight"`
	AgeDays  float64 `json:"age_days"`
	Decay    float64 `json:"decay"`
	Points   float64 `json:"points"`
}

// Score is an account's total plus the parts it is made of.
type Score struct {
	AccountID     string         `json:"account_id"`
	Total         float64        `json:"total"`
	Contributions []Contribution `json:"contributions"`
	Unscored      []string       `json:"unscored,omitempty"` // signal kinds with no enabled rule
	At            time.Time      `json:"at"`
}

var (
	ErrEmptyName    = errors.New("name is empty")
	ErrInvalidStage = errors.New("stage is not one of acquire, expand, retain")
	ErrEmptyKind    = errors.New("kind is empty")
	ErrNoOccurredAt = errors.New("occurred_at is missing")
	ErrNoAccount    = errors.New("account_id is empty")
)

func (a *Account) Validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return ErrEmptyName
	}
	if !a.Stage.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidStage, a.Stage)
	}
	return nil
}

func (s *Signal) Validate() error {
	if strings.TrimSpace(s.AccountID) == "" {
		return ErrNoAccount
	}
	if strings.TrimSpace(s.Kind) == "" {
		return ErrEmptyKind
	}
	if s.OccurredAt.IsZero() {
		return ErrNoOccurredAt
	}
	return nil
}
