package core

import (
	"math"
	"sort"
	"time"
)

// Day is the unit every half-life is expressed in.
const Day = 24 * time.Hour

// DefaultRules is the starting model. The weights are not measured truth, they
// are a defensible opening position: a signal is worth more the closer it sits
// to a buying decision, and every signal fades, because a post from March says
// nothing about this week. Operators are expected to change these; the API
// exposes them for exactly that reason.
func DefaultRules() []Rule {
	return []Rule{
		{Kind: "job_change", Weight: 25, HalfLifeDays: 60, Enabled: true,
			Note: "new role, new budget, no incumbent loyalty yet"},
		{Kind: "funding", Weight: 20, HalfLifeDays: 90, Enabled: true,
			Note: "money raised is money about to be spent"},
		{Kind: "hiring", Weight: 12, HalfLifeDays: 45, Enabled: true,
			Note: "a role opened is a problem stated in public"},
		{Kind: "competitor_interaction", Weight: 18, HalfLifeDays: 21, Enabled: true,
			Note: "engaging with a competitor is the cheapest early warning there is"},
		{Kind: "post", Weight: 6, HalfLifeDays: 14, Enabled: true,
			Note: "what they chose to say this week"},
		{Kind: "comment", Weight: 4, HalfLifeDays: 14, Enabled: true,
			Note: "weaker than a post: the topic was someone else's"},
		{Kind: "reaction", Weight: 1, HalfLifeDays: 10, Enabled: true,
			Note: "near noise on its own, useful in volume"},
		{Kind: "profile_view", Weight: 8, HalfLifeDays: 7, Enabled: true,
			Note: "they looked at you, which is the rarest signal and the shortest lived"},
		{Kind: "note", Weight: 0, HalfLifeDays: 365, Enabled: true,
			Note: "a human note belongs in the timeline, not in the score"},
	}
}

// RuleSet indexes rules by kind for lookup.
type RuleSet map[string]Rule

// NewRuleSet indexes rules by kind. A later rule with the same kind replaces an
// earlier one, so a stored override beats a default.
func NewRuleSet(rules []Rule) RuleSet {
	set := make(RuleSet, len(rules))
	for _, r := range rules {
		set[r.Kind] = r
	}
	return set
}

// decay returns the fraction of a signal's weight still standing after age.
// A half-life of zero or less means the signal does not fade at all; a negative
// age (a signal dated in the future, which a careless scraper will produce) is
// treated as fresh rather than amplified.
func decay(age time.Duration, halfLifeDays float64) float64 {
	if halfLifeDays <= 0 {
		return 1
	}
	days := age.Hours() / 24
	if days <= 0 {
		return 1
	}
	return math.Pow(0.5, days/halfLifeDays)
}

// ScoreAccount adds up what an account's signals are worth at time `at`.
//
// The result carries every contribution and every signal kind that no enabled
// rule covered. That second list is the point: a total of 0 because nothing
// happened and a total of 0 because every signal had an unknown kind look
// identical otherwise, and only one of them means "nothing happened".
func ScoreAccount(accountID string, signals []Signal, rules RuleSet, at time.Time) Score {
	score := Score{AccountID: accountID, At: at}
	unscored := map[string]bool{}

	for _, s := range signals {
		rule, known := rules[s.Kind]
		if !known || !rule.Enabled {
			unscored[s.Kind] = true
			continue
		}
		if rule.Weight == 0 {
			continue
		}
		age := at.Sub(s.OccurredAt)
		d := decay(age, rule.HalfLifeDays)
		points := rule.Weight * d
		score.Contributions = append(score.Contributions, Contribution{
			SignalID: s.ID,
			Kind:     s.Kind,
			Weight:   rule.Weight,
			AgeDays:  age.Hours() / 24,
			Decay:    d,
			Points:   points,
		})
		score.Total += points
	}

	// Biggest contribution first: the answer to "why is this account hot" is
	// the first line, not a line the reader has to hunt for.
	sort.SliceStable(score.Contributions, func(i, j int) bool {
		if score.Contributions[i].Points != score.Contributions[j].Points {
			return score.Contributions[i].Points > score.Contributions[j].Points
		}
		return score.Contributions[i].SignalID < score.Contributions[j].SignalID
	})

	for kind := range unscored {
		score.Unscored = append(score.Unscored, kind)
	}
	sort.Strings(score.Unscored)
	return score
}
