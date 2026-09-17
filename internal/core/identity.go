package core

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// NormalizeProfile reduces a LinkedIn profile or company URL to the one form
// vigil stores. Captured URLs differ by locale subdomain, tracking query and
// trailing slash while pointing at the same person, and an account matched by
// raw URL string therefore splits into several accounts over a few weeks.
//
// It returns the empty string for input it does not recognise as a LinkedIn
// profile. Callers treat that as "no profile", never as "profile: ”".
func NormalizeProfile(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "//") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	// de.linkedin.com, www.linkedin.com and linkedin.com are the same site;
	// linkedin.com.example.org is not, and neither is a.b.linkedin.com.
	host := strings.ToLower(u.Hostname())
	if host != "linkedin.com" {
		sub, ok := strings.CutSuffix(host, ".linkedin.com")
		if !ok || sub == "" || strings.Contains(sub, ".") {
			return ""
		}
	}

	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	kind, slug := strings.ToLower(parts[0]), parts[1]
	switch kind {
	case "in", "company", "school", "showcase":
	default:
		return ""
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return ""
	}
	return "linkedin.com/" + kind + "/" + slug
}

// Fingerprint is the identity of an observation, used to make ingest
// idempotent: capturing the same post twice must not double its weight.
//
// OccurredAt never enters the identity while a URL is present, because LinkedIn
// renders relative times ("2d") that resolve to a slightly different instant on
// every capture. Folding that jitter in would make every re-capture a new
// signal, which is the exact failure the fingerprint exists to prevent.
//
// What the URL is worth depends on what it points at, and getting that wrong
// costs data:
//
//   - A permalink names one item. /feed/update/urn:li:activity:7100 is that
//     post and nothing else, so source, kind and the URL are the whole
//     identity. Two people selecting different halves of it still captured one
//     post.
//   - A profile or company page names a place, not an observation. Everything
//     captured while standing on it carries the same URL. Until 17.09.2026 the
//     URL alone decided, so the second capture from a company page came back
//     "already known" and was dropped: silent loss wearing the costume of
//     correct deduplication. Found by driving the extension's popup against a
//     running instance. The title and the body now join the identity there, so
//     two observations stay two and pressing Send twice still stays one.
//   - No URL at all leaves nothing stable, so the timestamp goes in as well and
//     a re-capture with a shifted timestamp does count as new. That is the
//     honest outcome: vigil cannot tell those apart, and merging them would
//     lose one of two different notes.
func Fingerprint(s Signal) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	titel, text := strings.TrimSpace(s.Title), strings.TrimSpace(s.Body)
	canonical := canonicalSignalURL(s.URL)

	switch {
	case canonical == "":
		write("v1", s.Source, s.Kind, s.AccountID, s.PersonID,
			s.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"), titel, text)
	case NormalizeProfile(s.URL) != "":
		// NormalizeProfile answers non-empty exactly for the container pages:
		// /in/, /company/, /school/, /showcase/. Reusing it keeps one rule for
		// what those URLs are instead of a second list that drifts.
		write("v1", s.Source, s.Kind, canonical, titel, text)
	default:
		write("v1", s.Source, s.Kind, canonical)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalSignalURL strips the parts of a URL that change per visit: the
// scheme, the locale subdomain, tracking query parameters and the fragment.
func canonicalSignalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "//") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if strings.HasSuffix(host, ".linkedin.com") {
		host = "linkedin.com"
	}
	path := strings.TrimRight(u.EscapedPath(), "/")
	if path == "" {
		path = "/"
	}
	// Keep only query parameters that identify content. Everything LinkedIn
	// appends for attribution (trk, trackingId, originalSubdomain, ...) points
	// at the same item and must not create a second one.
	keep := url.Values{}
	for key, values := range u.Query() {
		switch strings.ToLower(key) {
		case "id", "urn", "activityid", "updateid", "currentjobid", "postid":
			keep[key] = values
		}
	}
	out := host + path
	if len(keep) > 0 {
		out += "?" + keep.Encode()
	}
	return out
}
