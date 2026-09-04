package engram

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRunner answers a canned response per command line and records what was
// asked. No test in this package may run a real `claude` or read ~/.claude:
// the whole point of Runner is that the install states are reachable on a
// machine that has neither.
type fakeRunner struct {
	replies map[string]reply
	calls   []string
	// mutated is bumped by any command that is not a query, so a test can
	// assert that nothing changed the machine.
	mutated int
}

type reply struct {
	out string
	err error
}

func newFake(replies map[string]reply) *fakeRunner {
	return &fakeRunner{replies: replies}
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Callers execute the path the lookup resolved, so key on the base name.
	line := commandLine(name, args)
	f.calls = append(f.calls, line)
	if !isQuery(line) {
		f.mutated++
	}
	r, ok := f.replies[line]
	if !ok {
		return nil, &CommandError{Args: append([]string{name}, args...),
			Stderr: "fake: unexpected command", Err: errors.New("exit status 1")}
	}
	return []byte(r.out), r.err
}

// RunStream is the mutating path. It answers from the same table as Run and
// copies what it would have printed into w, so a test can assert that the
// user's terminal receives the command's output instead of a buffer nobody
// reads.
func (f *fakeRunner) RunStream(ctx context.Context, w io.Writer, name string, args ...string) error {
	out, err := f.Run(ctx, name, args...)
	if w != nil && len(out) > 0 {
		w.Write(out)
	}
	return err
}

// commandLine is how the fakes name a call: the program's base name plus its
// arguments, so a resolved /fake/bin/claude keys the same as the constant.
func commandLine(name string, args []string) string {
	return strings.Join(append([]string{filepath.Base(name)}, args...), " ")
}

func isQuery(line string) bool {
	return line == strings.Join(MarketplaceListArgs(), " ") ||
		line == strings.Join(PluginListArgs(), " ")
}

// found resolves every lookup, so PATH plays no part in these tests.
func found(name string) (string, error) { return "/fake/bin/" + name, nil }

func onlyClaude(name string) (string, error) {
	if name == ClaudeBin {
		return "/fake/bin/claude", nil
	}
	return "", errors.New("not found")
}

func nothingFound(string) (string, error) { return "", errors.New("not found") }

const (
	goodMarketplace  = `[{"name":"engram","source":"github","repo":"Gentleman-Programming/engram","installLocation":"/home/u/.claude/plugins/marketplaces/engram"}]`
	otherMarketplace = `[{"name":"engram","source":"github","repo":"someone-else/engram","installLocation":"/tmp/x"}]`
	noMarketplaces   = `[]`
	enabledPlugin    = `[{"id":"engram@engram","version":"0.1.1","scope":"user","enabled":true,"installPath":"/p","installedAt":"x","lastUpdated":"y"}]`
	disabledPlugin   = `[{"id":"engram@engram","version":"0.1.1","scope":"user","enabled":false,"installPath":"/p"}]`
	projectPlugin    = `[{"id":"engram@engram","version":"0.1.1","scope":"project","enabled":true,"installPath":"/p"}]`
	noPlugins        = `[]`
)

func queries(markets, plugins string) map[string]reply {
	return map[string]reply{
		strings.Join(MarketplaceListArgs(), " "): {out: markets},
		strings.Join(PluginListArgs(), " "):      {out: plugins},
	}
}

// withMutations answers the three install commands successfully and flips the
// query answers once the marketplace has been added, so a whole install can be
// driven end to end.
type installFake struct {
	*fakeRunner
	markets, plugins string
	// corruptAfterEnable makes the plugin query stop parsing once the last
	// mutation succeeded, which is what a machine looks like when claude
	// prints something unexpected or the query budget is gone.
	corruptAfterEnable bool
}

func (f *installFake) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	line := commandLine(name, args)
	switch line {
	case strings.Join(MarketplaceListArgs(), " "):
		f.calls = append(f.calls, line)
		return []byte(f.markets), nil
	case strings.Join(PluginListArgs(), " "):
		f.calls = append(f.calls, line)
		return []byte(f.plugins), nil
	}
	out, err := f.fakeRunner.Run(ctx, name, args...)
	if err == nil {
		switch line {
		case strings.Join(MarketplaceAddArgs(), " "):
			f.markets = goodMarketplace
		case strings.Join(PluginInstallArgs(), " "):
			f.plugins = disabledPlugin
		case strings.Join(PluginEnableArgs(), " "):
			f.plugins = enabledPlugin
			if f.corruptAfterEnable {
				f.plugins = `{"broken":`
			}
		}
	}
	return out, err
}

