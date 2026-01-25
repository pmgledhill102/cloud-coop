// Package tui provides the terminal user interface for cloudcoop.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for the TUI.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginBottom(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

// Model represents the TUI application state.
type Model struct {
	width  int
	height int
	ready  bool
}

// New creates a new TUI application model.
func New() Model {
	return Model{}
}

// Init initializes the TUI application.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	title := titleStyle.Render("cloudcoop")
	subtitle := subtitleStyle.Render("Manage sandboxed AI coding agents on cloud VMs")

	content := boxStyle.Render(`Status: Ready

VMs:      0 configured
Agents:   0 running

Press 'q' to quit.`)

	help := helpStyle.Render("q: quit • ?: help • n: new VM")

	return fmt.Sprintf("\n%s\n%s\n\n%s\n\n%s\n",
		title,
		subtitle,
		content,
		help,
	)
}
