package cmd

import (
	"testing"
)

func TestParseRecent(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"12m", false},
		{"6m", false},
		{"1y", false},
		{"2y", false},
		{"abc", true},
		{"0m", true},
		{"-1m", true},
	}

	for _, tt := range tests {
		from, to, err := parseRecent(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseRecent(%q) expected error, got from=%q to=%q", tt.input, from, to)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseRecent(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if from == "" || to == "" {
			t.Errorf("parseRecent(%q) returned empty dates: from=%q to=%q", tt.input, from, to)
		}
	}
}

func TestSearchValidation(t *testing.T) {
	// No search terms
	rootCmd.SetArgs([]string{"search"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when no search terms provided")
	}

	// --recent with --from
	rootCmd.SetArgs([]string{"search", "-k", "test", "--recent", "12m", "--from", "2024-01"})
	err = rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --recent used with --from")
	}
}
