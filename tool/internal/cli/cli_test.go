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
		if !strings.Contains(err.Error(), "es el kit en sí") {
			t.Errorf("from %q: error = %q, want it to name the mistake", cwd, err)
		}
	}
}

func TestProjectRootOutsideAnyProject(t *testing.T) {
	_, err := Env{Cwd: t.TempDir()}.ProjectRoot()
	if err == nil {
		t.Fatal("expected an error outside any project")
	}
	if !strings.Contains(err.Error(), "no se está dentro de un proyecto") {
		t.Errorf("error = %q", err)
	}
}

func TestHereUsesTheCurrentDirectory(t *testing.T) {
	// A brand-new repository has no markers yet, and during the design stage
	// bootstrapping one is the common case rather than the exception.
	empty := t.TempDir()
	got, err := Env{Cwd: empty, Here: true}.ProjectRoot()
	if err != nil {
		t.Fatalf("--here must accept a directory with no markers: %v", err)
	}
	if got != empty {
		t.Errorf("ProjectRoot() = %q, want %q", got, empty)
	}
}

func TestHereStillRefusesTheKit(t *testing.T) {
	kit := t.TempDir()
	touch(t, filepath.Join(kit, "kit.yaml"))

	_, err := Env{Cwd: kit, Here: true}.ProjectRoot()
	if err == nil {
		t.Fatal("--here must not install the kit into itself")
	}
	if !strings.Contains(err.Error(), "es el kit en sí") {
		t.Errorf("error = %q", err)
	}
}

func TestHereDoesNotWalkUp(t *testing.T) {
	// Without --here the walk finds the parent project; with it, the current
	// directory wins, so a nested new package is not installed into its parent.
	parent := t.TempDir()
	touch(t, filepath.Join(parent, "package.json"))
	child := filepath.Join(parent, "packages", "new-app")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	if got, _ := (Env{Cwd: child}).ProjectRoot(); got != parent {
		t.Errorf("without --here: got %q, want the parent %q", got, parent)
	}
	if got, _ := (Env{Cwd: child, Here: true}).ProjectRoot(); got != child {
		t.Errorf("with --here: got %q, want %q", got, child)
	}
}

func TestProjectRootErrorNamesTheWayForward(t *testing.T) {
	_, err := Env{Cwd: t.TempDir()}.ProjectRoot()
	if err == nil {
		t.Fatal("expected an error")
	}
	// A message that says "create the project first" without saying how sends
	// the reader off to guess.
	// The browser accepts --here too, so naming only `init --here` reads as
	// though the directory is unusable.
	for _, want := range []string{"git init", "deal-kit --here"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%s", want, err)
		}
	}
}

func TestKitErrorDoesNotInventAPath(t *testing.T) {
	kit := t.TempDir()
	touch(t, filepath.Join(kit, "kit.yaml"))
	touch(t, filepath.Join(kit, ".git", "HEAD"))

	_, err := Env{Cwd: kit}.ProjectRoot()
	if err == nil {
		t.Fatal("expected an error")
	}
	// Naming a sibling directory that may not exist makes the user chase it.
	if strings.Contains(err.Error(), "cd ../") {
		t.Errorf("the message suggests a directory it cannot know exists:\n%s", err)
	}
}
