package greener

import (
	"testing"
)

func TestTrimBearer(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"No space", "Bearerabc123", ""},
		{"Standard Bearer", "Bearer abc123", "abc123"},
		{"Lowercase bearer", "bearer abc123", "abc123"},
		{"Mixed Case", "BEARER abc123", "abc123"},
		{"Extra Spaces", "  Bearer     abc123   ", "abc123"},
		{"No Prefix", "abc123", "abc123"},
		{"Empty String", "", ""},
		{"Only Bearer 1", "Bearer ", ""},
		{"Only Bearer 2", "Bearer", ""},
		{"Non-standard Spacing", "Bearer    abc123", "abc123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := trimBearer(tc.input)
			if result != tc.expected {
				t.Errorf("Failed %s: expected %q, got %q", tc.name, tc.expected, result)
			}
		})
	}
}
