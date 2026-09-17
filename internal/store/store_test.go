package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/LudwigJMarx/vigil/internal/core"
)

// open gives each test its own file on disk rather than an in-memory database.
// The migration, the WAL pragma and the partial unique indexes are exactly the
// parts an in-memory shortcut would stop exercising, and they are the parts
// that break on a real install.
func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "vigil.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ctx() context.Context { return context.Background() }

func account(t *testing.T, s *Store, name, profile string) core.Account {
	t.Helper()
	a, err := s.SaveAccount(ctx(), core.Account{Name: name, Profile: profile, Stage: core.StageAcquire})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	return a
}

func TestAFreshDatabaseIsAtTheLatestSchemaVersion(t *testing.T) {
	s := open(t)
	v, err := s.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v != len(migrations) {
		t.Fatalf("schema version %d, want %d", v, len(migrations))
	}
}

func TestReopeningAnExistingDatabaseDoesNotReapplyMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vigil.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a := account(t, first, "Acme", "https://www.linkedin.com/company/acme/")
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	if _, err := second.Account(ctx(), a.ID); err != nil {
		t.Fatalf("account lost across reopen: %v", err)
	}
}

func TestADatabaseFromANewerVigilIsRefusedNotSilentlyUsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vigil.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	if _, err := Open(path); err == nil {
		t.Fatal("opened a database written by a newer binary without complaint")
	}
}

func TestTheSameCompanyProfileResolvesToOneAccount(t *testing.T) {
	s := open(t)
	first := account(t, s, "Acme", "https://www.linkedin.com/company/acme/")
	second := account(t, s, "Acme GmbH", "https://de.linkedin.com/company/Acme?trk=nav")

	if first.ID != second.ID {
		t.Fatalf("two ids for one company: %s and %s", first.ID, second.ID)
	}
	all, err := s.Accounts(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("%d accounts stored, want 1", len(all))
	}
	if all[0].Name != "Acme GmbH" {
		t.Fatalf("name = %q, want the later capture to win", all[0].Name)
	}
}

func TestSeveralAccountsMayHaveNoProfile(t *testing.T) {
	// The unique index on profile is partial for this reason. A plain unique
	// index would let exactly one account exist without a LinkedIn page.
	s := open(t)
	account(t, s, "One", "")
	account(t, s, "Two", "")

	all, err := s.Accounts(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("%d accounts stored, want 2", len(all))
	}
}

func TestAnAccountWithoutANameIsRefused(t *testing.T) {
	s := open(t)
	if _, err := s.SaveAccount(ctx(), core.Account{Name: "  "}); !errors.Is(err, core.ErrEmptyName) {
		t.Fatalf("err = %v, want ErrEmptyName", err)
	}
}

