// Package main is the entry point for the cloudcoop CLI/TUI application.
package main

import (
	"fmt"
	"os"

	"github.com/cloud-coop/cloudcoop/internal/apperrors"
	"github.com/cloud-coop/cloudcoop/internal/cli"
	"github.com/cloud-coop/cloudcoop/internal/log"
)

// Version information - injected at build time via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Initialise logging
	log.Init()

	// Set version info for CLI
	cli.SetVersionInfo(Version, Commit, BuildTime)

	// Execute the root command
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(apperrors.ExitCodeFor(err))
	}
}