func (f *installFake) RunStream(ctx context.Context, w io.Writer, name string, args ...string) error {
	out, err := f.Run(ctx, name, args...)
	if w != nil && len(out) > 0 {
		w.Write(out)
	}
	return err
}

func TestDetectReportsEveryState(t *testing.T) {
	cases := []struct {
		name     string
		markets  string
		plugins  string
		look     Lookup
		want     State
		wantRepo string
	}{
		{"ready", goodMarketplace, enabledPlugin, found, StateReady, MarketplaceRepo},
		{"disabled", goodMarketplace, disabledPlugin, found, StatePluginDisabled, MarketplaceRepo},
		{"plugin missing", goodMarketplace, noPlugins, found, StatePluginMissing, MarketplaceRepo},
		{"marketplace missing", noMarketplaces, noPlugins, found, StateMarketplaceMissing, ""},
		{"conflict", otherMarketplace, enabledPlugin, found, StateMarketplaceConflict, "someone-else/engram"},
		// A copy installed at project scope is a different decision from the
		// global one, so it must not make the user-scope install look done.
		{"other scope", goodMarketplace, projectPlugin, found, StatePluginMissing, MarketplaceRepo},
		{"claude missing", goodMarketplace, enabledPlugin, nothingFound, StateClaudeMissing, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newFake(queries(tc.markets, tc.plugins))
			st := Detect(context.Background(), r, tc.look)
			if st.State != tc.want {
				t.Errorf("State = %v, want %v (err %v)", st.State, tc.want, st.Err)
			}
			if st.FoundRepo != tc.wantRepo {
				t.Errorf("FoundRepo = %q, want %q", st.FoundRepo, tc.wantRepo)
			}
			if r.mutated != 0 {
				t.Errorf("Detect ran %d mutating command(s): %v", r.mutated, r.calls)
			}
		})
	}
}

func TestDetectReportsTheEngramBinarySeparately(t *testing.T) {
	// The plugin installs and enables without the binary, and then every hook
	// fails at runtime, so this must not be folded into State.
	r := newFake(queries(goodMarketplace, enabledPlugin))
	st := Detect(context.Background(), r, onlyClaude)
	if st.State != StateReady {
		t.Fatalf("State = %v, want StateReady", st.State)
	}
	if st.EngramBinaryFound() {
		t.Error("EngramBinaryFound() is true with no engram on PATH")
	}
	if st.ClaudePath == "" {
		t.Error("ClaudePath is empty although claude resolved")
	}
}

func TestMalformedJSONIsRejectedRatherThanReadAsNotInstalled(t *testing.T) {
	// Treating unreadable output as "nothing installed" would make deal-kit
	// re-add a marketplace that is already there.
	for _, bad := range []string{
		"",
		"not json",
		`{"marketplaces": []}`, // the --available shape, an object
		`[{"name":`,
	} {
		r := newFake(map[string]reply{
			strings.Join(MarketplaceListArgs(), " "): {out: bad},
			strings.Join(PluginListArgs(), " "):      {out: enabledPlugin},
		})
		st := Detect(context.Background(), r, found)
		if st.State != StateUnknown {
			t.Errorf("%q: State = %v, want StateUnknown", bad, st.State)
		}
		if st.Err == nil {
			t.Errorf("%q: StateUnknown with no error to show the user", bad)
		}
		if PlanFor(st).Empty() != true {
			t.Errorf("%q: an unreadable state produced a plan", bad)
		}
	}
}

func TestMalformedPluginJSONIsRejected(t *testing.T) {
	r := newFake(map[string]reply{
		strings.Join(MarketplaceListArgs(), " "): {out: goodMarketplace},
		strings.Join(PluginListArgs(), " "):      {out: `{"plugins":[]}`},
	})
	st := Detect(context.Background(), r, found)
	if st.State != StateUnknown || st.Err == nil {
		t.Fatalf("State = %v, err = %v; want StateUnknown with an error", st.State, st.Err)
	}
}

