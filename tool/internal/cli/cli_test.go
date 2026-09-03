package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// touch creates a file, and any directories leading to it.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectRootFindsTheNearestProject(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "package.json"))
	deep := filepath.Join(root, "src", "features", "orders")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	// Running from deep inside must resolve to the project root, or a second
	// tree gets installed wherever the developer happened to be standing.
	got, err := Env{Cwd: deep}.ProjectRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("ProjectRoot() = %q, want %q", got, root)
	}
}

func TestProjectRootRecognisesEachMarker(t *testing.T) {
	for _, marker := range []string{"deal-kit.lock", "package.json", ".git"} {
		t.Run(marker, func(t *testing.T) {
			root := t.TempDir()
			touch(t, filepath.Join(root, marker))
			got, err := Env{Cwd: root}.ProjectRoot()
			if err != nil || got != root {
				t.Errorf("ProjectRoot() = (%q, %v), want %q", got, err, root)
			}
		})
	}
}

func TestProjectRootRejectsTheKitItself(t *testing.T) {
	// The kit is a git repository too, so a .git check alone would accept it
	// and then fail with an unrelated message about the project type.
	kit := t.TempDir()
	touch(t, filepath.Join(kit, "kit.yaml"))
	touch(t, filepath.Join(kit, ".git", "HEAD"))

	for _, cwd := range []string{kit, filepath.Join(kit, "tool")} {
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := Env{Cwd: cwd}.ProjectRoot()
		if err == nil {
			t.Fatalf("from %q: expected an error, got none", cwd)
		}
		if !strings.Contains(err.Error(), "is the kit itself") {
			t.Errorf("from %q: error = %q, want it to name the mistake", cwd, err)
		}
	}
}

func TestProjectRootOutsideAnyProject(t *testing.T) {
	_, err := Env{Cwd: t.TempDir()}.ProjectRoot()
	if err == nil {
		t.Fatal("expected an error outside any project")
	}
	if !strings.Contains(err.Error(), "not inside a project") {
		t.Errorf("error = %q", err)
	}
}
