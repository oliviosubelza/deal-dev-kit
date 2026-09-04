// Package engram installs the Engram plugin into Claude Code at user-global
// scope by driving the `claude` CLI.
//
// It never edits Claude Code's own configuration files: `claude` owns that
// format, and a second writer would drift from it the first time the format
// changes. Everything this package does goes through documented subcommands
// whose arguments are constants below — never a shell, never a string built
// from kit.yaml, a flag or the environment. An installer that takes its
// arguments from data can be pointed at another repository by editing data.
package engram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/execenv"
)

// The fixed command surface. These are the only values the commands are built
// from.
const (
	// ClaudeBin is the Claude Code CLI, which owns the plugin installation.
	ClaudeBin = "claude"
	// EngramBin is the Engram binary itself. The plugin's hooks call it, so a
	// working install needs it on PATH, but deal-kit does not install it.
	EngramBin = "engram"

	// MarketplaceName is how the marketplace registers itself.
	MarketplaceName = "engram"
	// MarketplaceRepo identifies the legitimate marketplace. It is the only
	// identity check available: the JSON carries no full URL and no ref.
	MarketplaceRepo = "Gentleman-Programming/engram"
	// MarketplaceURL is what gets cloned.
	MarketplaceURL = "https://github.com/Gentleman-Programming/engram.git"
	// MarketplaceTag pins what is cloned. The `#ref` suffix is honoured by
	// `marketplace add` only; `plugin install` takes no ref at all.
	MarketplaceTag = "v1.20.0"

	// PluginID is <plugin>@<marketplace>, which is how `claude` names it.
	PluginID = "engram@engram"
	// Scope is user-global. Valid scopes are user, project and local; deal-kit
	// only ever installs at user scope, so a project checkout never carries a
	// plugin decision for whoever clones it.
	Scope = "user"
)

// Timeouts. Queries are local reads and must never hang a session; the
// mutating steps clone a repository, so they get a real budget.
const (
	QueryTimeout   = 20 * time.Second
	InstallTimeout = 5 * time.Minute
)

// Runner runs one external command. It exists so tests can drive every state
// without a real `claude` on the machine, and without ever touching ~/.claude.
//
// The two methods exist because the two kinds of command have opposite needs.
// A query's output is parsed as JSON, so it has to be captured whole. A
// mutation's output is for the user: `marketplace add` clones a repository —
// measured at ~13s here and minutes on a slow link — and buffering it means a
// silent terminal that is indistinguishable from a hang.
type Runner interface {
	// Run captures standard output for parsing and keeps standard error out
	// of it. Queries only.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	// RunStream runs a mutating command with both its streams written to w as
	// they are produced. w may be nil, which discards them.
	RunStream(ctx context.Context, w io.Writer, name string, args ...string) error
}

// Lookup resolves a program on PATH. It matches exec.LookPath's signature.
type Lookup func(name string) (string, error)

// CommandError carries what a failed command wrote to stderr. Without it the
// user sees "exit status 1" and has nothing to act on.
type CommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	line := strings.Join(e.Args, " ")
	if e.Stderr == "" {
		return fmt.Sprintf("%s falló: %v", line, e.Err)
	}
	return fmt.Sprintf("%s falló: %v: %s", line, e.Err, e.Stderr)
}

func (e *CommandError) Unwrap() error { return e.Err }

// ExecRunner runs the real command.
type ExecRunner struct{}

// Run executes name with args, honouring ctx for both timeout and
// cancellation. Standard output is returned for parsing and standard error is
// kept separate so a JSON parse never has a warning line mixed into it.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = sanitizedEnv()
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	// No stdin: these run unattended, and a prompt with nothing to read it
	// would hang the terminal after the TUI has already exited.
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return out.Bytes(), &CommandError{
			Args:   append([]string{name}, args...),
			Stderr: strings.TrimSpace(errOut.String()),
			Err:    err,
		}
	}
	return out.Bytes(), nil
}

// RunStream executes a mutating command with its output going straight to w
// instead of a buffer, so the user watches the clone happen rather than facing
// a silent terminal for minutes. Both streams go to the same writer: this is
// the command's own progress report, not something that gets parsed.
func (ExecRunner) RunStream(ctx context.Context, w io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = sanitizedEnv()
	if w == nil {
		w = io.Discard
	}
	var errOut bytes.Buffer
	// stderr is teed: the user sees it live, and a copy survives to build the
	// CommandError, which is the only thing that turns "exit status 1" into
	// something actionable.
	cmd.Stdout, cmd.Stderr = w, io.MultiWriter(w, &errOut)
	// No stdin, for the same reason Run has none: nothing is there to answer a
	// prompt once the TUI has exited.
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return &CommandError{
			Args:   append([]string{name}, args...),
			Stderr: strings.TrimSpace(errOut.String()),
			Err:    err,
		}
	}
	return nil
}

