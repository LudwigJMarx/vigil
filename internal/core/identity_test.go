package core

import (
	"testing"
	"time"
)

func TestNormalizeProfileFoldsTheFormsOfTheSameURL(t *testing.T) {
	want := "linkedin.com/in/jane-doe"
	for _, raw := range []string{
		"https://www.linkedin.com/in/jane-doe/",
		"https://de.linkedin.com/in/jane-doe",
		"linkedin.com/in/Jane-Doe",
		"https://www.linkedin.com/in/jane-doe/?trk=public_profile_browsemap",
		"HTTPS://WWW.LINKEDIN.COM/in/jane-doe",
	} {
		if got := NormalizeProfile(raw); got != want {
			t.Errorf("NormalizeProfile(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeProfileRejectsWhatIsNotAProfile(t *testing.T) {
	for _, raw := range []string{
		"",
		"   ",
		"https://example.com/in/jane-doe",
		"https://linkedin.com.example.org/in/jane-doe", // lookalike host
		"https://a.b.linkedin.com/in/jane-doe",         // no such subdomain
		"https://www.linkedin.com/feed/",               // not a profile path
		"https://www.linkedin.com/in/",                 // no slug
		"https://www.linkedin.com/jobs/view/123",       // not a profile kind
	} {
		if got := NormalizeProfile(raw); got != "" {
			t.Errorf("NormalizeProfile(%q) = %q, want the empty string", raw, got)
		}
	}
}

func TestNormalizeProfileKeepsCompanyAndSchoolApart(t *testing.T) {
	cases := map[string]string{
		"https://www.linkedin.com/company/acme-gmbh/":     "linkedin.com/company/acme-gmbh",
		"https://www.linkedin.com/school/tu-berlin/":      "linkedin.com/school/tu-berlin",
		"https://www.linkedin.com/showcase/acme-cloud/":   "linkedin.com/showcase/acme-cloud",
		"https://www.linkedin.com/company/acme-gmbh/jobs": "linkedin.com/company/acme-gmbh",
	}
	for raw, want := range cases {
		if got := NormalizeProfile(raw); got != want {
			t.Errorf("NormalizeProfile(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestTheSamePostCapturedTwiceKeepsOneFingerprint(t *testing.T) {
	// LinkedIn shows "2d", which resolves to a different instant every time the
	// page is opened. If that jitter entered the identity, every re-capture
	// would add weight to the account for an event that happened once.
	first := Signal{
		Source: "linkedin", Kind: "post", AccountID: "acc",
		URL:        "https://www.linkedin.com/feed/update/urn:li:activity:7100?trk=sidebar",
		OccurredAt: time.Date(2026, 9, 15, 8, 30, 0, 0, time.UTC),
	}
	second := first
	second.URL = "https://de.linkedin.com/feed/update/urn:li:activity:7100/"
	second.OccurredAt = first.OccurredAt.Add(41 * time.Minute)
	second.Title = "a title the second capture happened to pick up"

	if Fingerprint(first) != Fingerprint(second) {
		t.Fatalf("same post got two fingerprints:\n  %s\n  %s",
			Fingerprint(first), Fingerprint(second))
	}
}

func TestADifferentKindOnTheSameURLIsADifferentSignal(t *testing.T) {
	post := Signal{Source: "linkedin", Kind: "post", URL: "linkedin.com/feed/update/urn:li:activity:7100"}
	comment := post
	comment.Kind = "comment"
	if Fingerprint(post) == Fingerprint(comment) {
		t.Fatal("a post and a comment on the same URL collapsed into one signal")
	}
}

func TestWithoutAURLTheTimestampIsPartOfTheIdentity(t *testing.T) {
	// There is nothing stable left to key on, so two manual notes a minute
	// apart stay two notes. Merging them would lose one.
	base := Signal{Source: "manual", Kind: "note", AccountID: "acc", Body: "spoke to Jane",
		OccurredAt: time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)}
	later := base
	later.OccurredAt = base.OccurredAt.Add(time.Minute)

	if Fingerprint(base) == Fingerprint(later) {
		t.Fatal("two notes a minute apart collapsed into one")
	}
	if Fingerprint(base) != Fingerprint(base) {
		t.Fatal("fingerprint is not stable for identical input")
	}
}

func TestTrackingParametersDoNotCreateANewSignal(t *testing.T) {
	a := Signal{Source: "linkedin", Kind: "post", URL: "https://www.linkedin.com/posts/acme_x?trk=abc&trackingId=zzz"}
	b := Signal{Source: "linkedin", Kind: "post", URL: "https://www.linkedin.com/posts/acme_x"}
	if Fingerprint(a) != Fingerprint(b) {
		t.Fatal("tracking parameters produced a second fingerprint")
	}
}

func TestAnIdentifyingQueryParameterIsKept(t *testing.T) {
	a := Signal{Source: "linkedin", Kind: "hiring", URL: "https://www.linkedin.com/jobs/view?currentJobId=1"}
	b := Signal{Source: "linkedin", Kind: "hiring", URL: "https://www.linkedin.com/jobs/view?currentJobId=2"}
	if Fingerprint(a) == Fingerprint(b) {
		t.Fatal("two different job postings collapsed into one signal")
	}
}

func TestTwoObservationsFromOneCompanyPageStayTwoSignals(t *testing.T) {
	// Found on 17.09.2026 by driving the extension's popup against a running
	// instance. A company page URL is the same for everything captured from
	// it, so the second capture of a different post came back as "already
	// known" and was dropped. Silent data loss that looks like correct
	// deduplication, which is the worst shape a bug can have here.
	seite := "https://www.linkedin.com/company/acme-gmbh/"
	erste := Signal{
		Source: "linkedin", Kind: "post", AccountID: "acc", URL: seite,
		Body:       "We are replacing our CRM this quarter.",
		OccurredAt: time.Date(2026, 9, 14, 16, 16, 0, 0, time.UTC),
	}
	zweite := erste
	zweite.Body = "Different post, two weeks later"
	zweite.OccurredAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)

	if Fingerprint(erste) == Fingerprint(zweite) {
		t.Fatal("two different observations of one company page collapsed into one signal")
	}
}

func TestResendingTheSameCapturedPageIsStillOneSignal(t *testing.T) {
	// The other half. Pressing Send twice, or capturing the same page again a
	// minute later, must not double the weight of one observation. That is why
	// the timestamp stays out of the identity even here: LinkedIn renders "2d".
	basis := Signal{
		Source: "linkedin", Kind: "post", AccountID: "acc",
		URL:        "https://www.linkedin.com/company/acme-gmbh/",
		Body:       "We are replacing our CRM this quarter.",
		OccurredAt: time.Date(2026, 9, 14, 16, 16, 0, 0, time.UTC),
	}
	nochmal := basis
	nochmal.URL = "https://de.linkedin.com/company/acme-gmbh?trk=nav"
	nochmal.OccurredAt = basis.OccurredAt.Add(43 * time.Minute)

	if Fingerprint(basis) != Fingerprint(nochmal) {
		t.Fatal("the same capture was sent twice and counted twice")
	}
}

func TestAPermalinkKeepsItsIdentityWithoutTheText(t *testing.T) {
	// A post permalink identifies the post on its own. The selection a human
	// happened to make must not turn one post into two signals.
	basis := Signal{
		Source: "linkedin", Kind: "post",
		URL:  "https://www.linkedin.com/feed/update/urn:li:activity:7100",
		Body: "the first half of the post",
	}
	andere := basis
	andere.Body = "a different part of the same post"

	if Fingerprint(basis) != Fingerprint(andere) {
		t.Fatal("one permalink produced two signals because the selection differed")
	}
}
