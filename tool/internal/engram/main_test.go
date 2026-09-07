package engram

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain makes this test binary default-deny before a single test runs.
//
// PlanFor prepends a StepBinaryDownload to any Status that carries no
// EngramPath, and that step is the one mutation that does not go through
// Runner — so no test double intercepts it. A test that only meant to exercise
// the plugin commands used to fetch the real 19.5 MB release and rename it
// over the developer's own ~/.local/bin/engram.
//
// Two things close that hole, and both are checked or set here rather than
// left to whoever writes the next test. releaseBase already refuses under `go
// test` (defaultReleaseBase), which is asserted below so the guard cannot rot
// silently; and HOME, USERPROFILE and LOCALAPPDATA — the only inputs the
// destination is derived from — point at a scratch directory, so even a
// download that somehow succeeded would land nowhere that matters. PATH is
// deliberately left alone: the destination is under a fresh temporary HOME and
// therefore cannot be on it, and the end-to-end tests run /bin/sh scripts that
// need `cat` and `sleep` to resolve.
//
// A test that wants a working download opts in explicitly with fakeRelease and
// fakeHome. One that forgets fails naming them.
func TestMain(m *testing.M) {
	code, err := runHermetic(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "engram tests:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runHermetic(m *testing.M) (int, error) {
	if releaseBase != testReleaseBase {
		return 0, fmt.Errorf("releaseBase is %q at startup, want %q:\n"+
			"the suite can reach the real release and overwrite the engram binary on this machine",
			releaseBase, testReleaseBase)
	}
	// t.TempDir and t.Setenv are unavailable here, so this is the plain
	// os-level equivalent, undone before the process exits.
	dir, err := os.MkdirTemp("", "deal-kit-engram-tests-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	os.Setenv("LOCALAPPDATA", filepath.Join(dir, "AppData", "Local"))
	return m.Run(), nil
}
