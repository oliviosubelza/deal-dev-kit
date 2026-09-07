package main

import (
	"os"
	"testing"
)

// TestUsageNamesWhatTheUserTyped pins that help text is built from argv[0].
// The published command is `deal` while the package directory is deal-kit, so
// a hardcoded name would print a command the reader cannot run.
func TestUsageNamesWhatTheUserTyped(t *testing.T) {
	prev := os.Args
	t.Cleanup(func() { os.Args = prev })

	for _, tc := range []struct{ argv0, want string }{
		{"/usr/local/bin/deal", "deal"},
		{`C:\Users\x\bin\deal.exe`, "deal"},
		{"./deal-kit", "deal-kit"},
		{"", "deal"},
	} {
		os.Args = []string{tc.argv0}
		if got := invoked(); got != tc.want {
			t.Errorf("invoked() with argv[0]=%q = %q, want %q", tc.argv0, got, tc.want)
		}
	}
}