func TestAFailedQueryIsUnknownNotMissing(t *testing.T) {
	r := newFake(map[string]reply{
		strings.Join(MarketplaceListArgs(), " "): {err: errors.New("exit status 1")},
	})
	st := Detect(context.Background(), r, found)
	if st.State != StateUnknown {
		t.Errorf("State = %v, want StateUnknown", st.State)
	}
}

func TestMarketplaceIdentityIsComparedByRepo(t *testing.T) {
	// The JSON carries no URL and no ref, so repo is the only identity there
	// is. It must match the way a host compares one: case and .git aside.
	for _, repo := range []string{
		"Gentleman-Programming/engram",
		"gentleman-programming/ENGRAM",
		"Gentleman-Programming/engram.git",
		"/Gentleman-Programming/engram/",
	} {
		if !sameRepo(repo, MarketplaceRepo) {
			t.Errorf("%q was not accepted as the legitimate marketplace", repo)
		}
	}
	for _, repo := range []string{
		"Gentleman-Programming/engram-fork",
		"attacker/engram",
		"",
	} {
		if sameRepo(repo, MarketplaceRepo) {
			t.Errorf("%q was accepted as the legitimate marketplace", repo)
		}
	}
}

func TestAConflictingMarketplaceIsNeverMutated(t *testing.T) {
	r := newFake(queries(otherMarketplace, noPlugins))
	st := Detect(context.Background(), r, found)
	p := PlanFor(st)
	if !p.Empty() {
		t.Fatalf("a conflicting marketplace produced a plan: %v", p.Lines())
	}
	out := Apply(context.Background(), r, found, p, nil)
	if r.mutated != 0 {
		t.Errorf("the conflict path ran %d mutating command(s): %v", r.mutated, r.calls)
	}
	if !out.Applied() {
		t.Errorf("an empty plan reported a failure: %v", out.Err)
	}
}

func TestPlanForEachState(t *testing.T) {
	cases := []struct {
		state State
		want  []string
	}{
		{StateMarketplaceMissing, []string{
			strings.Join(MarketplaceAddArgs(), " "),
			strings.Join(PluginInstallArgs(), " "),
			strings.Join(PluginEnableArgs(), " "),
		}},
		{StatePluginMissing, []string{
			strings.Join(PluginInstallArgs(), " "),
			strings.Join(PluginEnableArgs(), " "),
		}},
		{StatePluginDisabled, []string{strings.Join(PluginEnableArgs(), " ")}},
		{StateReady, nil},
		{StateClaudeMissing, nil},
		{StateMarketplaceConflict, nil},
		{StateUnknown, nil},
	}
	for _, tc := range cases {
		got := PlanFor(Status{State: tc.state}).Lines()
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("state %v: plan = %v, want %v", tc.state, got, tc.want)
		}
	}
}

func TestTheCommandsAreExactlyTheDocumentedSurface(t *testing.T) {
	// These are constants, not configuration. Pinning them here is what stops
	// a refactor from quietly changing the scope or dropping --yes.
	want := map[string]string{
		"marketplace add": ClaudeBin + " plugin marketplace add " +
			MarketplaceURL + "#" + MarketplaceTag + " --scope user",
		"install": ClaudeBin + " plugin install engram@engram --scope user --yes",
		"enable":  ClaudeBin + " plugin enable engram@engram --scope user",
		"markets": ClaudeBin + " plugin marketplace list --json",
		"plugins": ClaudeBin + " plugin list --json",
	}
	got := map[string]string{
		"marketplace add": strings.Join(MarketplaceAddArgs(), " "),
		"install":         strings.Join(PluginInstallArgs(), " "),
		"enable":          strings.Join(PluginEnableArgs(), " "),
		"markets":         strings.Join(MarketplaceListArgs(), " "),
		"plugins":         strings.Join(PluginListArgs(), " "),
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s:\n got %q\nwant %q", k, got[k], w)
		}
	}
	// `#ref` pinning works on marketplace add only; install takes no ref.
	if strings.Contains(got["install"], "#") {
		t.Error("plugin install carries a ref, which it does not accept")
	}
	if strings.Contains(got["enable"], "--yes") {
		t.Error("plugin enable carries --yes, which it does not have")
	}
	// --available changes the response shape from an array to an object.
	if strings.Contains(got["plugins"], "--available") {
		t.Error("plugin list uses --available, whose response is not an array")
	}
}

