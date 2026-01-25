package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

// SetVersionInfo sets the version information from build-time ldflags.
func SetVersionInfo(v, c, bt string) {
	version = v
	commit = c
	buildTime = bt
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit hash, and build time of cloudcoop.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cloudcoop %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  built:   %s\n", buildTime)
	},
}