// sanitizedEnv is the environment the claude subprocess runs with. The
// variable list and the reason it exists live in internal/execenv: it stops an
// inherited GIT_DIR from redirecting the clone `claude plugin marketplace add`
// performs, and a second copy of a security list drifts the first time only
// one of them is fixed.
func sanitizedEnv() []string { return execenv.Sanitized() }

// State is what was found on this machine.
type State int

const (
	// StateUnknown means a query failed or returned something we refuse to
	// interpret. It is deliberately distinct from "not installed": guessing
	// "not installed" from unreadable output would make deal-kit re-add a
	// marketplace that is already there.
	StateUnknown State = iota
	// StateClaudeMissing means the claude CLI is not on PATH.
	StateClaudeMissing
	// StateMarketplaceMissing means no marketplace named engram is registered.
	StateMarketplaceMissing
	// StateMarketplaceConflict means a marketplace named engram exists but
	// points at a different repository. Never mutated: the name is taken by
	// something the user set up, and replacing it is their decision.
	StateMarketplaceConflict
	// StatePluginMissing means the marketplace is right but the plugin is not
	// installed at user scope.
	StatePluginMissing
	// StatePluginDisabled means it is installed at user scope but disabled.
	StatePluginDisabled
	// StateReady means marketplace, plugin and enablement are all correct.
	StateReady
)

// Status is the resolved state of the machine.
type Status struct {
	State      State
	ClaudePath string // resolved path of the claude executable
	EngramPath string // resolved path of the engram binary, empty when absent
	FoundRepo  string // the repo of the marketplace named engram, when one exists
	Version    string // the installed plugin version, when installed
	Err        error  // why State is StateUnknown
}

// EngramBinaryFound reports whether the engram binary the plugin's hooks call
// is on PATH. The plugin installs and enables without it, but every hook then
// fails at runtime, so it is reported separately rather than folded into State.
func (s Status) EngramBinaryFound() bool { return s.EngramPath != "" }

type marketplace struct {
	Name            string `json:"name"`
	Source          string `json:"source"`
	Repo            string `json:"repo"`
	InstallLocation string `json:"installLocation"`
}

type installedPlugin struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Scope       string `json:"scope"`
	Enabled     bool   `json:"enabled"`
	InstallPath string `json:"installPath"`
}

// Detect resolves the current state without changing anything.
func Detect(ctx context.Context, r Runner, look Lookup) Status {
	if look == nil {
		look = exec.LookPath
	}
	var st Status

	path, err := look(ClaudeBin)
	if err != nil {
		st.State = StateClaudeMissing
		return st
	}
	st.ClaudePath = path
	if p, err := look(EngramBin); err == nil {
		st.EngramPath = p
	}

	markets, err := listMarketplaces(ctx, r, path)
	if err != nil {
		st.State, st.Err = StateUnknown, err
		return st
	}
	mp, found := findMarketplace(markets)
	if !found {
		st.State = StateMarketplaceMissing
		return st
	}
	st.FoundRepo = mp.Repo
	if !sameRepo(mp.Repo, MarketplaceRepo) {
		st.State = StateMarketplaceConflict
		return st
	}

	plugins, err := listPlugins(ctx, r, path)
	if err != nil {
		st.State, st.Err = StateUnknown, err
		return st
	}
	pl, found := findPlugin(plugins)
	if !found {
		st.State = StatePluginMissing
		return st
	}
	st.Version = pl.Version
	if !pl.Enabled {
		st.State = StatePluginDisabled
		return st
	}
	st.State = StateReady
	return st
}

// MarketplaceListArgs and the others are pure functions so a test can assert
// the exact command line without executing anything.
func MarketplaceListArgs() []string {
	return []string{ClaudeBin, "plugin", "marketplace", "list", "--json"}
}

// PluginListArgs lists what is installed. --available is deliberately not
// passed: it changes the response from an array to an object.
func PluginListArgs() []string {
	return []string{ClaudeBin, "plugin", "list", "--json"}
}