func TestPlanIsImmutable(t *testing.T) {
	p := PlanFor(Status{State: StateMarketplaceMissing})
	steps := p.Steps()
	steps[0].Args[0] = "rm"
	steps = append(steps, Step{Kind: StepEnable, Args: []string{"rm", "-rf", "/"}})
	_ = steps
	if got := p.Lines()[0]; !strings.HasPrefix(got, ClaudeBin+" ") {
		t.Errorf("mutating the returned steps changed the plan: %q", got)
	}
	if len(p.Steps()) != 3 {
		t.Errorf("appending to the returned steps changed the plan: %d steps", len(p.Steps()))
	}
}

func TestApplyInstallsFromScratchAndReachesReady(t *testing.T) {
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(MarketplaceAddArgs(), " "): {},
			strings.Join(PluginInstallArgs(), " "):  {},
			strings.Join(PluginEnableArgs(), " "):   {},
		}),
		markets: noMarketplaces, plugins: noPlugins,
	}
	st := Detect(context.Background(), f, found)
	out := Apply(context.Background(), f, found, PlanFor(st), nil)
	if !out.Applied() {
		t.Fatalf("Apply failed: %v (at %v)", out.Err, out.Failed)
	}
	if len(out.Done) != 3 {
		t.Errorf("ran %d steps, want 3", len(out.Done))
	}
	if out.Status.State != StateReady {
		t.Errorf("final state = %v, want StateReady", out.Status.State)
	}
	if out.Status.Version != "0.1.1" {
		t.Errorf("Version = %q, want the version the re-query reported", out.Status.Version)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	f := &installFake{
		fakeRunner: newFake(map[string]reply{}),
		markets:    goodMarketplace, plugins: enabledPlugin,
	}
	st := Detect(context.Background(), f, found)
	p := PlanFor(st)
	if !p.Empty() {
		t.Fatalf("an already-ready machine produced a plan: %v", p.Lines())
	}
	out := Apply(context.Background(), f, found, p, nil)
	if f.mutated != 0 {
		t.Errorf("a second run mutated %d time(s): %v", f.mutated, f.calls)
	}
	if out.Status.State != StateReady {
		t.Errorf("state = %v, want StateReady", out.Status.State)
	}
}

func TestPartialFailureStopsAndReportsWhatIsOnTheMachine(t *testing.T) {
	// marketplace add succeeds, install fails. The user must be told the
	// marketplace is now registered, or a retry looks like a no-op.
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(MarketplaceAddArgs(), " "): {},
			strings.Join(PluginInstallArgs(), " "): {err: &CommandError{
				Args: PluginInstallArgs(), Stderr: "network unreachable",
				Err: errors.New("exit status 1")}},
		}),
		markets: noMarketplaces, plugins: noPlugins,
	}
	st := Detect(context.Background(), f, found)
	out := Apply(context.Background(), f, found, PlanFor(st), nil)

	if out.Applied() {
		t.Fatal("Apply reported success although install failed")
	}
	if len(out.Done) != 1 {
		t.Errorf("Done = %v, want only the marketplace add", out.Done)
	}
	if out.Failed == nil || out.Failed.Kind != StepInstall {
		t.Errorf("Failed = %v, want the install step", out.Failed)
	}
	// enable must not have run after install failed.
	for _, c := range f.calls {
		if c == strings.Join(PluginEnableArgs(), " ") {
			t.Error("enable ran after install failed")
		}
	}
	if out.Status.State != StatePluginMissing {
		t.Errorf("re-queried state = %v, want StatePluginMissing (the marketplace is registered now)",
			out.Status.State)
	}
	if !strings.Contains(out.Err.Error(), "network unreachable") {
		t.Errorf("the error drops what the command wrote to stderr: %v", out.Err)
	}
}

