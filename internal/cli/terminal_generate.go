package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/terminal"
)

var (
	terminalFormat string
	terminalGrid   string
	terminalOutput string
)

var terminalGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate terminal config for multi-agent viewing",
	Long: `Generate terminal emulator configuration for multi-agent viewing.

Creates config files or shell scripts that launch your terminal with multiple
panes in a grid layout, each connected to a different agent session.

Supported formats:
  ghostty   Generates a shell script using Ghostty splits
  iterm2    Generates an AppleScript for iTerm2
  kitty     Generates a Kitty session config file

The grid layout is automatically calculated based on agent count, or you can
specify it explicitly with --grid (e.g., --grid=4x3 for 4 columns, 3 rows).

Examples:
  cloudcoop terminal generate --format=ghostty
  cloudcoop terminal generate --format=iterm2 --output=launch-agents.scpt
  cloudcoop terminal generate --format=kitty --grid=3x3
  cloudcoop terminal generate --format=ghostty > launch-agents.sh && chmod +x launch-agents.sh`,
	RunE: runTerminalGenerate,
}

func init() {
	terminalGenerateCmd.Flags().StringVarP(&terminalFormat, "format", "f", "", "terminal format: ghostty, iterm2, kitty (required)")
	terminalGenerateCmd.Flags().StringVarP(&terminalGrid, "grid", "g", "", "grid layout (e.g., 4x3), auto-calculated if not specified")
	terminalGenerateCmd.Flags().StringVarP(&terminalOutput, "output", "o", "", "output file (default: stdout)")

	_ = terminalGenerateCmd.MarkFlagRequired("format")
}

func runTerminalGenerate(cmd *cobra.Command, args []string) error {
	// Validate format
	if !terminal.IsValidFormat(terminalFormat) {
		return fmt.Errorf("invalid format %q: supported formats are ghostty, iterm2, kitty", terminalFormat)
	}

	conn, err := connectToVM(cmd)
	if err != nil {
		return err
	}
	if conn == nil {
		return nil
	}
	defer conn.Close()

	// List agent sessions
	result, err := agent.ListSessions(conn.Client, defaultSessionName)
	if err != nil {
		if errors.Is(err, agent.ErrTmuxNotInstalled) {
			fmt.Fprintln(os.Stderr, "tmux is not installed on the VM")
			return nil
		}
		return fmt.Errorf("list sessions: %w", err)
	}

	if result.NoSession {
		fmt.Fprintln(os.Stderr, "No tmux session exists on the VM")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Create agents first with:")
		fmt.Fprintln(os.Stderr, "  cloudcoop agents add")
		return nil
	}

	if len(result.Sessions) == 0 {
		fmt.Fprintln(os.Stderr, "Agents session exists but has no windows")
		return nil
	}

	// Parse grid if specified, otherwise auto-calculate
	var grid terminal.Grid
	if terminalGrid != "" {
		grid, err = terminal.ParseGrid(terminalGrid)
		if err != nil {
			return fmt.Errorf("invalid grid: %w", err)
		}
	} else {
		grid = terminal.CalculateGrid(len(result.Sessions))
	}

	// Build generator config
	genConfig := terminal.Config{
		Format:   terminalFormat,
		Grid:     grid,
		Host:     conn.IP,
		User:     conn.User,
		Port:     conn.Port,
		Session:  defaultSessionName,
		Sessions: result.Sessions,
	}

	// Generate the config
	output, err := terminal.Generate(genConfig)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	// Write output
	if terminalOutput != "" {
		if err := os.WriteFile(terminalOutput, []byte(output), 0600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Config written to: %s\n", terminalOutput)

		// Print usage hint
		printUsageHint(terminalFormat, terminalOutput)
	} else {
		fmt.Print(output)
	}

	return nil
}

func printUsageHint(format, filename string) {
	switch format {
	case "ghostty":
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To use:")
		fmt.Fprintf(os.Stderr, "  chmod +x %s && ./%s\n", filename, filename)
	case "iterm2":
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To use:")
		fmt.Fprintf(os.Stderr, "  osascript %s\n", filename)
	case "kitty":
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To use:")
		fmt.Fprintf(os.Stderr, "  kitty --session %s\n", filename)
	}
}
