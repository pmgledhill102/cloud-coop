// Package main is the entry point for the cloudcoop CLI/TUI application.
package main

import (
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
		// Usage errors (invalid commands, flags) are already printed by Cobra
		if apperrors.IsUsageError(err) {
			os.Exit(apperrors.ExitUsage)
		}

		// Log other errors with structured logging
		log.Error("command failed", log.Err(err))
		os.Exit(apperrors.ExitCodeFor(err))
	}
}