func TestRecoveryAfterAPartialFailureFinishesTheJob(t *testing.T) {
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(PluginInstallArgs(), " "): {},
			strings.Join(PluginEnableArgs(), " "):  {},
		}),
		// where the partial failure above left the machine
		markets: goodMarketplace, plugins: noPlugins,
	}
	st := Detect(context.Background(), f, found)
	if st.State != StatePluginMissing {
		t.Fatalf("State = %v, want StatePluginMissing", st.State)
	}
	p := PlanFor(st)
	if len(p.Steps()) != 2 {
		t.Fatalf("the retry re-adds the marketplace: %v", p.Lines())
	}
	out := Apply(context.Background(), f, found, p, nil)
	if !out.Applied() || out.Status.State != StateReady {
		t.Fatalf("retry did not finish: applied=%v state=%v err=%v",
			out.Applied(), out.Status.State, out.Err)
	}
}

func TestACancelledContextRunsNothing(t *testing.T) {
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(MarketplaceAddArgs(), " "): {},
			strings.Join(PluginInstallArgs(), " "):  {},
			strings.Join(PluginEnableArgs(), " "):   {},
		}),
		markets: noMarketplaces, plugins: noPlugins,
	}
	p := PlanFor(Status{State: StateMarketplaceMissing})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := Apply(ctx, f, found, p, nil)

	if out.Applied() {
		t.Fatal("Apply succeeded with a cancelled context")
	}
	if !errors.Is(out.Err, context.Canceled) {
		t.Errorf("Err = %v, want context.Canceled", out.Err)
	}
	if f.mutated != 0 {
		t.Errorf("a cancelled run mutated %d time(s): %v", f.mutated, f.calls)
	}
	// The re-query must still work: it runs on a context detached from the
	// cancelled one, or the report would say "unknown" for a readable machine.
	if out.Status.State != StateMarketplaceMissing {
		t.Errorf("state after cancellation = %v, want StateMarketplaceMissing", out.Status.State)
	}
}

// --- end to end, against simulated executables ---

// fakeClaude writes a claude executable into a temporary directory and returns
// a Lookup that finds it there. HOME is redirected as well so nothing in this
// test can reach the developer's real ~/.claude.
func fakeClaude(t *testing.T, script string) (Lookup, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the simulated executable is a shell script")
	}
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path := filepath.Join(dir, ClaudeBin)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	look := func(name string) (string, error) {
		if name == ClaudeBin {
			return path, nil
		}
		return "", errors.New("not found")
	}
	return look, home
}