func MarketplaceAddArgs() []string {
	return []string{ClaudeBin, "plugin", "marketplace", "add",
		MarketplaceURL + "#" + MarketplaceTag, "--scope", Scope}
}

// PluginInstallArgs installs the plugin. --yes is required because this runs
// with no stdin; the real consent gate is the confirmation screen, not this
// flag.
func PluginInstallArgs() []string {
	return []string{ClaudeBin, "plugin", "install", PluginID, "--scope", Scope, "--yes"}
}

// PluginEnableArgs enables it. `enable` has no --yes flag.
func PluginEnableArgs() []string {
	return []string{ClaudeBin, "plugin", "enable", PluginID, "--scope", Scope}
}

func listMarketplaces(ctx context.Context, r Runner, claudePath string) ([]marketplace, error) {
	args := MarketplaceListArgs()
	out, err := r.Run(ctx, claudePath, args[1:]...)
	if err != nil {
		return nil, err
	}
	var markets []marketplace
	if err := json.Unmarshal(bytes.TrimSpace(out), &markets); err != nil {
		return nil, fmt.Errorf("no se pudo interpretar la salida de `%s`: %w",
			strings.Join(args, " "), err)
	}
	return markets, nil
}

func listPlugins(ctx context.Context, r Runner, claudePath string) ([]installedPlugin, error) {
	args := PluginListArgs()
	out, err := r.Run(ctx, claudePath, args[1:]...)
	if err != nil {
		return nil, err
	}
	var plugins []installedPlugin
	if err := json.Unmarshal(bytes.TrimSpace(out), &plugins); err != nil {
		return nil, fmt.Errorf("no se pudo interpretar la salida de `%s`: %w",
			strings.Join(args, " "), err)
	}
	return plugins, nil
}

func findMarketplace(markets []marketplace) (marketplace, bool) {
	for _, mp := range markets {
		if mp.Name == MarketplaceName {
			return mp, true
		}
	}
	return marketplace{}, false
}

// findPlugin looks only at user scope. A project-scoped or local-scoped copy
// is a different decision from the global one deal-kit offers, so it must not
// make the global install look done.
func findPlugin(plugins []installedPlugin) (installedPlugin, bool) {
	for _, p := range plugins {
		if p.ID == PluginID && p.Scope == Scope {
			return p, true
		}
	}
	return installedPlugin{}, false
}

// sameRepo compares two owner/name pairs the way a host does: case-insensitive
// and indifferent to a .git suffix or surrounding slashes.
func sameRepo(a, b string) bool {
	return normalizeRepo(a) == normalizeRepo(b)
}

func normalizeRepo(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Trim(s, "/")
	return strings.TrimSuffix(s, ".git")
}

// StepKind names one mutation.
type StepKind int

const (
	StepMarketplaceAdd StepKind = iota
	StepInstall
	StepEnable
)

// Step is one command to run.
type Step struct {
	Kind StepKind
	Args []string // program first
}

// Line is the command as a user would type it.
func (s Step) Line() string { return strings.Join(s.Args, " ") }

// Plan is the immutable sequence of mutations that would bring this machine to
// StateReady. The steps are unexported and handed out as a copy so nothing
// downstream — a screen, a renderer — can append to what will be executed.
type Plan struct {
	steps []Step
}

// Steps returns a deep copy of the planned commands. Copying the slice alone
// would still hand out the backing array of every Args, and a caller that
// rewrote one element would change what gets executed.
func (p Plan) Steps() []Step {
	out := make([]Step, len(p.steps))
	for i, s := range p.steps {
		args := make([]string, len(s.Args))
		copy(args, s.Args)
		out[i] = Step{Kind: s.Kind, Args: args}
	}
	return out
}

// Empty reports whether there is nothing to do.
func (p Plan) Empty() bool { return len(p.steps) == 0 }

// NeedsDownload reports whether the plan contacts the network. Enabling a
// plugin that is already on disk does not, so --offline still allows it. It
// lives here rather than next to one of its callers because both the TUI gate
// and the CLI gate must answer this question the same way.
func (p Plan) NeedsDownload() bool {
	for _, s := range p.steps {
		if s.Kind != StepEnable {
			return true
		}
	}
	return false
}

// Lines is every planned command line, for display.
func (p Plan) Lines() []string {
	out := make([]string, 0, len(p.steps))
	for _, s := range p.steps {
		out = append(out, s.Line())
	}
	return out
}

