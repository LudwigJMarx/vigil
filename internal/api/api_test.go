package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/LudwigJMarx/vigil/internal/core"
	"github.com/LudwigJMarx/vigil/internal/store"
)

var fixed = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

type harness struct {
	*httptest.Server
	store  *store.Store
	secret string
	t      *testing.T
}

func start(t *testing.T) *harness {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "vigil.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, secret, err := db.CreateToken(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(New(Options{
		Store:   db,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: "test",
		Now:     func() time.Time { return fixed },
	}))
	t.Cleanup(srv.Close)
	return &harness{Server: srv, store: db, secret: secret, t: t}
}

// do sends a request with the test token unless auth is the empty string.
func (h *harness) do(method, path string, body any, auth string) (*http.Response, []byte) {
	h.t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.URL+path, payload)
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return resp, raw
}

func (h *harness) auth(method, path string, body any) (*http.Response, []byte) {
	h.t.Helper()
	return h.do(method, path, body, h.secret)
}

func decodeInto[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func TestEveryAPIRouteRefusesAnUnauthenticatedCall(t *testing.T) {
	// Walked route by route on purpose. A guard applied by hand to a map of
	// handlers is a guard someone forgets to apply to the next entry.
	h := start(t)
	routes := []struct{ method, path string }{
		{"POST", "/api/v1/ingest"},
		{"GET", "/api/v1/accounts"},
		{"POST", "/api/v1/accounts"},
		{"GET", "/api/v1/accounts/acc_x"},
		{"DELETE", "/api/v1/accounts/acc_x"},
		{"POST", "/api/v1/accounts/acc_x/note"},
		{"GET", "/api/v1/rules"},
		{"PUT", "/api/v1/rules/post"},
		{"DELETE", "/api/v1/rules/post"},
		{"GET", "/api/v1/tokens"},
		{"POST", "/api/v1/tokens"},
		{"DELETE", "/api/v1/tokens/tok_x"},
	}
	for _, route := range routes {
		resp, _ := h.do(route.method, route.path, map[string]any{}, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: %d, want 401", route.method, route.path, resp.StatusCode)
		}
	}
}

func TestAWrongTokenIsRefusedWithTheSameAnswerAsAMissingOne(t *testing.T) {
	h := start(t)
	resp, body := h.do("GET", "/api/v1/accounts", nil, "vgl_wrong")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", resp.StatusCode)
	}
	if bytes.Contains(body, []byte("revoked")) || bytes.Contains(body, []byte("unknown")) {
		t.Fatalf("the error tells a guesser how close they were: %s", body)
	}
}

func TestARevokedTokenStopsWorkingImmediately(t *testing.T) {
	h := start(t)
	_, raw := h.auth("GET", "/api/v1/tokens", nil)
	list := decodeInto[struct {
		Tokens []store.Token `json:"tokens"`
	}](t, raw)
	if len(list.Tokens) != 1 {
		t.Fatalf("%d tokens, want 1", len(list.Tokens))
	}

	resp, _ := h.auth("DELETE", "/api/v1/tokens/"+list.Tokens[0].ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}
	resp, _ = h.auth("GET", "/api/v1/accounts", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("the revoked token still works: %d", resp.StatusCode)
	}
}

