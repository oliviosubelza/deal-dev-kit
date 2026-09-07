package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/engram"
)

// TestMain makes this test binary default-deny before a single test runs, for
// the reason spelled out in internal/engram's own TestMain: engram.PlanFor
// prepends a binary download to any Status with no EngramPath, that step does
// not go through the Runner these tests replace, and it used to fetch the real
// release and rename it over the developer's ~/.local/bin/engram.
//
// The download root itself is refused inside internal/engram under `go test`,
// which covers this package too — asserted below through the only surface that
// is exported, the URL a plan carries. What is left for here is the
// destination: HOME, USERPROFILE and LOCALAPPDATA are the only inputs it is
// derived from, and they point at a scratch directory. PATH is left alone; the
// destination is under a fresh temporary HOME and cannot be on it.
func TestMain(m *testing.M) {
	code, err := runHermetic(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cli tests:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runHermetic(m *testing.M) (int, error) {
	dir, err := os.MkdirTemp("", "deal-kit-cli-tests-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	os.Setenv("LOCALAPPDATA", filepath.Join(dir, "AppData", "Local"))

	// The plan is built the way a real run builds it, so this fails if
	// internal/engram ever stops refusing downloads under test.
	p := engram.PlanFor(engram.Status{State: engram.StateReady, ClaudePath: "/fake/bin/claude"})
	if d, ok := p.Binary(); ok {
		if !strings.HasPrefix(d.URL, "http://127.0.0.1:") {
			return 0, fmt.Errorf("a planned download points at %q:\n"+
				"the suite can reach the real release and overwrite the engram binary on this machine", d.URL)
		}
		if !strings.HasPrefix(d.Dir, dir+string(filepath.Separator)) {
			return 0, fmt.Errorf("a planned download lands in %q, outside the scratch HOME %q", d.Dir, dir)
		}
	}
	return m.Run(), nil
}