// PlanFor builds the plan for a state. Every state that must not be mutated —
// claude missing, a conflicting marketplace, an unreadable query, and already
// being ready — yields an empty plan, so "do nothing" is decided once here
// rather than at each call site.
func PlanFor(st Status) Plan {
	switch st.State {
	case StateMarketplaceMissing:
		return Plan{steps: []Step{
			{Kind: StepMarketplaceAdd, Args: MarketplaceAddArgs()},
			{Kind: StepInstall, Args: PluginInstallArgs()},
			{Kind: StepEnable, Args: PluginEnableArgs()},
		}}
	case StatePluginMissing:
		return Plan{steps: []Step{
			{Kind: StepInstall, Args: PluginInstallArgs()},
			{Kind: StepEnable, Args: PluginEnableArgs()},
		}}
	case StatePluginDisabled:
		return Plan{steps: []Step{
			{Kind: StepEnable, Args: PluginEnableArgs()},
		}}
	default:
		return Plan{}
	}
}

// Outcome is what Apply did.
type Outcome struct {
	Done   []Step // the steps that succeeded, in order
	Failed *Step  // the step that failed, if one did
	Err    error  // why it failed
	Status Status // the state re-queried after the last successful step
}

// Applied reports whether every planned step ran without an error. It says
// nothing about the machine's final state: the commands can all succeed and
// the re-query that confirms the result still fail.
func (o Outcome) Applied() bool { return o.Err == nil }

// Verified reports success that was confirmed by reading the machine back.
// Applied alone is not enough to call an install done: if the final re-query
// returns malformed JSON, or its budget is gone, Status is StateUnknown and
// what actually landed is not known. Reporting that as plain success is the
// same guess this package refuses to make everywhere else — deciding "not
// installed" from unreadable output.
func (o Outcome) Verified() bool { return o.Err == nil && o.Status.State == StateReady }

// Apply runs the plan, re-querying the state after each mutation so a partial
// failure reports what the machine actually looks like now rather than what
// the plan intended. It stops at the first failure: running `plugin install`
// after `marketplace add` failed would only produce a second, confusing error.
// The live writer receives the mutating commands' output as they produce it;
// nil discards it, which is what the tests use.
func Apply(ctx context.Context, r Runner, look Lookup, p Plan, live io.Writer) Outcome {
	var o Outcome
	if look == nil {
		look = exec.LookPath
	}
	if p.Empty() {
		o.Status = Detect(ctx, r, look)
		return o
	}
	// Run the executable the lookup resolved, not the bare name. Resolving
	// once and executing something else is how a PATH that changed mid-session
	// — or a test that thinks it is hermetic — ends up running a different
	// program than the one that was inspected.
	claudePath, err := look(ClaudeBin)
	if err != nil {
		first := p.Steps()[0]
		o.Failed, o.Err = &first, fmt.Errorf("no se encontró %s en el PATH: %w", ClaudeBin, err)
		o.Status = Status{State: StateClaudeMissing}
		return o
	}
	for _, step := range p.Steps() {
		if err := ctx.Err(); err != nil {
			s := step
			o.Failed, o.Err = &s, err
			o.Status = detectFresh(ctx, r, look)
			return o
		}
		if err := r.RunStream(ctx, live, claudePath, step.Args[1:]...); err != nil {
			s := step
			o.Failed, o.Err = &s, err
			// Re-query even on failure: `marketplace add` can succeed and
			// `install` fail, and the user needs to know the marketplace is
			// now registered so a retry does not look like a no-op.
			o.Status = detectFresh(ctx, r, look)
			return o
		}
		o.Done = append(o.Done, step)
		// Re-query after every mutation: the next step's precondition is the
		// state this one just produced, and reporting the plan's intention
		// instead would hide a step that silently did nothing.
		//
		// On a fresh context, for the same reason the failure path uses one: a
		// clone that ate most of the install budget would leave the shared
		// context with no time for the query, and the run would report
		// StateUnknown for a machine that is fine and readable.
		o.Status = detectFresh(ctx, r, look)
	}
	return o
}

// detectFresh re-queries with a context detached from the one Apply ran on:
// that one may already be cancelled or past its deadline, and then every query
// would fail too and report StateUnknown for a machine we can still read.
func detectFresh(ctx context.Context, r Runner, look Lookup) Status {
	fresh, cancel := context.WithTimeout(context.WithoutCancel(ctx), QueryTimeout)
	defer cancel()
	return Detect(fresh, r, look)
}
