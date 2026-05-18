package main

import (
	"context"
	"os"

	"github.com/axiom-orient/providers-goa/cmd/goa/internal/cli"
)

func main() {
	os.Exit(cli.New(os.Stdout, os.Stderr).Run(context.Background(), os.Args[1:]))
}
