package core

import (
	"math"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

func rules(rs ...Rule) RuleSet { return NewRuleSet(rs) }

func sig(id, kind string, occurred time.Time) Signal {
	return Signal{ID: id, AccountID: "acc", Kind: kind, OccurredAt: occurred}
}

func near(t *testing.T, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("got %.4f, want %.4f (tolerance %.4f)", got, want, tol)
	}
}

func TestFreshSignalScoresItsFullWeight(t *testing.T) {
	s := ScoreAccount("acc", []Signal{sig("a", "post", now)},
		rules(Rule{Kind: "post", Weight: 6, HalfLifeDays: 14, Enabled: true}), now)
	near(t, s.Total, 6, 1e-9)
}

func TestOneHalfLifeHalvesTheWeight(t *testing.T) {
	s := ScoreAccount("acc", []Signal{sig("a", "post", now.Add(-14*Day))},
		rules(Rule{Kind: "post", Weight: 6, HalfLifeDays: 14, Enabled: true}), now)
	near(t, s.Total, 3, 1e-9)
	near(t, s.Contributions[0].AgeDays, 14, 1e-9)
}

func TestHalfLifeZeroMeansNoDecay(t *testing.T) {
	s := ScoreAccount("acc", []Signal{sig("a", "funding", now.Add(-3650*Day))},
		rules(Rule{Kind: "funding", Weight: 20, HalfLifeDays: 0, Enabled: true}), now)
	near(t, s.Total, 20, 1e-9)
}

func TestAFutureDatedSignalIsNotAmplified(t *testing.T) {
	// A scraper reading "2d" against the wrong timezone produces signals dated
	// tomorrow. With pow(0.5, negative) that would score 2^n times its weight,
	// so the hottest account would be the one with the worst clock.
	s := ScoreAccount("acc", []Signal{sig("a", "post", now.Add(30*Day))},
		rules(Rule{Kind: "post", Weight: 6, HalfLifeDays: 14, Enabled: true}), now)
	near(t, s.Total, 6, 1e-9)
}

func TestAnUnknownKindIsReportedNotSilentlyDropped(t *testing.T) {
	// The house rule this encodes: a zero because nothing happened and a zero
	// because the scorer did not understand anything must not look the same.
	s := ScoreAccount("acc", []Signal{sig("a", "telepathy", now)},
		rules(Rule{Kind: "post", Weight: 6, HalfLifeDays: 14, Enabled: true}), now)
	near(t, s.Total, 0, 1e-9)
	if len(s.Contributions) != 0 {
		t.Fatalf("unknown kind contributed: %+v", s.Contributions)
	}
	if want := []string{"telepathy"}; len(s.Unscored) != 1 || s.Unscored[0] != want[0] {
		t.Fatalf("Unscored = %v, want %v", s.Unscored, want)
	}
}

func TestADisabledRuleLeavesItsSignalsUnscoredAndSaysSo(t *testing.T) {
	s := ScoreAccount("acc", []Signal{sig("a", "post", now)},
		rules(Rule{Kind: "post", Weight: 6, HalfLifeDays: 14, Enabled: false}), now)
	near(t, s.Total, 0, 1e-9)
	if len(s.Unscored) != 1 || s.Unscored[0] != "post" {
		t.Fatalf("Unscored = %v, want [post]", s.Unscored)
	}
}

func TestAZeroWeightRuleIsScoredAtZeroNotReportedAsUnknown(t *testing.T) {
	// "note" exists so a human remark lands in the timeline. The scorer knows
	// it and values it at nothing; that is not the same as not knowing it.
	s := ScoreAccount("acc", []Signal{sig("a", "note", now)},
		rules(Rule{Kind: "note", Weight: 0, HalfLifeDays: 365, Enabled: true}), now)
	near(t, s.Total, 0, 1e-9)
	if len(s.Unscored) != 0 {
		t.Fatalf("Unscored = %v, want empty", s.Unscored)
	}
}

func TestContributionsComeBackStrongestFirst(t *testing.T) {
	s := ScoreAccount("acc", []Signal{
		sig("weak", "reaction", now),
		sig("strong", "job_change", now),
		sig("middle", "post", now),
	}, NewRuleSet(DefaultRules()), now)

	want := []string{"strong", "middle", "weak"}
	if len(s.Contributions) != len(want) {
		t.Fatalf("got %d contributions, want %d", len(s.Contributions), len(want))
	}
	for i, id := range want {
		if s.Contributions[i].SignalID != id {
			t.Fatalf("position %d is %q, want %q", i, s.Contributions[i].SignalID, id)
		}
	}
}

func TestTheTotalIsTheSumOfItsContributions(t *testing.T) {
	// A score nobody can take apart is a score nobody can argue with.
	signals := []Signal{
		sig("a", "job_change", now.Add(-10*Day)),
		sig("b", "post", now.Add(-3*Day)),
		sig("c", "hiring", now.Add(-40*Day)),
	}
	s := ScoreAccount("acc", signals, NewRuleSet(DefaultRules()), now)

	var sum float64
	for _, c := range s.Contributions {
		sum += c.Points
		near(t, c.Points, c.Weight*c.Decay, 1e-9)
	}
	near(t, s.Total, sum, 1e-9)
}

func TestALaterRuleOverridesAnEarlierOneOfTheSameKind(t *testing.T) {
	set := NewRuleSet(append(DefaultRules(),
		Rule{Kind: "post", Weight: 99, HalfLifeDays: 0, Enabled: true}))
	s := ScoreAccount("acc", []Signal{sig("a", "post", now.Add(-100*Day))}, set, now)
	near(t, s.Total, 99, 1e-9)
}

func TestEveryDefaultRuleHasAKindAndNoNegativeHalfLife(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range DefaultRules() {
		if r.Kind == "" {
			t.Fatal("a default rule has no kind")
		}
		if seen[r.Kind] {
			t.Fatalf("kind %q appears twice in the defaults", r.Kind)
		}
		seen[r.Kind] = true
		if r.HalfLifeDays < 0 {
			t.Fatalf("kind %q has a negative half-life", r.Kind)
		}
	}
}
