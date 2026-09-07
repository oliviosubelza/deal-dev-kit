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
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/execenv"
)

// The fixed command surface. These are the only values the commands are built
// from.
const (
	// ClaudeBin is the Claude Code CLI, which owns the plugin installation.
	ClaudeBin = "claude"
	// EngramBin is the Engram binary itself. The plugin's MCP server and every
	// one of its hooks invoke it, so deal-kit installs it before the plugin;
	// how it is acquired lives in binary.go.
	EngramBin = "engram"

	// MarketplaceName is how the marketplace registers itself.
	MarketplaceName = "engram"
	// MarketplaceRepo identifies the legitimate marketplace, in the owner/name
	// form a host uses. `claude plugin marketplace list --json` reports that
	// identity in one of two different fields depending on how the marketplace
	// was added: the `owner/name` shorthand yields `source: "github"` with a
	// `repo`, while a URL yields `source: "git"` with `url` and `ref` and no
	// `repo` at all. deal-kit adds by URL, so the second shape is the one its
	// own installs produce; both have to resolve to this.
	MarketplaceRepo = "Gentleman-Programming/engram"
	// MarketplaceHost is the host MarketplaceRepo is a repository of. A URL
	// pointing at the same owner/name somewhere else is a different
	// repository, so identity is only read out of a URL on this host.
	// TestTheMarketplaceURLResolvesToTheMarketplaceRepo pins it to
	// MarketplaceURL so the two constants cannot drift apart.
	MarketplaceHost = "github.com"
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
	GoPath     string // resolved path of the go toolchain, empty when absent
	FoundRepo  string // what the marketplace named engram points at, when one exists
	FoundRef   string // the ref our marketplace is pinned at, when the query reports one
	Version    string // the installed plugin version, when installed
	Err        error  // why State is StateUnknown
}

// EngramBinaryFound reports whether the engram binary the plugin's hooks call
// is on PATH. The plugin installs and enables without it, but every hook then
// fails at runtime, so it is reported separately rather than folded into State.
func (s Status) EngramBinaryFound() bool { return s.EngramPath != "" }

// RefKnown reports whether the query said which ref the marketplace is
// registered at. Only the URL form carries one: a marketplace added with the
// owner/name shorthand reports no ref at all, and an unknown ref is not a
// mismatch.
func (s Status) RefKnown() bool { return s.FoundRef != "" }

// RefMismatch reports that our marketplace is registered at a ref other than
// the one deal-kit pins.
//
// It is deliberately not a State. The states are a ladder of how far the
// install got, and a marketplace at another tag is neither further nor less
// far along: it is the right repository, so nothing about it is a conflict,
// and the plugin can still be installed and enabled from it. Making it a state
// would also mean PlanFor answering it, and every answer is wrong — an empty
// plan would refuse to install a plugin that installs fine, and a re-pointing
// plan would `marketplace remove` something the user registered. So it is
// reported and never acted on, the same way a conflicting marketplace is.
//
// FoundRef is only ever set for our own marketplace (see Detect), so this
// never comments on a stranger's pinning.
func (s Status) RefMismatch() bool { return s.RefKnown() && s.FoundRef != MarketplaceTag }

// GoFound reports whether the Go toolchain is on PATH. It decides which of the
// two acquisition paths the plan uses, so it is resolved once here rather than
// looked up again when the plan is built.
func (s Status) GoFound() bool { return s.GoPath != "" }

