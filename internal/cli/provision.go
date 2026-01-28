package cli

import (
	"github.com/spf13/cobra"
)

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "VM provisioning management",
	Long:  `Commands for managing VM provisioning status and scripts.`,
}

func init() {
	provisionCmd.AddCommand(provisionStatusCmd)
	provisionCmd.AddCommand(provisionLogsCmd)
}
