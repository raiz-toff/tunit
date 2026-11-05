package main

import "testing"

func TestSplitHeader(t *testing.T) {
	tests := []struct {
		input string
		key   string
		val   string
		isNil bool
	}{
		{"Content-Type: application/json", "Content-Type", "application/json", false},
		{"Authorization: Bearer tok123", "Authorization", "Bearer tok123", false},
		{"X-Key:value-no-space", "X-Key", "value-no-space", false},
		{"X-Key:  extra-spaces", "X-Key", "extra-spaces", false},
		{"no-colon-at-all", "", "", true},
		{":", "", "", false},
	}
	for _, tt := range tests {
		got := splitHeader(tt.input)
		if tt.isNil {
			if got != nil {
				t.Errorf("splitHeader(%q) = %v, want nil", tt.input, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("splitHeader(%q) = nil, want [%q, %q]", tt.input, tt.key, tt.val)
			continue
		}
		if len(got) != 2 {
			t.Errorf("splitHeader(%q) len = %d, want 2", tt.input, len(got))
			continue
		}
		if got[0] != tt.key || got[1] != tt.val {
			t.Errorf("splitHeader(%q) = [%q, %q], want [%q, %q]", tt.input, got[0], got[1], tt.key, tt.val)
		}
	}
}
