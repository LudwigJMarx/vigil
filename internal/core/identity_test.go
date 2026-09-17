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
