package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestNoArgumentsPrintsTheUsageAndFails(t *testing.T) {
	code, _, stderr := run(t)
	if code == 0 {
		t.Fatal("exit code 0 for a call with no command")
	}
	if !strings.Contains(stderr, "vigil serve") {
		t.Fatalf("usage does not mention serve: %q", stderr)
	}
}

func TestAnUnknownCommandSaysWhichOne(t *testing.T) {
	code, _, stderr := run(t, "sync-everything")
	if code == 0 {
		t.Fatal("exit code 0 for an unknown command")
	}
	if !strings.Contains(stderr, "sync-everything") {
		t.Fatalf("the message does not name the command: %q", stderr)
	}
}

func TestTokenCreatePrintsTheSecretOnStdoutAndNothingElse(t *testing.T) {
	// The secret goes to stdout alone so `vigil token create > file` captures
	// the credential and not the explanation around it.
	db := filepath.Join(t.TempDir(), "vigil.db")
	code, stdout, stderr := run(t, "token", "create", "--db", db, "--name", "browser")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	secret := strings.TrimSpace(stdout)
	if !strings.HasPrefix(secret, "vgl_") || strings.Contains(secret, "\n") {
		t.Fatalf("stdout = %q, want one token", stdout)
	}
	if !strings.Contains(stderr, "only time it is printed") {
		t.Fatalf("stderr does not warn that the secret is not recoverable: %q", stderr)
	}
}

func TestTokenListSaysItIsEmptyRatherThanPrintingNothing(t *testing.T) {
	db := filepath.Join(t.TempDir(), "vigil.db")
	code, stdout, stderr := run(t, "token", "list", "--db", db)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "no tokens issued yet") {
		t.Fatalf("stdout = %q: silence and an empty list must not look alike", stdout)
	}
}

func TestATokenCanBeListedAndRevoked(t *testing.T) {
	db := filepath.Join(t.TempDir(), "vigil.db")
	run(t, "token", "create", "--db", db, "--name", "browser")

	_, stdout, _ := run(t, "token", "list", "--db", db)
	if !strings.Contains(stdout, "browser") || !strings.Contains(stdout, "active") {
		t.Fatalf("list = %q", stdout)
	}
	id := strings.Fields(stdout)[0]

	code, _, stderr := run(t, "token", "revoke", "--db", db, id)
	if code != 0 {
		t.Fatalf("revoke: exit %d: %s", code, stderr)
	}
	_, stdout, _ = run(t, "token", "list", "--db", db)
	if !strings.Contains(stdout, "revoked") {
		t.Fatalf("list after revoke = %q", stdout)
	}
}

func TestRevokingATokenThatDoesNotExistFails(t *testing.T) {
	db := filepath.Join(t.TempDir(), "vigil.db")
	code, _, stderr := run(t, "token", "revoke", "--db", db, "tok_nope")
	if code == 0 {
		t.Fatal("revoking an unknown id succeeded")
	}
	if !strings.Contains(stderr, "tok_nope") {
		t.Fatalf("stderr = %q, want the id named", stderr)
	}
}

func TestServeRefusesAPublicBindWithNoTokenIssued(t *testing.T) {
	// An instance on a public address with no credential is an open database.
	// The refusal is the feature: a warning in a log scrolls past.
	db := filepath.Join(t.TempDir(), "vigil.db")
	code, _, stderr := run(t, "serve", "--db", db, "--addr", "0.0.0.0:0")
	if code == 0 {
		t.Fatal("vigil started on 0.0.0.0 with no token")
	}
	if !strings.Contains(stderr, "token create") {
		t.Fatalf("the refusal does not say how to fix it: %q", stderr)
	}
}

func TestLoopbackRecognisesWhatIsReachableFromOutside(t *testing.T) {
	local := []string{"127.0.0.1:8099", "localhost:8099", "[::1]:8099", "127.0.0.53:80"}
	public := []string{":8099", "0.0.0.0:8099", "[::]:8099", "192.168.1.10:8099", "example.internal:8099"}

	for _, addr := range local {
		if !loopback(addr) {
			t.Errorf("loopback(%q) = false, want true", addr)
		}
	}
	for _, addr := range public {
		if loopback(addr) {
			t.Errorf("loopback(%q) = true, want false: it is reachable from the network", addr)
		}
	}
}

func TestVersionNamesTheDatabaseItWouldUse(t *testing.T) {
	t.Setenv("VIGIL_DB", "/tmp/somewhere/vigil.db")
	code, stdout, _ := run(t, "version")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "/tmp/somewhere/vigil.db") {
		t.Fatalf("stdout = %q, want the database path", stdout)
	}
}

func TestVersionFromPrefersTheStampThenTheModuleThenSaysDev(t *testing.T) {
	// Until 17.09.2026 a binary from `go install module@v0.1.0` reported "dev",
	// because only the release workflow passes -ldflags. /healthz answered
	// "dev" on an installed release, and every bug report from such an instance
	// named no version at all.
	cases := []struct {
		name, stamp, modul, want string
	}{
		{"release binary", "0.1.0", "v0.1.0", "0.1.0"},
		{"go install of a tagged version", "", "v0.1.0", "0.1.0"},
		{"go install keeps no leading v", "", "v1.2.3-rc1", "1.2.3-rc1"},
		{"build from a working tree", "", "(devel)", "dev"},
		{"no build info at all", "", "", "dev"},
		{"the stamp wins over the module", "0.2.0-rc", "v0.1.0", "0.2.0-rc"},
	}
	for _, c := range cases {
		if got := versionFrom(c.stamp, c.modul); got != c.want {
			t.Errorf("%s: versionFrom(%q, %q) = %q, want %q", c.name, c.stamp, c.modul, got, c.want)
		}
	}
}

func TestAnUnstampedBuildReportsDevRatherThanSomethingReleaseShaped(t *testing.T) {
	// The wiring: this test binary carries no stamp, so Version has been
	// through versionFrom for real. A test that only called versionFrom would
	// stay green if the variable stopped using it.
	if Version == "" {
		t.Fatal("Version is empty")
	}
	if stamped == "" && Version != "dev" && !strings.Contains(Version, ".") {
		t.Fatalf("Version = %q, which is neither a stamp, a module version nor dev", Version)
	}
}
