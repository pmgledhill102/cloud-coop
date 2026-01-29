// Package shell provides shared shell utility functions.
package shell

import "strings"

// Escape escapes a string for safe use in shell commands.
// It wraps the string in single quotes and escapes any embedded single quotes.
func Escape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
