package testutil

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockModel is a simple tea.Model for testing the TestModel wrapper.
type mockModel struct {
	value   string
	width   int
	height  int
	counter int
}

func newMockModel() mockModel {
	return mockModel{value: "initial"}
}

func (m mockModel) Init() tea.Cmd {
	return nil
}

func (m mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.value = "submitted"
			m.counter++
		case tea.KeyEscape:
			m.value = "canceled"
		case tea.KeyRunes:
			m.value += string(msg.Runes)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m mockModel) View() string {
	return fmt.Sprintf("Value: %s\nCounter: %d\nSize: %dx%d", m.value, m.counter, m.width, m.height)
}

func TestTestModel_NewTestModel(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	if tm.Model() == nil {
		t.Error("model should not be nil")
	}
	if tm.View() == "" {
		t.Error("view should not be empty after initialization")
	}
}

func TestTestModel_Update(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	tm.AssertViewContains("submitted")
	tm.AssertViewContains("Counter: 1")
}

func TestTestModel_SendKey(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.SendKey(KeyEnter)
	tm.AssertViewContains("submitted")
}

func TestTestModel_SendKeyType(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.SendKeyType(tea.KeyEscape)
	tm.AssertViewContains("canceled")
}

func TestTestModel_SendRune(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.SendRune('X')
	tm.AssertViewContains("initialX")
}

func TestTestModel_SendString(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.SendString("abc")
	tm.AssertViewContains("initialabc")
}

func TestTestModel_SendWindowSize(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.SendWindowSize(80, 24)
	tm.AssertViewContains("Size: 80x24")
}

func TestTestModel_AssertViewNotContains(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	// Should pass - "nonexistent" is not in the view
	tm.AssertViewNotContains("nonexistent")
}

func TestTestModel_AssertViewMatches(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	tm.AssertViewMatches("Value:", "Counter:", "Size:")
}

func TestTestModel_GetViewLines(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	lines := tm.GetViewLines()
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestTestModel_GetViewLine(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	line := tm.GetViewLine(0)
	if line != "Value: initial" {
		t.Errorf("unexpected first line: %s", line)
	}

	// Out of bounds should return empty string
	emptyLine := tm.GetViewLine(100)
	if emptyLine != "" {
		t.Errorf("expected empty string for out of bounds, got: %s", emptyLine)
	}
}

func TestTestModel_AssertViewLines(t *testing.T) {
	model := newMockModel()
	tm := NewTestModel(t, model)

	// Should pass - model has 3 lines
	tm.AssertViewLines(3)
}

func TestKey(t *testing.T) {
	key := Key('a')
	if key.Type != tea.KeyRunes {
		t.Errorf("expected KeyRunes type")
	}
	if len(key.Runes) != 1 || key.Runes[0] != 'a' {
		t.Errorf("unexpected rune: %v", key.Runes)
	}
}

func TestKeyWithAlt(t *testing.T) {
	key := KeyWithAlt('x')
	if !key.Alt {
		t.Error("expected Alt modifier")
	}
	if len(key.Runes) != 1 || key.Runes[0] != 'x' {
		t.Errorf("unexpected rune: %v", key.Runes)
	}
}

func TestCommonKeys(t *testing.T) {
	tests := []struct {
		name    string
		key     tea.Key
		keyType tea.KeyType
	}{
		{"KeyEnter", KeyEnter, tea.KeyEnter},
		{"KeyEsc", KeyEsc, tea.KeyEscape},
		{"KeyTab", KeyTab, tea.KeyTab},
		{"KeyUp", KeyUp, tea.KeyUp},
		{"KeyDown", KeyDown, tea.KeyDown},
		{"KeyLeft", KeyLeft, tea.KeyLeft},
		{"KeyRight", KeyRight, tea.KeyRight},
		{"KeyCtrlC", KeyCtrlC, tea.KeyCtrlC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key.Type != tt.keyType {
				t.Errorf("expected %v, got %v", tt.keyType, tt.key.Type)
			}
		})
	}
}
