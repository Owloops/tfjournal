package main

import (
	"fmt"
	"os"

	"github.com/Owloops/tfjournal/cmd/root"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	root.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