// marketplace is one entry of `claude plugin marketplace list --json`. Both
// shapes are decoded into it: `repo` is filled by the owner/name shorthand,
// `url` and `ref` by a marketplace added from a URL — which is what
// MarketplaceAddArgs does, so the second shape is deal-kit's own.
type marketplace struct {
	Name            string `json:"name"`
	Source          string `json:"source"`
	Repo            string `json:"repo"`
	URL             string `json:"url"`
	Ref             string `json:"ref"`
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
	if p, err := look(GoBin); err == nil {
		st.GoPath = p
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
	repo, shown := mp.identity()
	// Shown rather than repo: when the URL cannot be read as owner/name there
	// is no repo to name, and the conflict screen has to say what it actually
	// found instead of an empty dash.
	st.FoundRepo = shown
	if repo == "" || !sameRepo(repo, MarketplaceRepo) {
		st.State = StateMarketplaceConflict
		return st
	}
	// Only now, once this is known to be our marketplace: what ref somebody
	// else's marketplace is pinned at is none of deal-kit's business.
	st.FoundRef = strings.TrimSpace(mp.Ref)

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

// identity is what this marketplace points at: repo is the owner/name to
// compare against MarketplaceRepo, and shown is what to put in front of the
// user for it.
//
// They differ in exactly one case. A URL that cannot be reduced to owner/name
// yields no repo — an unreadable URL is not evidence of a match, and this
// package refuses to guess everywhere else — but it is still the concrete
// thing the machine has registered, so it is what gets shown.
func (m marketplace) identity() (repo, shown string) {
	if r := strings.TrimSpace(m.Repo); r != "" {
		return r, r
	}
	u := strings.TrimSpace(m.URL)
	if u == "" {
		return "", ""
	}
	if r := repoFromURL(u); r != "" {
		return r, r
	}
	return "", u
}

// repoFromURL reduces a git remote to owner/name, or "" when it cannot.
//
// It reads the two forms git accepts and `claude` reports: a URL with a scheme
// and the scp-style ssh address (git@github.com:owner/name.git). The ssh form
// costs one branch, and without it a marketplace someone added over ssh — the
// normal thing to do with a repository you also push to — is indistinguishable
// from a stranger's.
//
// Anything else returns "": another host, a path that is not exactly two
// segments, or a form with no host at all. Unknown stays unknown, which for
// this caller means a conflict that is reported and never mutated, rather than
// a match nobody verified.
func repoFromURL(raw string) string {
	s := strings.TrimSpace(raw)
	// A query or fragment is not part of the identity. `#ref` in particular is
	// how MarketplaceAddArgs pins the tag, and `claude` may echo it back.
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	var host, path string
	switch {
	case strings.Contains(s, "://"):
		s = s[strings.Index(s, "://")+len("://"):]
		// Drop userinfo: https://token@github.com/owner/name is the same
		// repository as the one without it.
		if i := strings.LastIndex(s, "@"); i >= 0 {
			s = s[i+1:]
		}
		i := strings.Index(s, "/")
		if i < 0 {
			return ""
		}
		host, path = s[:i], s[i+1:]
	case strings.Contains(s, "@") && strings.Contains(s, ":"):
		s = s[strings.Index(s, "@")+1:]
		i := strings.Index(s, ":")
		host, path = s[:i], s[i+1:]
	default:
		return ""
	}
	// A port is not part of the identity either.
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	if !strings.EqualFold(host, MarketplaceHost) {
		return ""
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	owner, name, ok := strings.Cut(path, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ""
	}
	return owner + "/" + name
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
	// StepGoInstall builds the engram binary with the Go toolchain.
	StepGoInstall
	// StepBinaryDownload fetches the pinned release asset. It is the only
	// step that is not an external command, so Apply dispatches on Kind.
	StepBinaryDownload
)

// Step is one mutation. Most steps are external commands, described by Args
// with the program first. StepBinaryDownload is not a command: it carries Get
// instead, and Note is what the confirmation screen shows for it, so Line()
// stays honest for every kind rather than rendering an empty command line.
type Step struct {
	Kind StepKind
	Args []string  // program first; empty for a step that is not a command
	Note string    // display line for a step that is not a command
	Get  *Download // the asset to fetch, for StepBinaryDownload only
}

// Line is the step as a user would read it: the command for a command, and the
// plain-language description for the download.
//
// It dispatches on Kind, the same tag runStep switches on. Deciding "is this a
// command?" from len(Args) instead would be a second, independent dispatch over
// one tagged union: a StepKind added later would execute down runStep's default
// branch while rendering an empty line into the confirmation screen.
func (s Step) Line() string {
	switch s.Kind {
	case StepBinaryDownload:
		return s.Note
	default:
		return strings.Join(s.Args, " ")
	}
}

// Plan is the immutable sequence of mutations that would bring this machine to
// StateReady. The steps are unexported and handed out as a copy so nothing
// downstream — a screen, a renderer — can append to what will be executed.
type Plan struct {
	steps []Step
	// blocked is why the plan is empty when emptiness is a refusal rather
	// than "nothing to do" — an unsupported platform with no Go toolchain,
	// for instance. Callers show it instead of "no hay nada que hacer".
	blocked string
}

// Steps returns a deep copy of the planned commands. Copying the slice alone
// would still hand out the backing array of every Args, and a caller that
// rewrote one element would change what gets executed.
func (p Plan) Steps() []Step {
	out := make([]Step, len(p.steps))
	for i, s := range p.steps {
		args := make([]string, len(s.Args))
		copy(args, s.Args)
		out[i] = Step{Kind: s.Kind, Args: args, Note: s.Note}
		// The download descriptor is copied too, for the same reason the
		// arguments are: handing out the pointer would let a caller retarget
		// where the binary lands.
		if s.Get != nil {
			get := *s.Get
			out[i].Get = &get
		}
	}
	return out
}

// Empty reports whether there is nothing to do.
func (p Plan) Empty() bool { return len(p.steps) == 0 }

// Blocked is why an empty plan is a refusal rather than a machine that is
// already fine, or "" when it is not a refusal.
func (p Plan) Blocked() string { return p.blocked }

// Binary returns the download the plan would perform, if it has one. It is how
// a screen names the destination directory before the user consents.
func (p Plan) Binary() (Download, bool) {
	for _, s := range p.steps {
		if s.Kind == StepBinaryDownload && s.Get != nil {
			return *s.Get, true
		}
	}
	return Download{}, false
}

// InstallsBinary reports whether the plan acquires the engram binary, by
// either route.
func (p Plan) InstallsBinary() bool {
	for _, s := range p.steps {
		if s.Kind == StepGoInstall || s.Kind == StepBinaryDownload {
			return true
		}
	}
	return false
}

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
// claude missing, a conflicting marketplace and an unreadable query — yields an
// empty plan, so "do nothing" is decided once here rather than at each call
// site.
//
// The binary step goes first when the binary is absent. It is the engine: the
// plugin's MCP server and every one of its hooks invoke `engram`, so installing
// the plugin ahead of it produces a Claude Code that starts and then fails at
// every hook. That is also why a machine whose plugin is already StateReady
// still gets a plan when the binary is missing.
func PlanFor(st Status) Plan { return planFor(runtime.GOOS, runtime.GOARCH, st) }

// planFor takes the platform as arguments so a test can ask what an
// unsupported one produces without pretending to be it.
func planFor(goos, goarch string, st Status) Plan {
	var steps []Step
	switch st.State {
	// `plugin install` leaves the plugin enabled, so no plan that installs
	// also enables: `enable` then fails with "already enabled at user scope"
	// and a working install reports failure. Found by running the installer
	// against an empty HOME; the simulated `claude` in the tests accepts any
	// `enable`, which is why the suite never saw it.
	case StateMarketplaceMissing:
		steps = []Step{
			{Kind: StepMarketplaceAdd, Args: MarketplaceAddArgs()},
			{Kind: StepInstall, Args: PluginInstallArgs()},
		}
	case StatePluginMissing:
		steps = []Step{
			{Kind: StepInstall, Args: PluginInstallArgs()},
		}
	// The only state where enabling is its own job: it is already on disk.
	case StatePluginDisabled:
		steps = []Step{{Kind: StepEnable, Args: PluginEnableArgs()}}
	case StateReady:
		// Nothing to do to the plugin; the binary check below may still
		// have something to do.
	default:
		return Plan{}
	}
	if st.EngramBinaryFound() {
		return Plan{steps: steps}
	}
	step, blocked := binaryStep(goos, goarch, st)
	if blocked != "" {
		// Refused rather than reduced to the plugin steps. A plugin whose
		// every hook fails is the bug this step exists to fix, so deal-kit
		// says what is missing instead of installing half of it.
		return Plan{blocked: blocked}
	}
	return Plan{steps: append([]Step{step}, steps...)}
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
// The binary counts too: the plugin declares an MCP server and hooks that
// shell out to `engram`, so a StateReady plugin with no binary on PATH is not
// a working install and must not be reported as one.
func (o Outcome) Verified() bool {
	return o.Err == nil && o.Status.State == StateReady && o.Status.EngramBinaryFound()
}

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
	// The step that ran last and what it cost, so an exhausted budget can name
	// who spent it instead of blaming whichever step happened to be next.
	var prev *Step
	var prevTook time.Duration
	for _, step := range p.Steps() {
		if err := ctx.Err(); err != nil {
			s := step
			o.Failed, o.Err = &s, budgetErr(err, step, prev, prevTook)
			o.Status = detectFresh(ctx, r, look)
			return o
		}
		started := time.Now()
		if err := runStep(ctx, r, look, claudePath, step, live); err != nil {
			s := step
			o.Failed, o.Err = &s, err
			// Re-query even on failure: `marketplace add` can succeed and
			// `install` fail, and the user needs to know the marketplace is
			// now registered so a retry does not look like a no-op.
			o.Status = detectFresh(ctx, r, look)
			return o
		}
		o.Done = append(o.Done, step)
		s := step
		prev, prevTook = &s, time.Since(started)
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

// budgetErr names the step that consumed the shared budget.
//
// InstallTimeout covers the whole plan, and the binary download runs first. A
// slow download therefore starves `marketplace add`, which fails on ctx.Err()
// for a reason that is not its own and is indistinguishable from its own hang.
// Naming the consumer was chosen over splitting the budget per step: it is the
// smaller change, it makes the failure honest, and it invents no per-step
// timeouts that nobody has measured against a real slow link. The cause is
// wrapped, so errors.Is against context.DeadlineExceeded still answers.
func budgetErr(err error, step Step, prev *Step, took time.Duration) error {
	if prev == nil {
		return err
	}
	return fmt.Errorf("no quedó presupuesto (%s en total) para `%s`: lo consumió `%s`, que tardó %s: %w",
		InstallTimeout, step.Line(), prev.Line(), took.Round(time.Second), err)
}

// runStep performs one step. Everything but the download is an external
// command, run as the path the lookup resolved rather than the bare name — for
// the same reason Apply resolves `claude` once: resolving one thing and
// executing another is how a "hermetic" test ends up running the real program.
func runStep(ctx context.Context, r Runner, look Lookup, claudePath string, step Step, live io.Writer) error {
	switch step.Kind {
	case StepBinaryDownload:
		if step.Get == nil {
			return errors.New("paso de descarga sin destino")
		}
		return fetchBinary(ctx, *step.Get, live)
	case StepGoInstall:
		goPath, err := look(GoBin)
		if err != nil {
			return fmt.Errorf("no se encontró %s en el PATH: %w", GoBin, err)
		}
		return r.RunStream(ctx, live, goPath, step.Args[1:]...)
	default:
		return r.RunStream(ctx, live, claudePath, step.Args[1:]...)
	}
}

// detectFresh re-queries with a context detached from the one Apply ran on:
// that one may already be cancelled or past its deadline, and then every query
// would fail too and report StateUnknown for a machine we can still read.
func detectFresh(ctx context.Context, r Runner, look Lookup) Status {
	fresh, cancel := context.WithTimeout(context.WithoutCancel(ctx), QueryTimeout)
	defer cancel()
	return Detect(fresh, r, look)
}
