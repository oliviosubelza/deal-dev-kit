package paths

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	roots := map[string]string{
		"src":      "src",
		"ui":       "src/shared/ui",
		"features": "src/features",
	}

	tests := []struct {
		name     string
		template string
		want     string
		wantErr  string
	}{
		{name: "single root", template: "{ui}/button.tsx", want: "src/shared/ui/button.tsx"},
		{name: "root with suffix", template: "{src}/shared/lib", want: "src/shared/lib"},
		{name: "bare root", template: "{features}", want: "src/features"},
		{name: "no placeholder", template: "docs/readme.md", want: "docs/readme.md"},
		{name: "unknown root", template: "{styles}/theme.css", wantErr: `unknown root "styles"`},
		{name: "traversal via literal", template: "../outside", wantErr: "escapes the project directory"},
		{name: "traversal via root", template: "{src}/../../etc", wantErr: "escapes the project directory"},
		{name: "absolute path", template: "/etc/passwd", wantErr: "escapes the project directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.template, roots)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}
