package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/engram"
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

// --- engram ---

// stubEngram replaces both entry points with recorded fakes, so no test here
// runs a real `claude` or touches the user's global Claude Code configuration.
func stubEngram(t *testing.T, st engram.Status, out engram.Outcome) *[]engram.Plan {
	t.Helper()
	var applied []engram.Plan
	prevDetect, prevApply, prevLook := engramDetect, engramApply, engramLook
	engramDetect = func(context.Context, engram.Runner, engram.Lookup) engram.Status { return st }
	engramApply = func(_ context.Context, _ engram.Runner, _ engram.Lookup, p engram.Plan, _ io.Writer) engram.Outcome {
		applied = append(applied, p)
		return out
	}
	engramLook = func(name string) (string, error) { return "/fake/bin/" + name, nil }
	t.Cleanup(func() { engramDetect, engramApply, engramLook = prevDetect, prevApply, prevLook })
	return &applied
}

func TestResolveEngramPairsTheStateWithItsPlan(t *testing.T) {
	stubEngram(t, engram.Status{State: engram.StateMarketplaceMissing}, engram.Outcome{})
	st, p := resolveEngram()
	if st.State != engram.StateMarketplaceMissing {
		t.Fatalf("State = %v", st.State)
	}
	if len(p.Steps()) != 3 {
		t.Errorf("plan = %v, want the three install commands", p.Lines())
	}
}

func TestInstallEngramPrintsTheCommandsAndReportsSuccess(t *testing.T) {
	plan := engram.PlanFor(engram.Status{State: engram.StateMarketplaceMissing})
	ready := engram.Status{State: engram.StateReady, EngramPath: "/bin/engram", Version: "0.1.1"}
	applied := stubEngram(t, engram.Status{}, engram.Outcome{Done: plan.Steps(), Status: ready})

	var out, errOut bytes.Buffer
	if err := installEngram(Env{Stdout: &out, Stderr: &errOut}, plan); err != nil {
		t.Fatalf("installEngram() = %v", err)
	}
	if len(*applied) != 1 {
		t.Fatalf("Apply was called %d times, want 1", len(*applied))
	}
	got := out.String()
	for _, want := range []string{
		"claude plugin marketplace add",
		"claude plugin install engram@engram --scope user --yes",
		"claude plugin enable engram@engram --scope user",
		"3 de 3 comando(s) ejecutados",
		"instalado y habilitado",
		"reiniciar Claude Code",
		"engram setup claude-code",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout does not mention %q:\n%s", want, got)
		}
	}
	if strings.Contains(errOut.String(), "falló") {
		t.Errorf("a successful install wrote a failure to stderr:\n%s", errOut.String())
	}
}

