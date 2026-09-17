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
// When the signal carries a URL, the URL alone identifies it, together with
// kind and source. OccurredAt is deliberately left out in that case, because
// LinkedIn renders relative times ("2d") that resolve to a slightly different
// instant on every capture. Folding that jitter into the identity would make
// every re-capture a new signal, which is the exact failure the fingerprint
// exists to prevent.
//
// Without a URL there is nothing stable left, so the timestamp and the text go
// in and a re-capture with a shifted timestamp does count as a new signal. That
// is the honest outcome: vigil cannot tell those apart, and pretending
// otherwise would silently merge two different notes.
func Fingerprint(s Signal) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	canonical := canonicalSignalURL(s.URL)
	if canonical != "" {
		write("v1", s.Source, s.Kind, canonical)
	} else {
		write("v1", s.Source, s.Kind, s.AccountID, s.PersonID,
			s.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
			strings.TrimSpace(s.Title), strings.TrimSpace(s.Body))
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
