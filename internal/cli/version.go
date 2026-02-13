package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/version"
)

// SetVersionInfo sets the version information from build-time ldflags.
func SetVersionInfo(v, c, bt string) {
	version.Set(v, c, bt)
}

var versionCmd = &cobra.Command{
	Use:         "version",
	Short:       "Print version information",
	Long:        `Print the version, commit hash, and build time of cloudcoop.`,
	Annotations: map[string]string{"skip-config": "true"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cloudcoop %s\n", version.Version)
		fmt.Printf("  commit:  %s\n", version.Commit)
		fmt.Printf("  built:   %s\n", version.BuildTime)
	},
}
