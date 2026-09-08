package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/doctor"
)

// newKit builds a minimal kit checkout New can scaffold against: one project
// type ("web"), a profile with a single artifact so a genuine run has
// something real for the trailing Init to install.
func newKit(t *testing.T) (kitDir string) {
	t.Helper()
	kitDir = t.TempDir()
	write(t, filepath.Join(kitDir, "kit.yaml"), testManifest)
	write(t, filepath.Join(kitDir, "skills", "web", "ui", "SKILL.md"), skillContent)
	return kitDir
}

// passingDoctor stands in for a machine with every tool installed, without
// touching the real PATH — doctor.Check runs exec.LookPath against whatever
// PATH the test happens to run under, which makes New's own suite depend on
// what is actually installed on the machine running it. Swapped in the same
// way the engram entry points in interactive.go are.
func passingDoctor(t *testing.T) {
	t.Helper()
	prev := doctorCheck
	doctorCheck = func(tools []doctor.Tool) doctor.Report {
		rep := doctor.Report{}
		for _, tool := range tools {
			r := doctor.Result{Tool: tool}
			if tool.Name == "git" || tool.Name == "node" || tool.Name == "pnpm" {
				r.Found, r.Version = true, "1.0.0"
			}
			rep.Results = append(rep.Results, r)
		}
		return rep
	}
	t.Cleanup(func() { doctorCheck = prev })
}

// fakeGeneratorRunner records whether it ran, and lets a test dictate
// success or failure without ever invoking a real generator.
//
// dir, as New calls it, is the parent directory the generator is run from —
// same as the real cmd.Dir, since a generator like create-vite takes the new
// project's directory name as its own argument and creates that subdirectory
// itself. createSubdir names it, so a success scenario can simulate a real
// generator's effect without hardcoding one test's directory name into the
// shared fixture.
type fakeGeneratorRunner struct {
	called       bool
	dir          string
	name         string
	args         []string
	failErr      error
	createSubdir string
}

func (f *fakeGeneratorRunner) Run(dir string, _, _ io.Writer, _ io.Reader, name string, args ...string) error {
	f.called, f.dir, f.name, f.args = true, dir, name, args
	if f.failErr != nil {
		return f.failErr
	}
	if f.createSubdir == "" {
		return nil
	}
	// A real generator would have created the target directory and its own
	// package.json; New's trailing Init needs a project marker there to
	// treat the new directory as the project root instead of refusing it.
	target := filepath.Join(dir, f.createSubdir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target, "package.json"), []byte("{}\n"), 0o644)
}

func withFakeRunner(t *testing.T) *fakeGeneratorRunner {
	t.Helper()
	f := &fakeGeneratorRunner{}
	prev := newRunner
	newRunner = f
	t.Cleanup(func() { newRunner = prev })
	return f
}

func TestNewAbortsWithoutTouchingAnythingWhenDeclined(t *testing.T) {
	passingDoctor(t)
	runner := withFakeRunner(t)
	kitDir := newKit(t)
	cwd := t.TempDir()

	e := Env{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader("n\n"),
		Cwd: cwd, KitDir: kitDir,
	}
	err := New(e, "crm-deal-web", "web")
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("New() error = %v, want ErrAborted", err)
	}
	if runner.called {
		t.Error("the generator ran despite the user declining")
	}
	if _, statErr := os.Stat(filepath.Join(cwd, "crm-deal-web")); statErr == nil {
		t.Error("a directory was created despite the user declining")
	}
}

func TestNewStopsBeforeInitWhenTheGeneratorFails(t *testing.T) {
	passingDoctor(t)
	runner := withFakeRunner(t)
	runner.failErr = errors.New("boom: create-vite exited 1")
	kitDir := newKit(t)
	cwd := t.TempDir()

	e := Env{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader(""),
		Cwd: cwd, KitDir: kitDir, AssumeYes: true,
	}
	err := New(e, "crm-deal-web", "web")
	if err == nil {
		t.Fatal("expected an error when the generator fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("the generator's own failure was swallowed: %q", err)
	}
	if !runner.called {
		t.Fatal("the generator was never invoked")
	}
	// Init would have written .claude/skills/web-ui — its absence proves New
	// did not proceed past the failed generator.
	if _, statErr := os.Stat(filepath.Join(cwd, "crm-deal-web", ".claude")); statErr == nil {
		t.Error("New proceeded to install the kit after the generator failed")
	}
}

func TestNewScaffoldsAndInstallsOnSuccess(t *testing.T) {
	passingDoctor(t)
	runner := withFakeRunner(t)
	runner.createSubdir = "crm-deal-web"
	kitDir := newKit(t)
	cwd := t.TempDir()

	e := Env{
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader(""),
		Cwd: cwd, KitDir: kitDir, AssumeYes: true, NoDeps: true, Version: "test",
	}
	if err := New(e, "crm-deal-web", "web"); err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !runner.called {
		t.Fatal("the generator was never invoked")
	}
	if runner.name != "pnpm" && runner.name != "npm" {
		t.Errorf("generator command = %q, want the detected package manager", runner.name)
	}
	// The trailing Init should have installed web/ui into the new directory.
	installed := filepath.Join(cwd, "crm-deal-web", ".claude", "skills", "web-ui", "SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Errorf("Init never ran against the scaffolded directory: %v", err)
	}
}

func TestNewRefusesANonEmptyTargetDirectory(t *testing.T) {
	passingDoctor(t)
	runner := withFakeRunner(t)
	kitDir := newKit(t)
	cwd := t.TempDir()
	write(t, filepath.Join(cwd, "crm-deal-web", "existing-file"), "x")

	e := Env{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader(""), Cwd: cwd, KitDir: kitDir}
	err := New(e, "crm-deal-web", "web")
	if err == nil {
		t.Fatal("expected an error for a non-empty target directory")
	}
	if !strings.Contains(err.Error(), "ya existe") {
		t.Errorf("error = %q, want it to name the conflict", err)
	}
	if runner.called {
		t.Error("the generator ran against a non-empty directory")
	}
}

func TestNewRejectsAnUnknownProjectType(t *testing.T) {
	passingDoctor(t)
	kitDir := newKit(t)
	cwd := t.TempDir()

	e := Env{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Stdin: strings.NewReader(""), Cwd: cwd, KitDir: kitDir}
	err := New(e, "crm-deal-web", "desktop")
	if err == nil {
		t.Fatal("expected an error for an unknown project type")
	}
	if !strings.Contains(err.Error(), "desktop") {
		t.Errorf("error = %q, want it to name the unknown type", err)
	}
}
