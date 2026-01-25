package example

import (
	"errors"
	"testing"
)

// =============================================================================
// Unit Tests - Tests alongside code in the same package
// =============================================================================
// These tests can access unexported functions and types because they are in
// the same package. Run with: go test ./internal/example/...

// TestCalculator_Add tests basic addition functionality.
func TestCalculator_Add(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{name: "positive numbers", a: 2, b: 3, expected: 5},
		{name: "negative numbers", a: -2, b: -3, expected: -5},
		{name: "mixed signs", a: 5, b: -3, expected: 2},
		{name: "with zero", a: 0, b: 5, expected: 5},
		{name: "decimals", a: 1.5, b: 2.5, expected: 4.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.Add(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Add(%v, %v) = %v; want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestCalculator_Divide tests division with error handling.
func TestCalculator_Divide(t *testing.T) {
	calc := NewCalculator()

	t.Run("valid division", func(t *testing.T) {
		result, err := calc.Divide(10, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 5 {
			t.Errorf("Divide(10, 2) = %v; want 5", result)
		}
	})

	t.Run("division by zero returns error", func(t *testing.T) {
		_, err := calc.Divide(10, 0)
		if err == nil {
			t.Fatal("expected error for division by zero")
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})
}

// =============================================================================
// Mock Types for Testing Dependencies
// =============================================================================

// mockFormatter is a test double for GreetingFormatter.
type mockFormatter struct {
	formatFunc func(name string) string
	called     bool
	lastInput  string
}

func (m *mockFormatter) Format(name string) string {
	m.called = true
	m.lastInput = name
	if m.formatFunc != nil {
		return m.formatFunc(name)
	}
	return "mocked greeting for " + name
}

// TestGreeter_Greet tests the Greeter with a mocked dependency.
func TestGreeter_Greet(t *testing.T) {
	t.Run("valid name", func(t *testing.T) {
		mock := &mockFormatter{
			formatFunc: func(name string) string {
				return "Custom: " + name
			},
		}
		greeter := NewGreeter(mock)

		result, err := greeter.Greet("Alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "Custom: Alice" {
			t.Errorf("Greet(Alice) = %q; want %q", result, "Custom: Alice")
		}
		if !mock.called {
			t.Error("formatter was not called")
		}
		if mock.lastInput != "Alice" {
			t.Errorf("formatter received %q; want %q", mock.lastInput, "Alice")
		}
	})

	t.Run("empty name returns error", func(t *testing.T) {
		mock := &mockFormatter{}
		greeter := NewGreeter(mock)

		_, err := greeter.Greet("")
		if err == nil {
			t.Fatal("expected error for empty name")
		}
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
		if mock.called {
			t.Error("formatter should not be called for invalid input")
		}
	})
}

// TestDefaultFormatter tests the production formatter.
func TestDefaultFormatter(t *testing.T) {
	f := &DefaultFormatter{}
	result := f.Format("World")
	expected := "Hello, World!"
	if result != expected {
		t.Errorf("Format(World) = %q; want %q", result, expected)
	}
}
