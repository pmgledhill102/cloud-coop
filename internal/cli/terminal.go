package cli

import (
	"github.com/spf13/cobra"
)

var terminalCmd = &cobra.Command{
	Use:   "terminal",
	Short: "Terminal emulator utilities",
	Long: `Terminal emulator utilities for multi-agent viewing.

Generate configuration files or scripts that launch your terminal emulator
with multiple panes, each connected to a different agent session.`,
}

func init() {
	terminalCmd.AddCommand(terminalGenerateCmd)
}
