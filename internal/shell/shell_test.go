package shell

import "testing"

func TestEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"", "''"},
		{"it's", `'it'"'"'s'`},
		{"spaces here", "'spaces here'"},
		{"a'b'c", `'a'"'"'b'"'"'c'`},
	}
	for _, tt := range tests {
		got := Escape(tt.input)
		if got != tt.want {
			t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
