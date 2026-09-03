package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakePATH replaces PATH with a directory holding stub executables, so the
// checks are tested against known output rather than whatever the machine has.
func fakePATH(t *testing.T, stubs map[string]string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs are POSIX")
	}
	dir := t.TempDir()
	for name, output := range stubs {
		script := "#!/bin/sh\necho '" + output + "'\n"
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestCheckFindsToolsAndVersions(t *testing.T) {
	fakePATH(t, map[string]string{
		"git":  "git version 2.43.0",
		"node": "v20.11.0",
		"pnpm": "9.1.0",
	})

	rep := Check(ForWeb())

	tests := []struct {
		name    string
		found   bool
		version string
	}{
		{"git", true, "2.43.0"},
		{"node", true, "20.11.0"},
		{"pnpm", true, "9.1.0"},
		{"npm", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, r := range rep.Results {
				if r.Name != tt.name {
					continue
				}
				if r.Found != tt.found {
					t.Errorf("found = %v, want %v", r.Found, tt.found)
				}
				if r.Version != tt.version {
					t.Errorf("version = %q, want %q", r.Version, tt.version)
				}
				return
			}
			t.Errorf("%q is not in the report", tt.name)
		})
	}
}

func TestMissingListsOnlyRequiredTools(t *testing.T) {
	// node is required and absent; npm is optional and absent.
	fakePATH(t, map[string]string{"git": "git version 2.43.0"})

	rep := Check(ForWeb())
	missing := rep.Missing()

	if len(missing) != 1 || missing[0].Name != "node" {
		t.Errorf("missing = %+v, want only node", names(missing))
	}
	if rep.OK() {
		t.Error("OK() is true with a required tool missing")
	}
}

func TestOKWhenEverythingRequiredIsPresent(t *testing.T) {
	fakePATH(t, map[string]string{
		"git":  "git version 2.43.0",
		"node": "v20.11.0",
	})
	rep := Check(ForWeb())
	if !rep.OK() {
		t.Errorf("OK() is false but only optional tools are missing: %+v", names(rep.Missing()))
	}
}

func TestPackageManagerPrefersPnpm(t *testing.T) {
	fakePATH(t, map[string]string{"pnpm": "9.1.0", "npm": "10.8.2"})
	rep := Check(ForWeb())
	if got, ok := rep.PackageManager(); !ok || got != "pnpm" {
		t.Errorf("PackageManager() = (%q, %v), want pnpm", got, ok)
	}
}

func TestPackageManagerFallsBackToNpm(t *testing.T) {
	fakePATH(t, map[string]string{"npm": "10.8.2"})
	rep := Check(ForWeb())
	if got, ok := rep.PackageManager(); !ok || got != "npm" {
		t.Errorf("PackageManager() = (%q, %v), want npm", got, ok)
	}
}

func TestPackageManagerWithNoneInstalled(t *testing.T) {
	fakePATH(t, map[string]string{"git": "git version 2.43.0"})
	rep := Check(ForWeb())
	if _, ok := rep.PackageManager(); ok {
		t.Error("reported a package manager with none installed")
	}
}

func TestVersionToleratesUnparseableOutput(t *testing.T) {
	// A tool that prints no version must still register as present: absence of
	// a version is not absence of the tool.
	fakePATH(t, map[string]string{"git": "some banner with no numbers"})
	rep := Check(Core())

	if !rep.Found("git") {
		t.Error("git was not detected")
	}
	if rep.Results[0].Version != "" {
		t.Errorf("version = %q, want empty", rep.Results[0].Version)
	}
}

func names(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}