func TestAMissingAccountIsNotFoundNotAnEmptyAccount(t *testing.T) {
	s := open(t)
	if _, err := s.Account(ctx(), "acc_nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCapturingTheSamePostTwiceStoresItOnce(t *testing.T) {
	s := open(t)
	a := account(t, s, "Acme", "https://www.linkedin.com/company/acme/")
	post := core.Signal{
		AccountID: a.ID, Kind: "post", Source: "linkedin",
		URL:        "https://www.linkedin.com/feed/update/urn:li:activity:7100?trk=x",
		OccurredAt: time.Now().Add(-48 * time.Hour).UTC(),
	}

	stored, inserted, err := s.SaveSignal(ctx(), post)
	if err != nil || !inserted {
		t.Fatalf("first save: inserted=%v err=%v", inserted, err)
	}

	again := post
	again.URL = "https://de.linkedin.com/feed/update/urn:li:activity:7100/"
	again.OccurredAt = post.OccurredAt.Add(37 * time.Minute)
	second, inserted, err := s.SaveSignal(ctx(), again)
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("the same post was stored a second time")
	}
	if second.ID != stored.ID {
		t.Fatalf("duplicate returned id %q, want the stored %q", second.ID, stored.ID)
	}

	timeline, err := s.Signals(ctx(), a.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 1 {
		t.Fatalf("%d signals on the timeline, want 1", len(timeline))
	}
}

func TestASignalForAnUnknownAccountIsRejectedByTheDatabase(t *testing.T) {
	// foreign_keys is a per-connection pragma in SQLite and defaults to off.
	// Forgetting it turns a bug into an orphan row that never surfaces.
	s := open(t)
	_, _, err := s.SaveSignal(ctx(), core.Signal{
		AccountID: "acc_does_not_exist", Kind: "post", Source: "linkedin",
		OccurredAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("stored a signal against an account that does not exist")
	}
}

func TestDeletingAnAccountTakesItsSignalsWithIt(t *testing.T) {
	s := open(t)
	a := account(t, s, "Acme", "")
	if _, _, err := s.SaveSignal(ctx(), core.Signal{
		AccountID: a.ID, Kind: "note", Source: "manual", Body: "x",
		OccurredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAccount(ctx(), a.ID); err != nil {
		t.Fatal(err)
	}
	n, err := s.CountSignals(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d signals survived the account, want 0", n)
	}
}

func TestTheTimelineComesBackNewestFirst(t *testing.T) {
	s := open(t)
	a := account(t, s, "Acme", "")
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, offset := range []time.Duration{0, 72 * time.Hour, 24 * time.Hour} {
		if _, _, err := s.SaveSignal(ctx(), core.Signal{
			AccountID: a.ID, Kind: "note", Source: "manual",
			Body: string(rune('a' + i)), OccurredAt: base.Add(offset),
		}); err != nil {
			t.Fatal(err)
		}
	}
	timeline, err := s.Signals(ctx(), a.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(timeline); i++ {
		if timeline[i-1].OccurredAt.Before(timeline[i].OccurredAt) {
			t.Fatalf("timeline is not newest first: %v then %v",
				timeline[i-1].OccurredAt, timeline[i].OccurredAt)
		}
	}
}

func TestStoredRulesOverrideDefaultsAndUnknownKindsAreAdded(t *testing.T) {
	s := open(t)
	if err := s.SaveRule(ctx(), core.Rule{Kind: "post", Weight: 1, HalfLifeDays: 3, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRule(ctx(), core.Rule{Kind: "rfp_published", Weight: 40, HalfLifeDays: 30, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	rules, err := s.Rules(ctx())
	if err != nil {
		t.Fatal(err)
	}
	set := core.NewRuleSet(rules)
	if got := set["post"]; got.Weight != 1 || got.Enabled {
		t.Fatalf("post rule = %+v, want the stored override", got)
	}
	if got := set["rfp_published"]; got.Weight != 40 {
		t.Fatalf("rfp_published rule = %+v, want the added rule", got)
	}
	if got := set["funding"]; got.Weight != 20 {
		t.Fatalf("funding rule = %+v, want the untouched default", got)
	}
}

func TestDeletingAnOverrideBringsTheDefaultBack(t *testing.T) {
	s := open(t)
	if err := s.SaveRule(ctx(), core.Rule{Kind: "post", Weight: 999, HalfLifeDays: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRule(ctx(), "post"); err != nil {
		t.Fatal(err)
	}
	rules, err := s.Rules(ctx())
	if err != nil {
		t.Fatal(err)
	}
	if got := core.NewRuleSet(rules)["post"]; got.Weight == 999 {
		t.Fatal("the override survived its deletion")
	}
}

func TestATokenAuthenticatesOnceIssuedAndNeverAfterRevocation(t *testing.T) {
	s := open(t)
	issued, secret, err := s.CreateToken(ctx(), "extension on the laptop")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Authenticate(ctx(), secret); err != nil || got.ID != issued.ID {
		t.Fatalf("authenticate: %v, %+v", err, got)
	}
	if err := s.RevokeToken(ctx(), issued.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx(), secret); !errors.Is(err, ErrBadToken) {
		t.Fatalf("a revoked token still authenticates: %v", err)
	}
}

func TestTheSecretIsNotInTheDatabase(t *testing.T) {
	s := open(t)
	_, secret, err := s.CreateToken(ctx(), "laptop")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM tokens WHERE hash = ?`, secret).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("the token secret is stored in clear text")
	}
}

func TestAnUnknownSecretIsRefused(t *testing.T) {
	s := open(t)
	if _, _, err := s.CreateToken(ctx(), "laptop"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx(), "vgl_not-a-real-secret"); !errors.Is(err, ErrBadToken) {
		t.Fatalf("err = %v, want ErrBadToken", err)
	}
}

func TestUsingATokenRecordsWhenItWasLastUsed(t *testing.T) {
	s := open(t)
	issued, secret, err := s.CreateToken(ctx(), "laptop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx(), secret); err != nil {
		t.Fatal(err)
	}
	list, err := s.Tokens(ctx())
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range list {
		if tk.ID == issued.ID && tk.LastUsedAt.IsZero() {
			t.Fatal("last_used_at stayed empty after a successful authentication")
		}
	}
}

func TestTwoAccountsCannotShareOnePersonProfile(t *testing.T) {
	s := open(t)
	one := account(t, s, "Acme", "")
	two := account(t, s, "Globex", "")
	if _, err := s.SavePerson(ctx(), core.Person{
		AccountID: one.ID, Name: "Jane Doe", Profile: "https://www.linkedin.com/in/jane-doe/",
	}); err != nil {
		t.Fatal(err)
	}
	// The same person captured while viewing the other company must move, not
	// duplicate: two rows would double every signal attached by profile.
	moved, err := s.SavePerson(ctx(), core.Person{
		AccountID: two.ID, Name: "Jane Doe", Profile: "https://de.linkedin.com/in/Jane-Doe",
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.AccountID != two.ID {
		t.Fatalf("person stayed on %s, want %s", moved.AccountID, two.ID)
	}
	if people, err := s.People(ctx(), one.ID); err != nil || len(people) != 0 {
		t.Fatalf("old account still holds %d people (err %v)", len(people), err)
	}
}
