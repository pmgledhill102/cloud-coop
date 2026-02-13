package tui

import "github.com/charmbracelet/lipgloss"

// Styles for the TUI. Adaptive colours ensure readability on both light and
// dark terminal backgrounds.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#D7005F", Dark: "#FF87AF"}).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#585858", Dark: "#626262"}).
			MarginBottom(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#585858", Dark: "#626262"}).
			MarginTop(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#5F5FD7", Dark: "#5F5FD7"}).
			Padding(1, 2)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF0000"})

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#00875F", Dark: "#00FF87"})

	stoppedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#585858", Dark: "#626262"})

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#585858", Dark: "#808080"}).
			Width(14)

	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#585858", Dark: "#626262"})
)
