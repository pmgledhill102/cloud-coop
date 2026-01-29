// Package terminal provides terminal emulator configuration generators
// for multi-agent viewing.
package terminal

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

// Grid represents a terminal pane grid layout.
type Grid struct {
	Cols int
	Rows int
}

// Config contains configuration for generating terminal configs.
type Config struct {
	Format   string          // terminal format: ghostty, iterm2, kitty
	Grid     Grid            // pane grid layout
	Host     string          // VM host (IP or hostname)
	User     string          // SSH username
	Port     int             // SSH port
	Session  string          // tmux session name
	Sessions []agent.Session // agent sessions to connect to
}

// ValidFormats lists all supported terminal formats.
var ValidFormats = []string{"ghostty", "iterm2", "kitty"}

// IsValidFormat checks if the given format is supported.
func IsValidFormat(format string) bool {
	format = strings.ToLower(format)
	for _, f := range ValidFormats {
		if f == format {
			return true
		}
	}
	return false
}

// ParseGrid parses a grid specification string like "4x3" (cols x rows).
func ParseGrid(spec string) (Grid, error) {
	parts := strings.SplitN(strings.ToLower(spec), "x", 2)
	if len(parts) != 2 {
		return Grid{}, fmt.Errorf("invalid grid format %q: expected COLSxROWS (e.g., 4x3)", spec)
	}

	cols, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || cols < 1 {
		return Grid{}, fmt.Errorf("invalid column count %q: must be a positive integer", parts[0])
	}

	rows, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || rows < 1 {
		return Grid{}, fmt.Errorf("invalid row count %q: must be a positive integer", parts[1])
	}

	return Grid{Cols: cols, Rows: rows}, nil
}

// CalculateGrid determines the best grid layout for the given number of agents.
// It tries to create a relatively square grid that fits all agents.
func CalculateGrid(count int) Grid {
	if count <= 0 {
		return Grid{Cols: 1, Rows: 1}
	}
	if count == 1 {
		return Grid{Cols: 1, Rows: 1}
	}
	if count == 2 {
		return Grid{Cols: 2, Rows: 1}
	}

	// Calculate the square root and round to get a good starting point
	sqrt := math.Sqrt(float64(count))
	cols := int(math.Ceil(sqrt))
	rows := int(math.Ceil(float64(count) / float64(cols)))

	return Grid{Cols: cols, Rows: rows}
}

// Generate creates terminal configuration for the specified format.
func Generate(cfg Config) (string, error) {
	switch strings.ToLower(cfg.Format) {
	case "ghostty":
		return generateGhostty(cfg), nil
	case "iterm2":
		return generateITerm2(cfg), nil
	case "kitty":
		return generateKitty(cfg), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", cfg.Format)
	}
}
