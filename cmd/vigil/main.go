// Command vigil watches the accounts you sell to and scores the signals you
// captured yourself. It sends nothing on your behalf, and it crawls nothing:
// see docs/scope.md for why that is a design decision and not a missing
// feature.
package main

import (
	"os"

	"github.com/LudwigJMarx/vigil/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