func TestEndToEndAgainstASimulatedClaude(t *testing.T) {
	// The script records every invocation and answers the two queries from a
	// state file it also updates, so the real ExecRunner — argv, exit codes,
	// stdout/stderr separation — is what is under test.
	look, home := fakeClaude(t, `#!/bin/sh
state="$HOME/state"
[ -f "$state" ] || echo missing > "$state"
echo "$@" >> "$HOME/calls"
case "$1 $2" in
  "plugin marketplace")
    case "$3" in
      list)
        if [ "$(cat "$state")" = missing ]; then echo '[]'; else
          echo '[{"name":"engram","source":"github","repo":"Gentleman-Programming/engram","installLocation":"/x"}]'
        fi ;;
      add) echo added > "$state" ;;
    esac ;;
  "plugin list")
    case "$(cat "$state")" in
      enabled) echo '[{"id":"engram@engram","version":"0.1.1","scope":"user","enabled":true}]' ;;
      installed) echo '[{"id":"engram@engram","version":"0.1.1","scope":"user","enabled":false}]' ;;
      *) echo '[]' ;;
    esac ;;
  "plugin install") echo installed > "$state" ;;
  "plugin enable") echo enabled > "$state" ;;
  *) echo "unexpected: $@" >&2; exit 1 ;;
esac
`)

	r := ExecRunner{}
	st := Detect(context.Background(), r, look)
	if st.State != StateMarketplaceMissing {
		t.Fatalf("State = %v, want StateMarketplaceMissing (err %v)", st.State, st.Err)
	}

	out := Apply(context.Background(), r, look, PlanFor(st), nil)
	if !out.Applied() {
		t.Fatalf("Apply failed: %v", out.Err)
	}
	if out.Status.State != StateReady {
		t.Fatalf("final state = %v, want StateReady", out.Status.State)
	}

	calls, err := os.ReadFile(filepath.Join(home, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"plugin marketplace add " + MarketplaceURL + "#" + MarketplaceTag + " --scope user",
		"plugin install engram@engram --scope user --yes",
		"plugin enable engram@engram --scope user",
	} {
		if !strings.Contains(string(calls), want) {
			t.Errorf("claude was never called with %q\n%s", want, calls)
		}
	}
}

func TestExecRunnerKeepsStderrOutOfTheParsedOutput(t *testing.T) {
	// A warning on stderr mixed into stdout would break every JSON parse.
	look, _ := fakeClaude(t, `#!/bin/sh
echo "warning: something" >&2
echo '[]'
`)
	path, _ := look(ClaudeBin)
	out, err := ExecRunner{}.Run(context.Background(), path, "plugin", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "[]" {
		t.Errorf("stdout = %q, want only the JSON", out)
	}
}

func TestExecRunnerReportsStderrOnFailure(t *testing.T) {
	look, _ := fakeClaude(t, `#!/bin/sh
echo "marketplace already exists" >&2
exit 1
`)
	path, _ := look(ClaudeBin)
	_, err := ExecRunner{}.Run(context.Background(), path, "plugin", "install")
	if err == nil {
		t.Fatal("a failing command returned no error")
	}
	if !strings.Contains(err.Error(), "marketplace already exists") {
		t.Errorf("the error drops stderr: %v", err)
	}
	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Errorf("err is %T, want *CommandError", err)
	}
}

func TestSanitizedEnvDropsLeakedGitVariables(t *testing.T) {
	// deal-kit normally runs inside a project checkout, and these would send
	// the marketplace clone into that repository.
	t.Setenv("GIT_DIR", "/some/project/.git")
	t.Setenv("GIT_WORK_TREE", "/some/project")
	t.Setenv("DEAL_KIT_KEEP_ME", "1")

	env := sanitizedEnv()
	joined := strings.Join(env, "\n")
	for _, banned := range []string{"GIT_DIR=", "GIT_WORK_TREE="} {
		if strings.Contains(joined, banned) {
			t.Errorf("%s survived sanitizedEnv", banned)
		}
	}
	if !strings.Contains(joined, "DEAL_KIT_KEEP_ME=1") {
		t.Error("sanitizedEnv dropped an unrelated variable")
	}
	if !strings.Contains(joined, "GIT_TERMINAL_PROMPT=0") {
		t.Error("sanitizedEnv does not disable the credential prompt")
	}
}

// --- streaming and verification ---

func TestMutatingStepsStreamTheirOutputToTheCaller(t *testing.T) {
	// A clone that prints nothing for minutes is indistinguishable from a
	// hang, so what the mutating commands write has to reach the terminal.
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(MarketplaceAddArgs(), " "): {out: "Cloning marketplace...\n"},
			strings.Join(PluginInstallArgs(), " "):  {out: "Installed engram\n"},
			strings.Join(PluginEnableArgs(), " "):   {out: "Enabled engram\n"},
		}),
		markets: noMarketplaces, plugins: noPlugins,
	}
	var live bytes.Buffer
	out := Apply(context.Background(), f, found, PlanFor(Status{State: StateMarketplaceMissing}), &live)
	if !out.Verified() {
		t.Fatalf("install did not finish: err=%v state=%v", out.Err, out.Status.State)
	}
	for _, want := range []string{"Cloning marketplace...", "Installed engram", "Enabled engram"} {
		if !strings.Contains(live.String(), want) {
			t.Errorf("the live writer never received %q:\n%s", want, live.String())
		}
	}
}

func TestQueryOutputNeverReachesTheLiveWriter(t *testing.T) {
	// The queries are JSON meant for the parser, not progress for the user.
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(PluginEnableArgs(), " "): {},
		}),
		markets: goodMarketplace, plugins: disabledPlugin,
	}
	var live bytes.Buffer
	Apply(context.Background(), f, found, PlanFor(Status{State: StatePluginDisabled}), &live)
	if strings.Contains(live.String(), "engram@engram") {
		t.Errorf("a query's JSON was printed to the user:\n%s", live.String())
	}
}

