package lspsymbols

import (
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

func TestNamePathMatcher_Regex(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		isRegex   bool
		nameParts []string
		want      bool
		wantErr   bool
	}{
		{
			name:      "Exact match without regex",
			pattern:   "pkg/Class/Method",
			isRegex:   false,
			nameParts: []string{"pkg", "Class", "Method"},
			want:      true,
		},
		{
			name:      "Regex match leaf",
			pattern:   ".*Method",
			isRegex:   true,
			nameParts: []string{"pkg", "Class", "MyMethod"},
			want:      true,
		},
		{
			name:      "Regex match hierarchy",
			pattern:   "pkg/.*/Method",
			isRegex:   true,
			nameParts: []string{"pkg", "Class", "Method"},
			want:      true,
		},
		{
			name:      "Regex no match",
			pattern:   "^Test.*",
			isRegex:   true,
			nameParts: []string{"pkg", "Class", "Method"},
			want:      false,
		},
		{
			name:      "Regex start anchor",
			pattern:   "^pkg",
			isRegex:   true,
			nameParts: []string{"pkg", "Class", "Method"},
			want:      true,
		},
		{
			name:    "Invalid regex",
			pattern: "[",
			isRegex: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newNamePathMatcher(tt.pattern, false, tt.isRegex)
			if (err != nil) != tt.wantErr {
				t.Errorf("newNamePathMatcher() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got := m.matches(tt.nameParts, nil, ""); got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSymbolKindFromName(t *testing.T) {
	k, ok := symbolKindFromName("Function")
	if !ok || k != protocol.Function {
		t.Errorf("symbolKindFromName(Function) = %v, %v", k, ok)
	}
}
