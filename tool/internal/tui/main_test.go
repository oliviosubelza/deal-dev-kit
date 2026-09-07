package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain pins the two things a golden snapshot must not depend on.
//
// hostGOOS decides whether the Engram screen prints the Windows hooks warning.
// Left as runtime.GOOS, the checked-in goldens would be whatever operating
// system last ran `-update`, and a Windows developer regenerating them would
// produce a diff that has nothing to do with their change. Pinned to linux
// here; TestTheWindowsHooksWarningIsShownOnWindowsOnly flips it deliberately.
//
// HOME and its Windows equivalents are where engram.PlanFor resolves the
// binary's destination from, so they point at a scratch directory: no test in
// this package may read the developer's real home. engramConfig redirects them
// per test as well, and this is the floor under any test that forgets.
func TestMain(m *testing.M) {
	code, err := runPinned(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tui tests:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runPinned(m *testing.M) (int, error) {
	hostGOOS = "linux"
	dir, err := os.MkdirTemp("", "deal-kit-tui-tests-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	os.Setenv("HOME", dir)
	os.Setenv("USERPROFILE", dir)
	os.Setenv("LOCALAPPDATA", filepath.Join(dir, "AppData", "Local"))
	return m.Run(), nil
}
