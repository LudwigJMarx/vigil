package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/LudwigJMarx/vigil/internal/core"
	"github.com/LudwigJMarx/vigil/internal/store"
)

// Health is what /healthz answers. It names what it counted rather than
// saying "ok": a process that is up with an unreadable database and a process
// that is up with an empty one are different problems, and "ok" hides both.
type Health struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	SchemaVersion int    `json:"schema_version"`
	Accounts      int    `json:"accounts"`
	Signals       int    `json:"signals"`
	ActiveTokens  int    `json:"active_tokens"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		s.log.Error("health: database unreachable", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "database unreachable", "version": s.version,
		})
		return
	}
	h := Health{Status: "ok", Version: s.version}
	var err error
	if h.SchemaVersion, err = s.store.SchemaVersion(); err != nil {
		s.fail(w, "read schema version", err)
		return
	}
	accounts, err := s.store.Accounts(r.Context())
	if err != nil {
		s.fail(w, "count accounts", err)
		return
	}
	h.Accounts = len(accounts)
	if h.Signals, err = s.store.CountSignals(r.Context()); err != nil {
		s.fail(w, "count signals", err)
		return
	}
	if h.ActiveTokens, err = s.store.CountActiveTokens(r.Context()); err != nil {
		s.fail(w, "count tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}

// fail logs the detail and tells the client only that it broke. The detail
// belongs in the operator's log, not in the response of an endpoint that an
// unauthenticated caller can reach.
func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	s.log.Error(what, "error", err)
	writeError(w, http.StatusInternalServerError, what+" failed")
}

// ScoredAccount is an account together with the score it currently carries.
type ScoredAccount struct {
	core.Account
	Score  float64 `json:"score"`
	Top    string  `json:"top_reason,omitempty"`
	Recent int     `json:"signals_14d"`
}

// AccountsResponse names how the list was built. `unscored_kinds` is the part
// that matters: it is the difference between "these accounts are quiet" and
// "the scoring model does not understand anything that was captured".
type AccountsResponse struct {
	Accounts      []ScoredAccount `json:"accounts"`
	ScoredAt      time.Time       `json:"scored_at"`
	SignalsRead   int             `json:"signals_read"`
	UnscoredKinds []string        `json:"unscored_kinds"`
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	now := s.now().UTC()
	accounts, err := s.store.Accounts(r.Context())
	if err != nil {
		s.fail(w, "list accounts", err)
		return
	}
	rules, err := s.store.Rules(r.Context())
	if err != nil {
		s.fail(w, "read rules", err)
		return
	}
	set := core.NewRuleSet(rules)

	// Every signal, not a recent window. A rule with a half-life of zero does
	// not fade, so a window would quietly drop weight the operator configured
	// to be permanent. vigil watches one salesperson's book; the full read is
	// affordable and the alternative is a number that is wrong by design.
	byAccount, err := s.store.SignalsSince(r.Context(), time.Time{})
	if err != nil {
		s.fail(w, "read signals", err)
		return
	}

	out := AccountsResponse{ScoredAt: now, Accounts: []ScoredAccount{}}
	unscored := map[string]bool{}
	for _, a := range accounts {
		signals := byAccount[a.ID]
		out.SignalsRead += len(signals)
		score := core.ScoreAccount(a.ID, signals, set, now)
		for _, kind := range score.Unscored {
			unscored[kind] = true
		}
		scored := ScoredAccount{Account: a, Score: score.Total}
		if len(score.Contributions) > 0 {
			scored.Top = score.Contributions[0].Kind
		}
		for _, sig := range signals {
			if now.Sub(sig.OccurredAt) <= 14*core.Day {
				scored.Recent++
			}
		}
		out.Accounts = append(out.Accounts, scored)
	}
	sort.SliceStable(out.Accounts, func(i, j int) bool {
		if out.Accounts[i].Score != out.Accounts[j].Score {
			return out.Accounts[i].Score > out.Accounts[j].Score
		}
		return out.Accounts[i].Name < out.Accounts[j].Name
	})
	out.UnscoredKinds = []string{}
	for kind := range unscored {
		out.UnscoredKinds = append(out.UnscoredKinds, kind)
	}
	sort.Strings(out.UnscoredKinds)

	writeJSON(w, http.StatusOK, out)
}

// AccountDetail is one account with everything known about it.
type AccountDetail struct {
	Account  core.Account  `json:"account"`
	People   []core.Person `json:"people"`
	Signals  []core.Signal `json:"signals"`
	Score    core.Score    `json:"score"`
	Returned int           `json:"signals_returned"`
	Total    int           `json:"signals_total"`
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	account, err := s.store.Account(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such account")
		return
	}
	if err != nil {
		s.fail(w, "read account", err)
		return
	}

	all, err := s.store.Signals(r.Context(), id, 0)
	if err != nil {
		s.fail(w, "read signals", err)
		return
	}
	rules, err := s.store.Rules(r.Context())
	if err != nil {
		s.fail(w, "read rules", err)
		return
	}
	people, err := s.store.People(r.Context(), id)
	if err != nil {
		s.fail(w, "read people", err)
		return
	}

	detail := AccountDetail{
		Account: account,
		People:  people,
		Score:   core.ScoreAccount(id, all, core.NewRuleSet(rules), s.now().UTC()),
		Total:   len(all),
	}
	// The score always reads the whole timeline; only the returned list is cut.
	// Scoring the truncated list would make an account look colder the smaller
	// the page the caller asked for.
	limit := queryInt(r, "limit", 200)
	detail.Signals = all
	if limit > 0 && len(all) > limit {
		detail.Signals = all[:limit]
	}
	detail.Returned = len(detail.Signals)

	writeJSON(w, http.StatusOK, detail)
}

func queryInt(r *http.Request, name string, fallback int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var in core.Account
	if !decode(w, r, &in) {
		return
	}
	in.ID = ""
	saved, err := s.store.SaveAccount(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteAccount(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such account")
		return
	}
	if err != nil {
		s.fail(w, "delete account", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body       string    `json:"body"`
		OccurredAt time.Time `json:"occurred_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Body == "" {
		writeError(w, http.StatusBadRequest, "body is empty")
		return
	}
	if in.OccurredAt.IsZero() {
		in.OccurredAt = s.now().UTC()
	}
	if _, err := s.store.Account(r.Context(), r.PathValue("id")); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such account")
		return
	} else if err != nil {
		s.fail(w, "read account", err)
		return
	}

	saved, inserted, err := s.store.SaveSignal(r.Context(), core.Signal{
		AccountID: r.PathValue("id"), Kind: "note", Source: "manual",
		Body: in.Body, OccurredAt: in.OccurredAt, ObservedAt: s.now().UTC(),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusCreated
	if !inserted {
		status = http.StatusOK
	}
	writeJSON(w, status, saved)
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.Rules(r.Context())
	if err != nil {
		s.fail(w, "read rules", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *Server) handlePutRule(w http.ResponseWriter, r *http.Request) {
	var in core.Rule
	if !decode(w, r, &in) {
		return
	}
	in.Kind = r.PathValue("kind")
	if err := s.store.SaveRule(r.Context(), in); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRule(r.Context(), r.PathValue("kind")); err != nil {
		s.fail(w, "delete rule", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.Tokens(r.Context())
	if err != nil {
		s.fail(w, "list tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	token, secret, err := s.store.CreateToken(r.Context(), in.Name)
	if err != nil {
		s.fail(w, "create token", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "secret": secret})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	err := s.store.RevokeToken(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such token, or already revoked")
		return
	}
	if err != nil {
		s.fail(w, "revoke token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
