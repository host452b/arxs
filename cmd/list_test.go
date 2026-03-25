package cmd

import "testing"

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		// ASCII: unchanged when short enough
		{"hello", 10, "hello"},
		// ASCII: truncated with ellipsis
		{"hello world", 8, "hello..."},
		// Multi-byte rune boundary: "你好世界很好看" is 7 runes × 3 bytes = 21 bytes.
		// truncate with max=5 should give "你好..." (2 runes + "..."), not garbage bytes.
		{"你好世界很好看", 5, "你好..."},
		// Short enough: 4 runes ≤ 5, no truncation
		{"你好世界", 5, "你好世界"},
		// Exactly at boundary
		{"abcde", 5, "abcde"},
		// Empty string
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}
