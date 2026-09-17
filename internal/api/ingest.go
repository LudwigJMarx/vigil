package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/LudwigJMarx/vigil/internal/core"
)

// MaxSignalsPerRequest caps one capture. The extension sends what is on the
// screen in front of a human, so a batch in the thousands means something else
// is going on and should be refused rather than absorbed.
const MaxSignalsPerRequest = 200

// IngestRequest is what the browser extension posts: the account the user was
// looking at, optionally the person, and the observations from that page.
type IngestRequest struct {
	AccountID string        `json:"account_id,omitempty"`
	Account   *core.Account `json:"account,omitempty"`
	Person    *core.Person  `json:"person,omitempty"`
	Signals   []core.Signal `json:"signals"`
}

// Rejection names one entry that did not make it, and why. Dropping an entry
// without saying so turns a broken capture into a quiet account.
type Rejection struct {
	Index  int    `json:"index"`
	Kind   string `json:"kind,omitempty"`
	Reason string `json:"reason"`
}

// IngestResponse reports what the call did, entry by entry. "accepted" is not
// an outcome: a batch that was entirely duplicate and a batch that was entirely
// new are the same 200 otherwise, and the person testing their extension needs
// to tell those apart on the first try.
type IngestResponse struct {
	AccountID string      `json:"account_id"`
	PersonID  string      `json:"person_id,omitempty"`
	Received  int         `json:"received"`
	Inserted  int         `json:"inserted"`
	Duplicate int         `json:"duplicate"`
	Rejected  []Rejection `json:"rejected"`
	SignalIDs []string    `json:"signal_ids"`
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var in IngestRequest
	if !decode(w, r, &in) {
		return
	}
	if len(in.Signals) > MaxSignalsPerRequest {
		writeError(w, http.StatusRequestEntityTooLarge, "more signals than one page can hold")
		return
	}

	accountID, err := s.resolveAccount(r, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	personID := ""
	if in.Person != nil && strings.TrimSpace(in.Person.Name) != "" {
		person := *in.Person
		person.ID = ""
		person.AccountID = accountID
		saved, err := s.store.SavePerson(r.Context(), person)
		if err != nil {
			writeError(w, http.StatusBadRequest, "person: "+err.Error())
			return
		}
		personID = saved.ID
	}

	observed := s.now().UTC()
	out := IngestResponse{
		AccountID: accountID, PersonID: personID,
		Received: len(in.Signals), Rejected: []Rejection{}, SignalIDs: []string{},
	}
	for i, sig := range in.Signals {
		sig.ID = ""
		sig.Fingerprint = ""
		sig.AccountID = accountID
		if sig.PersonID == "" {
			sig.PersonID = personID
		}
		if sig.Source == "" {
			sig.Source = "linkedin"
		}
		sig.ObservedAt = observed
		if sig.OccurredAt.IsZero() {
			// A capture with no time is not a signal, it is a note with a hole
			// in it. Guessing "now" would date a three-month-old post to today
			// and make a cold account the hottest one on the list.
			out.Rejected = append(out.Rejected, Rejection{
				Index: i, Kind: sig.Kind, Reason: "occurred_at is missing"})
			continue
		}
		if sig.OccurredAt.After(observed.Add(24 * time.Hour)) {
			out.Rejected = append(out.Rejected, Rejection{
				Index: i, Kind: sig.Kind, Reason: "occurred_at is more than a day in the future"})
			continue
		}

		saved, inserted, err := s.store.SaveSignal(r.Context(), sig)
		if err != nil {
			out.Rejected = append(out.Rejected, Rejection{
				Index: i, Kind: sig.Kind, Reason: err.Error()})
			continue
		}
		if inserted {
			out.Inserted++
		} else {
			out.Duplicate++
		}
		out.SignalIDs = append(out.SignalIDs, saved.ID)
	}

	status := http.StatusOK
	if len(out.Rejected) > 0 && out.Inserted == 0 && out.Duplicate == 0 {
		// Nothing was stored. A 200 here would let a permanently broken
		// extension look healthy for as long as nobody opens the response.
		status = http.StatusBadRequest
	}
	writeJSON(w, status, out)
}

// resolveAccount finds or creates the account a capture belongs to.
func (s *Server) resolveAccount(r *http.Request, in IngestRequest) (string, error) {
	if in.AccountID != "" {
		account, err := s.store.Account(r.Context(), in.AccountID)
		if err != nil {
			return "", err
		}
		return account.ID, nil
	}
	if in.Account == nil {
		return "", errNoAccountGiven
	}
	account := *in.Account
	account.ID = ""
	if account.Stage == "" {
		account.Stage = core.StageAcquire
	}
	saved, err := s.store.SaveAccount(r.Context(), account)
	if err != nil {
		return "", err
	}
	return saved.ID, nil
}

var errNoAccountGiven = ingestError("neither account_id nor account was given")

type ingestError string

func (e ingestError) Error() string { return string(e) }