func TestHealthIsOpenAndSaysWhatItCounted(t *testing.T) {
	h := start(t)
	resp, raw := h.do("GET", "/healthz", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	health := decodeInto[Health](t, raw)
	if health.Status != "ok" || health.Version != "test" {
		t.Fatalf("health = %+v", health)
	}
	if health.SchemaVersion < 1 {
		t.Fatalf("schema version %d, want at least 1", health.SchemaVersion)
	}
	if health.ActiveTokens != 1 {
		t.Fatalf("active tokens %d, want 1", health.ActiveTokens)
	}
}

func TestTheAPINeverSetsACookie(t *testing.T) {
	// The wildcard CORS origin is only defensible while vigil authenticates
	// with a header. The day something here sets a cookie, that wildcard turns
	// every page on the internet into an authenticated client.
	h := start(t)
	for _, path := range []string{"/healthz", "/api/v1/accounts", "/"} {
		resp, _ := h.auth("GET", path, nil)
		if len(resp.Header.Values("Set-Cookie")) > 0 {
			t.Fatalf("%s set a cookie while CORS allows every origin", path)
		}
	}
}

func ingestPost(url string, occurred time.Time) IngestRequest {
	return IngestRequest{
		Account: &core.Account{Name: "Acme GmbH", Profile: "https://www.linkedin.com/company/acme/"},
		Person:  &core.Person{Name: "Jane Doe", Profile: "https://www.linkedin.com/in/jane-doe/"},
		Signals: []core.Signal{{
			Kind: "post", Source: "linkedin", Title: "we are replacing our CRM",
			URL: url, OccurredAt: occurred,
		}},
	}
}

func TestIngestCreatesTheAccountAndThePersonItWasGiven(t *testing.T) {
	h := start(t)
	resp, raw := h.auth("POST", "/api/v1/ingest",
		ingestPost("https://www.linkedin.com/feed/update/urn:li:activity:7100", fixed.Add(-48*time.Hour)))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	out := decodeInto[IngestResponse](t, raw)
	if out.Inserted != 1 || out.Duplicate != 0 || len(out.Rejected) != 0 {
		t.Fatalf("result = %+v", out)
	}
	if out.AccountID == "" || out.PersonID == "" {
		t.Fatalf("no ids came back: %+v", out)
	}
}

func TestCapturingTheSamePageTwiceIsReportedAsDuplicateNotAsSuccess(t *testing.T) {
	h := start(t)
	first := ingestPost("https://www.linkedin.com/feed/update/urn:li:activity:7100?trk=x", fixed.Add(-48*time.Hour))
	h.auth("POST", "/api/v1/ingest", first)

	second := ingestPost("https://de.linkedin.com/feed/update/urn:li:activity:7100/", fixed.Add(-47*time.Hour))
	_, raw := h.auth("POST", "/api/v1/ingest", second)
	out := decodeInto[IngestResponse](t, raw)

	if out.Inserted != 0 || out.Duplicate != 1 {
		t.Fatalf("result = %+v, want one duplicate and nothing inserted", out)
	}
}

func TestASignalWithoutATimeIsRejectedByNameNotDroppedQuietly(t *testing.T) {
	h := start(t)
	resp, raw := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Acme"},
		Signals: []core.Signal{{Kind: "post", Source: "linkedin", Title: "no time"}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 when nothing was stored", resp.StatusCode)
	}
	out := decodeInto[IngestResponse](t, raw)
	if len(out.Rejected) != 1 || out.Rejected[0].Index != 0 {
		t.Fatalf("rejected = %+v", out.Rejected)
	}
	if out.Rejected[0].Reason == "" {
		t.Fatal("a rejection with no reason is a silent drop with extra steps")
	}
}

func TestOneBadSignalDoesNotDiscardTheGoodOnesBesideIt(t *testing.T) {
	h := start(t)
	resp, raw := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Acme"},
		Signals: []core.Signal{
			{Kind: "post", Source: "linkedin", URL: "linkedin.com/a", OccurredAt: fixed.Add(-time.Hour)},
			{Kind: "post", Source: "linkedin", URL: "linkedin.com/b"}, // no time
			{Kind: "post", Source: "linkedin", URL: "linkedin.com/c", OccurredAt: fixed.Add(-2 * time.Hour)},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	out := decodeInto[IngestResponse](t, raw)
	if out.Inserted != 2 || len(out.Rejected) != 1 {
		t.Fatalf("result = %+v, want 2 inserted and 1 rejected", out)
	}
	if out.Received != 3 {
		t.Fatalf("received = %d, want 3", out.Received)
	}
}

func TestASignalDatedInTheFutureIsRefused(t *testing.T) {
	h := start(t)
	_, raw := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Acme"},
		Signals: []core.Signal{{
			Kind: "post", Source: "linkedin", URL: "linkedin.com/a",
			OccurredAt: fixed.Add(72 * time.Hour),
		}},
	})
	out := decodeInto[IngestResponse](t, raw)
	if len(out.Rejected) != 1 {
		t.Fatalf("a post dated three days from now was accepted: %+v", out)
	}
}

func TestIngestWithoutAnAccountIsRefused(t *testing.T) {
	h := start(t)
	resp, _ := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Signals: []core.Signal{{Kind: "post", OccurredAt: fixed}},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", resp.StatusCode)
	}
}

func TestTheAccountListIsOrderedByScoreAndNamesWhatItRead(t *testing.T) {
	h := start(t)
	h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Quiet Ltd", Profile: "linkedin.com/company/quiet"},
		Signals: []core.Signal{{Kind: "reaction", Source: "linkedin",
			URL: "linkedin.com/x", OccurredAt: fixed.Add(-24 * time.Hour)}},
	})
	h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Hot AG", Profile: "linkedin.com/company/hot"},
		Signals: []core.Signal{{Kind: "job_change", Source: "linkedin",
			URL: "linkedin.com/y", OccurredAt: fixed.Add(-24 * time.Hour)}},
	})
	// A kind the scoring model has never heard of.
	h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Hot AG", Profile: "linkedin.com/company/hot"},
		Signals: []core.Signal{{Kind: "telepathy", Source: "linkedin",
			URL: "linkedin.com/z", OccurredAt: fixed.Add(-24 * time.Hour)}},
	})

	_, raw := h.auth("GET", "/api/v1/accounts", nil)
	out := decodeInto[AccountsResponse](t, raw)

	if len(out.Accounts) != 2 {
		t.Fatalf("%d accounts, want 2", len(out.Accounts))
	}
	if out.Accounts[0].Name != "Hot AG" {
		t.Fatalf("first account is %q, want Hot AG", out.Accounts[0].Name)
	}
	if out.SignalsRead != 3 {
		t.Fatalf("signals_read = %d, want 3", out.SignalsRead)
	}
	if len(out.UnscoredKinds) != 1 || out.UnscoredKinds[0] != "telepathy" {
		t.Fatalf("unscored_kinds = %v, want [telepathy]", out.UnscoredKinds)
	}
	if out.Accounts[0].Top != "job_change" {
		t.Fatalf("top_reason = %q, want job_change", out.Accounts[0].Top)
	}
}

