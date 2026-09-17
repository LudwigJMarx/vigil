package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The Go server and the TypeScript extension both reduce a LinkedIn URL to the
// form accounts are keyed on. Two implementations of one rule drift, and the
// drift is invisible: the server simply starts holding two accounts for one
// company. Both sides read the same table, so drift fails a build instead.
//
// The matching test is extension/test/guess.test.ts.
func TestNormalizeProfileAgreesWithTheSharedTable(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "profile-normalisation.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the shared table is missing: %v", err)
	}
	var table struct {
		Cases []struct {
			Input string `json:"input"`
			Want  string `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Cases) == 0 {
		// An empty table would pass silently and prove nothing.
		t.Fatalf("%s holds no cases", path)
	}

	for _, c := range table.Cases {
		if got := NormalizeProfile(c.Input); got != c.Want {
			t.Errorf("NormalizeProfile(%q) = %q, want %q", c.Input, got, c.Want)
		}
	}
	t.Logf("%d shared case(s) checked against the TypeScript side", len(table.Cases))
}
