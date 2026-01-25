// Package example demonstrates the testing patterns used in this project.
// This package serves as a reference for writing unit tests alongside code.
package example

import (
	"errors"
	"fmt"
)

// ErrInvalidInput is returned when input validation fails.
var ErrInvalidInput = errors.New("invalid input")

// Calculator provides basic arithmetic operations.
// It demonstrates a simple struct that can be tested.
type Calculator struct {
	// Precision sets the number of decimal places for results.
	Precision int
}

// NewCalculator creates a new Calculator with default settings.
func NewCalculator() *Calculator {
	return &Calculator{Precision: 2}
}

// Add returns the sum of two numbers.
func (c *Calculator) Add(a, b float64) float64 {
	return a + b
}

// Divide returns a divided by b.
// Returns an error if b is zero.
func (c *Calculator) Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, fmt.Errorf("%w: division by zero", ErrInvalidInput)
	}
	return a / b, nil
}

// Greeter demonstrates testing with dependencies.
type Greeter struct {
	// Formatter is used to format the greeting.
	// This can be mocked in tests.
	Formatter GreetingFormatter
}

// GreetingFormatter defines how greetings are formatted.
type GreetingFormatter interface {
	Format(name string) string
}

// NewGreeter creates a Greeter with the given formatter.
func NewGreeter(f GreetingFormatter) *Greeter {
	return &Greeter{Formatter: f}
}

// Greet returns a formatted greeting for the given name.
func (g *Greeter) Greet(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: name cannot be empty", ErrInvalidInput)
	}
	return g.Formatter.Format(name), nil
}

// DefaultFormatter provides a simple greeting format.
type DefaultFormatter struct{}

// Format returns a greeting in the form "Hello, {name}!".
func (f *DefaultFormatter) Format(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