func TestAskingForFewerTimelineEntriesDoesNotLowerTheScore(t *testing.T) {
	// Scoring the page instead of the timeline would make an account look
	// colder the smaller the page, which is a number that depends on how it
	// was asked for.
	h := start(t)
	signals := []core.Signal{}
	for i := 0; i < 5; i++ {
		signals = append(signals, core.Signal{
			Kind: "post", Source: "linkedin",
			URL:        "linkedin.com/p/" + string(rune('a'+i)),
			OccurredAt: fixed.Add(-time.Duration(i) * 24 * time.Hour),
		})
	}
	_, raw := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Acme"}, Signals: signals,
	})
	accountID := decodeInto[IngestResponse](t, raw).AccountID

	_, raw = h.auth("GET", "/api/v1/accounts/"+accountID+"?limit=2", nil)
	small := decodeInto[AccountDetail](t, raw)
	_, raw = h.auth("GET", "/api/v1/accounts/"+accountID, nil)
	full := decodeInto[AccountDetail](t, raw)

	if small.Returned != 2 || full.Returned != 5 {
		t.Fatalf("returned %d and %d, want 2 and 5", small.Returned, full.Returned)
	}
	if small.Total != 5 {
		t.Fatalf("signals_total = %d, want 5 regardless of the limit", small.Total)
	}
	if small.Score.Total != full.Score.Total {
		t.Fatalf("score changed with the page size: %.4f vs %.4f",
			small.Score.Total, full.Score.Total)
	}
}

func TestAnUnknownAccountIsA404(t *testing.T) {
	h := start(t)
	resp, _ := h.auth("GET", "/api/v1/accounts/acc_nope", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", resp.StatusCode)
	}
}

func TestChangingARuleChangesTheScore(t *testing.T) {
	h := start(t)
	_, raw := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Acme"},
		Signals: []core.Signal{{Kind: "post", Source: "linkedin",
			URL: "linkedin.com/p", OccurredAt: fixed}},
	})
	accountID := decodeInto[IngestResponse](t, raw).AccountID

	_, raw = h.auth("GET", "/api/v1/accounts/"+accountID, nil)
	before := decodeInto[AccountDetail](t, raw).Score.Total

	resp, raw := h.auth("PUT", "/api/v1/rules/post",
		core.Rule{Kind: "post", Weight: 100, HalfLifeDays: 30, Enabled: true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put rule: %d %s", resp.StatusCode, raw)
	}

	_, raw = h.auth("GET", "/api/v1/accounts/"+accountID, nil)
	after := decodeInto[AccountDetail](t, raw).Score.Total

	if after <= before {
		t.Fatalf("score did not follow the rule: %.2f then %.2f", before, after)
	}
}

func TestANoteLandsOnTheTimelineAndScoresNothing(t *testing.T) {
	h := start(t)
	_, raw := h.auth("POST", "/api/v1/accounts", core.Account{Name: "Acme", Stage: core.StageAcquire})
	accountID := decodeInto[core.Account](t, raw).ID

	resp, _ := h.auth("POST", "/api/v1/accounts/"+accountID+"/note",
		map[string]string{"body": "Jane asked for a reference customer"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d, want 201", resp.StatusCode)
	}

	_, raw = h.auth("GET", "/api/v1/accounts/"+accountID, nil)
	detail := decodeInto[AccountDetail](t, raw)
	if detail.Total != 1 {
		t.Fatalf("%d signals on the timeline, want 1", detail.Total)
	}
	if detail.Score.Total != 0 {
		t.Fatalf("score = %.2f, want 0: a note is context, not a buying signal", detail.Score.Total)
	}
}

func TestTheUIIsServedFromTheBinaryWithAStrictPolicy(t *testing.T) {
	h := start(t)
	resp, body := h.do("GET", "/", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("<title>vigil</title>")) {
		t.Fatalf("that is not the UI: %.120s", body)
	}
	policy := resp.Header.Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("no Content-Security-Policy on a page that holds a bearer token")
	}
	if bytes.Contains(body, []byte("<script>")) {
		t.Fatal("an inline script would force 'unsafe-inline' into the policy")
	}
}

func TestAnOversizedBatchIsRefusedRatherThanAbsorbed(t *testing.T) {
	h := start(t)
	signals := make([]core.Signal, MaxSignalsPerRequest+1)
	for i := range signals {
		signals[i] = core.Signal{Kind: "post", Source: "linkedin", OccurredAt: fixed}
	}
	resp, _ := h.auth("POST", "/api/v1/ingest", IngestRequest{
		Account: &core.Account{Name: "Acme"}, Signals: signals,
	})
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", resp.StatusCode)
	}
}