func TestSucceedingCommandsWithAnUnreadableRecheckAreNotVerified(t *testing.T) {
	// Every mutation succeeds and the re-query that would confirm the result
	// fails. Reporting that as success is the guess this package refuses to
	// make: the machine's actual state is not known.
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(MarketplaceAddArgs(), " "): {},
			strings.Join(PluginInstallArgs(), " "):  {},
			strings.Join(PluginEnableArgs(), " "):   {},
		}),
		markets: noMarketplaces, plugins: noPlugins,
		corruptAfterEnable: true,
	}
	out := Apply(context.Background(), f, found, PlanFor(Status{State: StateMarketplaceMissing}), nil)

	if !out.Applied() {
		t.Fatalf("Applied() = false although every command succeeded: %v", out.Err)
	}
	if len(out.Done) != 3 {
		t.Errorf("Done = %d step(s), want 3", len(out.Done))
	}
	if out.Verified() {
		t.Error("Verified() = true although the final state could not be read")
	}
	if out.Status.State != StateUnknown {
		t.Errorf("Status.State = %v, want StateUnknown", out.Status.State)
	}
}

func TestAFinishedInstallIsVerified(t *testing.T) {
	// The counterpart: a genuinely successful install must not start
	// reporting failure.
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(MarketplaceAddArgs(), " "): {},
			strings.Join(PluginInstallArgs(), " "):  {},
			strings.Join(PluginEnableArgs(), " "):   {},
		}),
		markets: noMarketplaces, plugins: noPlugins,
	}
	out := Apply(context.Background(), f, found, PlanFor(Status{State: StateMarketplaceMissing}), nil)
	if !out.Verified() {
		t.Fatalf("Verified() = false for a finished install: err=%v state=%v", out.Err, out.Status.State)
	}
}

func TestTheFinalRecheckDoesNotInheritAnExhaustedBudget(t *testing.T) {
	// A slow clone can consume the whole install budget. The re-query that
	// confirms the result must still run, or a good install reports unknown.
	f := &installFake{
		fakeRunner: newFake(map[string]reply{
			strings.Join(PluginEnableArgs(), " "): {},
		}),
		markets: goodMarketplace, plugins: disabledPlugin,
	}
	// A context that is cancelled by the step itself is the closest hermetic
	// stand-in for a budget that ran out during the mutation.
	ctx, cancel := context.WithCancel(context.Background())
	f.replies[strings.Join(PluginEnableArgs(), " ")] = reply{}
	spent := &cancelAfterMutation{installFake: f, cancel: cancel}

	out := Apply(ctx, spent, found, PlanFor(Status{State: StatePluginDisabled}), nil)
	if !out.Verified() {
		t.Fatalf("a finished install reported %v (err %v) after its budget ran out",
			out.Status.State, out.Err)
	}
}

// cancelAfterMutation burns the context the moment the mutation completes.
type cancelAfterMutation struct {
	*installFake
	cancel context.CancelFunc
}

// Run refuses a spent context the way a real command does; installFake's own
// query shortcut does not, and without this the test would pass on a context
// nobody was honouring.
func (c *cancelAfterMutation) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.installFake.Run(ctx, name, args...)
}

func (c *cancelAfterMutation) RunStream(ctx context.Context, w io.Writer, name string, args ...string) error {
	err := c.installFake.RunStream(ctx, w, name, args...)
	c.cancel()
	return err
}

func TestExecRunnerStreamsBeforeTheCommandExits(t *testing.T) {
	// Proves the streaming is real and not a buffer flushed at exit: the
	// script does not finish until the test has seen its first line.
	look, home := fakeClaude(t, `#!/bin/sh
echo first
while [ ! -f "$HOME/go" ]; do sleep 0.05; done
echo second
`)
	seen := make(chan struct{})
	w := &signalOnWrite{fire: seen}

	done := make(chan error, 1)
	path, _ := look(ClaudeBin)
	go func() { done <- ExecRunner{}.RunStream(context.Background(), w, path, "plugin", "install") }()

	select {
	case <-seen:
	case <-time.After(10 * time.Second):
		t.Fatal("nothing was written before the command exited: the output is buffered")
	}
	if err := os.WriteFile(filepath.Join(home, "go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := w.String(); !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Errorf("stream = %q, want both lines", got)
	}
}

// signalOnWrite closes fire on the first write, so a test can prove output
// arrived while the process was still running.
type signalOnWrite struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	fire chan struct{}
	once sync.Once
}

func (s *signalOnWrite) Write(p []byte) (int, error) {
	s.mu.Lock()
	n, err := s.buf.Write(p)
	s.mu.Unlock()
	s.once.Do(func() { close(s.fire) })
	return n, err
}

func (s *signalOnWrite) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
