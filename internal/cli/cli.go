// Package cli is the command line. It is a package rather than a main so the
// commands can be run in a test the same way a shell runs them, arguments in
// and exit code out.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/LudwigJMarx/vigil/internal/store"
)

const usage = `vigil - watch the accounts you sell to, and decide yourself when to write.

Usage:
  vigil serve [flags]              run the API and the operator UI
  vigil token create --name NAME   issue an API token (printed once)
  vigil token list                 list tokens without their secrets
  vigil token revoke ID            disable a token
  vigil version                    print the version and the database path
  vigil help                       print this text

Common flags:
  --db PATH     database file (env VIGIL_DB, default %s)

Run "vigil <command> --help" for the flags of one command.
`

// Version is what the binary reports about itself. The release workflow stamps
// it with -ldflags "-X ...cli.Version=..."; everything else falls back to
// versionFrom below.
var Version = versionFrom(stamped, buildInfoVersion())

// stamped is what -ldflags writes into. It stays empty in any build that does
// not pass the flag, which is every build a user makes themselves.
var stamped string

// versionFrom decides what to report, and is a pure function so the decision
// has a test that does not need three different kinds of build.
//
// The order matters. A release binary carries the stamp and reports it. A
// binary from `go install module@v0.1.0` carries no stamp, but the module
// system knows the version, so it reports that: until 17.09.2026 it said
// "dev", which meant /healthz answered "dev" on an installed release and every
// bug report from such an instance named no version at all. A build from a
// working tree knows neither and says "dev", which is the honest answer rather
// than a number that looks like a release.
func versionFrom(stamp, ausDemModul string) string {
	if stamp != "" {
		return stamp
	}
	if ausDemModul != "" && ausDemModul != "(devel)" {
		return strings.TrimPrefix(ausDemModul, "v")
	}
	return "dev"
}

func buildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// Run executes one command. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, usage, defaultDBPath())
		return 2
	}
	command, rest := args[0], args[1:]
	var err error
	switch command {
	case "serve":
		err = runServe(rest, stdout, stderr)
	case "token":
		err = runToken(rest, stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "vigil %s\ndatabase: %s\n", Version, resolveDBPath(""))
		return 0
	case "help", "-h", "--help":
		fmt.Fprintf(stdout, usage, defaultDBPath())
		return 0
	default:
		fmt.Fprintf(stderr, "vigil: unknown command %q\n\n", command)
		fmt.Fprintf(stderr, usage, defaultDBPath())
		return 2
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "vigil: %v\n", err)
		return 1
	}
	return 0
}

// defaultDBPath is where the database lands when nothing says otherwise:
// under the user's data directory, not the working directory. A database that
// follows the shell's current directory is a database that exists in several
// copies within a week.
func defaultDBPath() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "vigil", "vigil.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "vigil.db")
	}
	return filepath.Join(home, ".local", "share", "vigil", "vigil.db")
}

func resolveDBPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("VIGIL_DB"); env != "" {
		return env
	}
	return defaultDBPath()
}

func openStore(path string) (*store.Store, error) {
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("create %s: %w", dir, err)
			}
		}
	}
	return store.Open(path)
}

func runToken(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("token: expected create, list or revoke")
	}
	set := flag.NewFlagSet("vigil token "+args[0], flag.ContinueOnError)
	set.SetOutput(stderr)
	dbPath := set.String("db", "", "database file")
	name := set.String("name", "", "what will use this token")
	if err := set.Parse(args[1:]); err != nil {
		return err
	}

	db, err := openStore(resolveDBPath(*dbPath))
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()

	switch args[0] {
	case "create":
		token, secret, err := db.CreateToken(ctx, *name)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n", secret)
		fmt.Fprintf(stderr,
			"issued token %s (%s). It is stored as a hash: this is the only time it is printed.\n",
			token.ID, token.Name)
		return nil

	case "list":
		tokens, err := db.Tokens(ctx)
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			// Not silence. An empty list and a failed query must not look alike.
			fmt.Fprintln(stdout, "no tokens issued yet (vigil token create --name browser)")
			return nil
		}
		for _, t := range tokens {
			state := "active"
			if !t.RevokedAt.IsZero() {
				state = "revoked"
			}
			last := "never used"
			if !t.LastUsedAt.IsZero() {
				last = "last used " + t.LastUsedAt.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(stdout, "%s  %-24s %-8s %s\n", t.ID, t.Name, state, last)
		}
		fmt.Fprintf(stdout, "%d token(s)\n", len(tokens))
		return nil

	case "revoke":
		rest := set.Args()
		if len(rest) != 1 {
			return errors.New("token revoke: expected exactly one token id")
		}
		if err := db.RevokeToken(ctx, rest[0]); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("no active token with id %q", rest[0])
			}
			return err
		}
		fmt.Fprintf(stdout, "revoked %s\n", rest[0])
		return nil

	default:
		return fmt.Errorf("token: unknown subcommand %q", args[0])
	}
}

// loopback reports whether an address binds to this machine only. Anything
// else is reachable from the network and has to carry a credential.
func loopback(addr string) bool {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	host = strings.Trim(host, "[]")
	// ":8099" has an empty host, which Go's server reads as every interface.
	// Treating that as loopback would be the exact mistake this guard exists
	// to catch: the most exposed bind spelled in the shortest way.
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return strings.HasPrefix(host, "127.")
}
