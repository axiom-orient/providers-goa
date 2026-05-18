package cli

import (
	"flag"
	"fmt"

	goa "github.com/axiom-orient/providers-goa/client"
)

func (a *App) runVersion(args []string) int {
	fs := flag.NewFlagSet("goa version", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	info := goa.BuildInfoSnapshot()
	if *asJSON {
		return a.writeJSON(info)
	}
	fmt.Fprintf(a.Stdout, "version: %s\n", info.Version)
	fmt.Fprintf(a.Stdout, "commit: %s\n", info.Commit)
	fmt.Fprintf(a.Stdout, "build_date: %s\n", info.BuildDate)
	return 0
}