func TestInstallEngramReportsAPartialFailureOnStderr(t *testing.T) {
	plan := engram.PlanFor(engram.Status{State: engram.StateMarketplaceMissing})
	steps := plan.Steps()
	failed := steps[1]
	partial := engram.Outcome{
		Done:   steps[:1],
		Failed: &failed,
		Err:    errors.New("network unreachable"),
		// The marketplace add did land, and the user has to know.
		Status: engram.Status{State: engram.StatePluginMissing, EngramPath: "/bin/engram"},
	}
	stubEngram(t, engram.Status{}, partial)

	var out, errOut bytes.Buffer
	err := installEngram(Env{Stdout: &out, Stderr: &errOut}, plan)
	if err == nil {
		t.Fatal("installEngram() succeeded although a step failed")
	}
	if !strings.Contains(errOut.String(), "network unreachable") {
		t.Errorf("stderr does not carry the failure:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), "1 de 3 comando(s) ejecutados") {
		t.Errorf("stdout does not say how far it got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "marketplace registrado, plugin sin instalar") {
		t.Errorf("stdout does not report the state a retry starts from:\n%s", out.String())
	}
}

func TestInstallEngramWarnsWhenTheBinaryIsMissing(t *testing.T) {
	plan := engram.PlanFor(engram.Status{State: engram.StateMarketplaceMissing})
	stubEngram(t, engram.Status{}, engram.Outcome{
		Done:   plan.Steps(),
		Status: engram.Status{State: engram.StateReady}, // no EngramPath
	})
	var out, errOut bytes.Buffer
	if err := installEngram(Env{Stdout: &out, Stderr: &errOut}, plan); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "engram no está en el PATH") {
		t.Errorf("no warning about the missing binary:\n%s", errOut.String())
	}
}

func TestInstallEngramDoesNothingWithAnEmptyPlan(t *testing.T) {
	applied := stubEngram(t, engram.Status{State: engram.StateReady}, engram.Outcome{})
	var out, errOut bytes.Buffer
	if err := installEngram(Env{Stdout: &out, Stderr: &errOut}, engram.Plan{}); err != nil {
		t.Fatal(err)
	}
	if len(*applied) != 0 {
		t.Errorf("an empty plan reached Apply %d time(s)", len(*applied))
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("an empty plan printed something:\n%s\n%s", out.String(), errOut.String())
	}
}

func TestInstallEngramRefusesToDownloadWhileOffline(t *testing.T) {
	// The TUI already refuses this, but the CLI boundary gates it again for
	// the same reason --dry-run is checked twice: the flag is far from the
	// mutation, and one gate is one change away from being gone.
	plan := engram.PlanFor(engram.Status{State: engram.StateMarketplaceMissing})
	applied := stubEngram(t, engram.Status{}, engram.Outcome{Done: plan.Steps()})

	var out, errOut bytes.Buffer
	err := installEngram(Env{Stdout: &out, Stderr: &errOut, Offline: true}, plan)
	if err == nil {
		t.Fatal("installEngram() cloned the marketplace with --offline")
	}
	if !strings.Contains(err.Error(), "--offline") {
		t.Errorf("the error does not name the flag that stopped it: %v", err)
	}
	if len(*applied) != 0 {
		t.Errorf("Apply ran %d time(s) with --offline", len(*applied))
	}
}

func TestInstallEngramStillEnablesWhileOffline(t *testing.T) {
	// Enabling a plugin already on disk contacts nothing, so --offline must
	// not block it — the same rule the screen encodes.
	plan := engram.PlanFor(engram.Status{State: engram.StatePluginDisabled})
	ready := engram.Status{State: engram.StateReady, EngramPath: "/bin/engram"}
	applied := stubEngram(t, engram.Status{}, engram.Outcome{Done: plan.Steps(), Status: ready})

	var out, errOut bytes.Buffer
	if err := installEngram(Env{Stdout: &out, Stderr: &errOut, Offline: true}, plan); err != nil {
		t.Fatalf("installEngram() refused an enable-only plan with --offline: %v", err)
	}
	if len(*applied) != 1 {
		t.Errorf("Apply ran %d time(s), want 1", len(*applied))
	}
}

func TestInstallEngramDoesNotReportAnUnverifiableRunAsSuccess(t *testing.T) {
	// Every command ran, and the state could not be read back afterwards.
	// Exiting 0 here would tell the user the plugin is ready when nothing
	// checked that it is.
	plan := engram.PlanFor(engram.Status{State: engram.StateMarketplaceMissing})
	stubEngram(t, engram.Status{}, engram.Outcome{
		Done:   plan.Steps(),
		Status: engram.Status{State: engram.StateUnknown, EngramPath: "/bin/engram"},
	})

	var out, errOut bytes.Buffer
	err := installEngram(Env{Stdout: &out, Stderr: &errOut}, plan)
	if err == nil {
		t.Fatal("installEngram() returned success although the final state is unknown")
	}
	if !strings.Contains(errOut.String(), "no se pudo confirmar") {
		t.Errorf("nothing warned about it on stderr:\n%s", errOut.String())
	}
}

// interruptRunner raises a real SIGINT from inside the first mutating command,
// the way a user's Ctrl+C does, and then waits for the context to be
// cancelled. If deal-kit did not wire the signal, the default disposition
// kills this test binary — which is exactly the failure being pinned.
type interruptRunner struct{ streamed int }

func (r *interruptRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	switch strings.Join(args, " ") {
	case "plugin marketplace list --json":
		return []byte(`[{"name":"engram","repo":"Gentleman-Programming/engram"}]`), nil
	case "plugin list --json":
		return []byte(`[]`), nil
	}
	return nil, errors.New("unexpected query: " + strings.Join(args, " "))
}

func (r *interruptRunner) RunStream(ctx context.Context, _ io.Writer, _ string, _ ...string) error {
	r.streamed++
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	if err := p.Signal(os.Interrupt); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return errors.New("SIGINT never cancelled the install context")
	}
}

func TestAnInterruptedInstallReportsWhatLandedInsteadOfDying(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt cannot be sent to a process on Windows")
	}
	prevApply, prevRunner, prevLook := engramApply, engramRunner, engramLook
	r := &interruptRunner{}
	engramApply, engramRunner = engram.Apply, r
	engramLook = func(name string) (string, error) { return "/fake/bin/" + name, nil }
	t.Cleanup(func() { engramApply, engramRunner, engramLook = prevApply, prevRunner, prevLook })

	plan := engram.PlanFor(engram.Status{State: engram.StatePluginMissing})
	var out, errOut bytes.Buffer
	err := installEngram(Env{Stdout: &out, Stderr: &errOut}, plan)

	if err == nil {
		t.Fatal("an interrupted install reported success")
	}
	if r.streamed != 1 {
		t.Errorf("%d mutating command(s) ran; the interrupt must stop the plan", r.streamed)
	}
	// The point of surviving the signal: the machine gets read back and the
	// user is told where the install actually stopped.
	if !strings.Contains(out.String(), "marketplace registrado, plugin sin instalar") {
		t.Errorf("the partial state was never reported:\n%s", out.String())
	}
}
