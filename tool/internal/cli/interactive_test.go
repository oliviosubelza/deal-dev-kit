package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/tui"
)

// terminalStdout returns a writer tui.IsTerminal accepts, so the interactive
// branch can be exercised without a pty.
func terminalStdout(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no character device available: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	if !tui.IsTerminal(f) {
		t.Skip("os.DevNull is not reported as a terminal on this platform")
	}
	return f
}

// bootstrapKit writes a kit checkout whose manifest declares the given project
// types, so a test can prove the options are not a hardcoded list.
func bootstrapKit(t *testing.T, types ...string) string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("version: 1\nproject_types:\n")
	for _, ty := range types {
		b.WriteString("  " + ty + ": { match: \"deal-" + ty + "\", roots: { src: src } }\n")
	}
	b.WriteString("profiles:\n")
	for _, ty := range types {
		b.WriteString("  " + ty + ": [" + ty + "/x]\n")
	}
	b.WriteString("artifacts:\n")
	for _, ty := range types {
		b.WriteString("  - { id: " + ty + "/x, type: skill, applies_to: [" + ty + "], src: skills/" + ty + "/x }\n")
		write(t, filepath.Join(dir, "skills", ty, "x", "SKILL.md"), "---\nname: "+ty+"-x\ndescription: d\n---\n")
	}
	write(t, filepath.Join(dir, "kit.yaml"), b.String())
	return dir
}

// stubBootstrap replaces the screen with a recorded canned answer.
func stubBootstrap(t *testing.T, res tui.BootstrapResult) *tui.BootstrapConfig {
	t.Helper()
	var got tui.BootstrapConfig
	calls := 0
	prev := runBootstrap
	runBootstrap = func(cfg tui.BootstrapConfig) (tui.BootstrapResult, error) {
		got, calls = cfg, calls+1
		return res, nil
	}
	t.Cleanup(func() {
		runBootstrap = prev
		if calls > 1 {
			t.Errorf("the create screen opened %d times, want at most 1", calls)
		}
	})
	return &got
}

func TestInteractiveWithoutATerminalKeepsTheNotAProjectError(t *testing.T) {
	// A pipe or CI cannot answer a screen, so the message that names --here
	// stays exactly as it was.
	empty := t.TempDir()
	stubBootstrap(t, tui.BootstrapResult{Confirmed: true, ProjectType: "web"})

	var out, errOut bytes.Buffer
	err := Interactive(Env{Stdout: &out, Stderr: &errOut, Cwd: empty, KitDir: bootstrapKit(t, "web")}, "")
	if err == nil {
		t.Fatal("Interactive() succeeded without a terminal, want the not-a-project error")
	}
	for _, want := range []string{
		"no se está dentro de un proyecto",
		"no se encontró package.json, .git ni deal-kit.lock",
		empty,
		"--here",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Errorf("the non-terminal path wrote %d entries into the directory", len(entries))
	}
}

func TestInteractiveOnATerminalOpensTheCreateScreen(t *testing.T) {
	empty := t.TempDir()
	got := stubBootstrap(t, tui.BootstrapResult{})

	var errOut bytes.Buffer
	err := Interactive(Env{
		Stdout: terminalStdout(t), Stderr: &errOut, Cwd: empty,
		KitDir: bootstrapKit(t, "web"),
	}, "")
	if err != nil {
		t.Fatalf("Interactive() = %v, want the create screen instead of an error", err)
	}
	if got.Dir != empty {
		t.Errorf("the screen was given Dir %q, want the absolute current directory %q", got.Dir, empty)
	}
	if strings.Join(got.Markers, ",") != "package.json,.git,deal-kit.lock" {
		t.Errorf("Markers = %v, want the markers ProjectRoot looks for", got.Markers)
	}
}

func TestInteractiveCreateScreenOffersTheProjectTypesTheManifestDeclares(t *testing.T) {
	got := stubBootstrap(t, tui.BootstrapResult{})
	var errOut bytes.Buffer
	err := Interactive(Env{
		Stdout: terminalStdout(t), Stderr: &errOut, Cwd: t.TempDir(),
		KitDir: bootstrapKit(t, "api", "desktop"),
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Types, ",") != "api,desktop" {
		t.Errorf("Types = %v, want the manifest's own project types [api desktop]", got.Types)
	}
}

func TestInteractiveCancellingTheCreateScreenWritesNothing(t *testing.T) {
	empty := t.TempDir()
	stubBootstrap(t, tui.BootstrapResult{}) // cancelled: nothing confirmed

	var errOut bytes.Buffer
	err := Interactive(Env{
		Stdout: terminalStdout(t), Stderr: &errOut, Cwd: empty,
		KitDir: bootstrapKit(t, "web"),
	}, "")
	if err != nil {
		t.Fatalf("cancelling returned %v, want a clean exit", err)
	}
	entries, readErr := os.ReadDir(empty)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("cancelling created %v, want nothing", names)
	}
	if !strings.Contains(errOut.String(), "cd") {
		t.Errorf("cancelling did not tell the user to change directory:\n%s", errOut.String())
	}
}

func TestInteractivePassesTheTypeFlagToTheCreateScreen(t *testing.T) {
	// --type used to be ignored: ProjectRoot failed before it was read.
	got := stubBootstrap(t, tui.BootstrapResult{})
	var errOut bytes.Buffer
	if err := Interactive(Env{
		Stdout: terminalStdout(t), Stderr: &errOut, Cwd: t.TempDir(),
		KitDir: bootstrapKit(t, "backend", "web"),
	}, "backend"); err != nil {
		t.Fatal(err)
	}
	if got.Selected != "backend" {
		t.Errorf("Selected = %q, want backend", got.Selected)
	}
}

func TestNotAProjectIsDistinguishedFromBeingInsideTheKit(t *testing.T) {
	// Offering to scaffold a project inside the kit checkout would install the
	// kit into itself, which ProjectRoot already refuses by name.
	kitDir := t.TempDir()
	touch(t, filepath.Join(kitDir, "kit.yaml"))
	_, err := Env{Cwd: kitDir}.ProjectRoot()
	if err == nil {
		t.Fatal("ProjectRoot() accepted the kit itself")
	}
	if IsNotAProject(err) {
		t.Errorf("the kit-itself error is reported as not-a-project, so the create screen would open there:\n%v", err)
	}

	_, err = Env{Cwd: t.TempDir()}.ProjectRoot()
	if err == nil {
		t.Fatal("ProjectRoot() accepted an empty directory")
	}
	if !IsNotAProject(err) {
		t.Errorf("an empty directory is not reported as not-a-project:\n%v", err)
	}
}
