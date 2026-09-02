// Package cli implements the deal-kit commands. Everything the commands touch
// arrives through Env, so the whole surface is testable without a real
// terminal, home directory, or kit checkout.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/pm"
)

// Env is the environment a command runs in.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	Cwd     string // where the command was invoked
	Version string // the CLI's own version

	// Where the kit comes from. KitDir short-circuits the fetch and points at
	// a local checkout, which is how the kit itself is developed.
	KitDir  string
	Repo    string
	Ref     string
	Offline bool

	AssumeYes bool // --yes: apply without confirming
	DryRun    bool // --dry-run: print the plan and stop
	NoDeps    bool // --no-deps: skip the package manager install
}

// ErrAborted is returned when the user declines to apply a plan.
var ErrAborted = errors.New("aborted")

// kit resolves the kit to work against, cloning or fetching unless a local
// directory was given.
func (e Env) kit() (kit.Checkout, error) {
	if e.KitDir != "" {
		return kit.Checkout{Dir: e.KitDir, Version: "local"}, nil
	}
	return kit.Fetch(kit.Source{Repo: e.Repo, Ref: e.Ref, Offline: e.Offline})
}

// ProjectRoot walks up from Cwd to the nearest directory that looks like a
// project. Commands must never write relative to Cwd itself: a developer
// running deal-kit from src/features would otherwise install a second tree.
func (e Env) ProjectRoot() (string, error) {
	dir := filepath.Clean(e.Cwd)
	for {
		for _, marker := range []string{lockfile.Name, "package.json", ".git"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}
		// Running from inside the kit itself is the most likely mistake: the
		// kit is what you install FROM, not into.
		if _, err := os.Stat(filepath.Join(dir, "kit.yaml")); err == nil {
			return "", fmt.Errorf("%s is the kit itself, not a project.\n"+
				"  Run deal-kit from inside the project you want to install into:\n"+
				"    cd ../my-project && deal-kit --kit-dir %s", dir, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a project: no package.json, .git or %s found above %s.\n"+
				"  Create the project first, or cd into an existing one.", lockfile.Name, e.Cwd)
		}
		dir = parent
	}
}

// Init sets up the current project: detects its type, installs the matching
// profile, and writes the lockfile.
func Init(e Env, typeOverride string) error {
	root, err := e.ProjectRoot()
	if err != nil {
		return err
	}
	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("reading the kit: %w", err)
	}

	lock, existed, err := lockfile.Load(root)
	if err != nil {
		return err
	}

	pt, err := resolveProjectType(m, root, typeOverride, lock, existed)
	if err != nil {
		return err
	}
	spec := m.ProjectTypes[pt]

	ids := m.Profiles[pt]
	if len(ids) == 0 {
		return fmt.Errorf("project type %s has no profile in kit.yaml", pt)
	}
	// An existing lockfile keeps whatever it already had installed.
	for _, a := range lock.Artifacts {
		ids = appendUnique(ids, a.ID)
	}

	roots := spec.Roots
	if existed && len(lock.Roots) > 0 {
		roots = lock.Roots // a project may override the standard layout
	}

	fmt.Fprintf(e.Stdout, "  detected  %s  (%s)\n", pt, filepath.Base(root))
	fmt.Fprintf(e.Stdout, "  kit       %s\n", ck.Version)
	fmt.Fprintf(e.Stdout, "  roots     %s\n", formatRoots(roots))
	fmt.Fprintf(e.Stdout, "  profile   %s\n\n", pt)

	lock.ProjectType = string(pt)
	lock.Roots = roots
	return syncArtifacts(e, root, ck, m, lock, pt, ids)
}

// Add installs additional artifacts into an initialised project.
func Add(e Env, ids []string) error {
	if len(ids) == 0 {
		return errors.New("nothing to add: name at least one artifact (see `deal-kit status`)")
	}
	root, err := e.ProjectRoot()
	if err != nil {
		return err
	}
	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("reading the kit: %w", err)
	}
	lock, existed, err := lockfile.Load(root)
	if err != nil {
		return err
	}
	if !existed {
		return fmt.Errorf("this project has no %s yet: run `deal-kit init` first", lockfile.Name)
	}
	pt := kit.ProjectType(lock.ProjectType)
	if _, ok := m.ProjectTypes[pt]; !ok {
		return fmt.Errorf("%s records unknown project type %q", lockfile.Name, lock.ProjectType)
	}

	want := ids
	for _, a := range lock.Artifacts {
		want = appendUnique(want, a.ID)
	}
	return syncArtifacts(e, root, ck, m, lock, pt, want)
}

// Status reports what is installed and whether it has drifted.
func Status(e Env) error {
	root, err := e.ProjectRoot()
	if err != nil {
		return err
	}
	lock, existed, err := lockfile.Load(root)
	if err != nil {
		return err
	}
	if !existed {
		fmt.Fprintf(e.Stdout, "no %s in %s\n\nrun `deal-kit init` to set this project up\n",
			lockfile.Name, filepath.Base(root))
		return nil
	}
	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("reading the kit: %w", err)
	}

	pt := kit.ProjectType(lock.ProjectType)
	var ids []string
	for _, a := range lock.Artifacts {
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)

	pinned := lock.KitVersion
	if pinned == "" {
		pinned = "(unpinned)"
	}
	fmt.Fprintf(e.Stdout, "  kit       %s\n  type      %s\n", pinned, pt)
	if ck.Version != pinned && ck.Version != "local" {
		fmt.Fprintf(e.Stdout, "  available %s  (run `deal-kit update`)\n", ck.Version)
	}
	fmt.Fprintln(e.Stdout)

	artifacts, err := m.Resolve(pt, ids)
	if err != nil {
		return err
	}
	p, err := plan.Build(plan.Input{
		Artifacts: artifacts, Lock: lock,
		KitDir: ck.Dir, ProjectDir: root, Roots: lock.Roots,
		Rewrites: m.Rewrites(pt),
	})
	if err != nil {
		return err
	}
	renderStatus(e.Stdout, artifacts, p)
	return nil
}

// syncArtifacts resolves, plans, confirms and applies. Every command that
// changes the project goes through here, so the TUI and the flags can never
// diverge in what they actually do.
func syncArtifacts(e Env, root string, ck kit.Checkout, m *kit.Manifest, lock *lockfile.File, pt kit.ProjectType, ids []string) error {
	artifacts, err := m.Resolve(pt, ids)
	if err != nil {
		return err
	}
	p, err := plan.Build(plan.Input{
		Artifacts: artifacts, Lock: lock,
		KitDir: ck.Dir, ProjectDir: root, Roots: lock.Roots,
		Rewrites: m.Rewrites(pt),
	})
	if err != nil {
		return err
	}

	manager, hasManager := pm.Detect(root)
	renderPlan(e.Stdout, p, manager, hasManager, e.NoDeps)

	if blocked := p.Blocked(); len(blocked) > 0 {
		return fmt.Errorf("%d file(s) need attention before anything is written; resolve them and run again", len(blocked))
	}
	if len(p.Changes()) == 0 {
		// The pin can still move even when no file changes.
		if lock.KitVersion != ck.Version && ck.Version != "local" {
			lock.KitVersion = ck.Version
			if err := lock.Save(root); err != nil {
				return err
			}
			fmt.Fprintf(e.Stdout, "\n  already up to date (pinned to %s)\n", ck.Version)
			return nil
		}
		fmt.Fprintln(e.Stdout, "\n  already up to date")
		return nil
	}
	if e.DryRun {
		fmt.Fprintln(e.Stdout, "\n  --dry-run: nothing was written")
		return nil
	}
	if !e.AssumeYes {
		ok, err := confirm(e, "apply?")
		if err != nil {
			return err
		}
		if !ok {
			return ErrAborted
		}
	}

	if err := p.Apply(root, lock); err != nil {
		return err
	}
	if ck.Version != "local" {
		lock.KitVersion = ck.Version
	}
	if err := lock.Save(root); err != nil {
		return err
	}

	if len(p.Deps) > 0 && !e.NoDeps {
		if !hasManager {
			fmt.Fprintf(e.Stderr, "\n  warning: no package manager detected; install these yourself:\n    %s\n",
				strings.Join(pm.InstallArgs(pm.NPM, p.Deps)[2:], " "))
		} else if err := pm.Install(root, manager, p.Deps, e.Stdout); err != nil {
			return fmt.Errorf("dependency install failed: %w", err)
		}
	}
	fmt.Fprintf(e.Stdout, "\n  done: %d file(s) changed\n", len(p.Changes()))
	return nil
}

func resolveProjectType(m *kit.Manifest, root, override string, lock *lockfile.File, existed bool) (kit.ProjectType, error) {
	if override != "" {
		pt := kit.ProjectType(override)
		if _, ok := m.ProjectTypes[pt]; !ok {
			return "", fmt.Errorf("unknown project type %q (known: %s)", override, strings.Join(projectTypeNames(m), ", "))
		}
		return pt, nil
	}
	if existed && lock.ProjectType != "" {
		return kit.ProjectType(lock.ProjectType), nil
	}
	if pt, ok := m.MatchProjectType(filepath.Base(root)); ok {
		return pt, nil
	}
	return "", fmt.Errorf("could not tell what kind of project %q is; pass --type (one of: %s)",
		filepath.Base(root), strings.Join(projectTypeNames(m), ", "))
}

func projectTypeNames(m *kit.Manifest) []string {
	var out []string
	for pt := range m.ProjectTypes {
		out = append(out, string(pt))
	}
	sort.Strings(out)
	return out
}

func formatRoots(roots map[string]string) string {
	keys := make([]string, 0, len(roots))
	for k := range roots {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s → %s", k, roots[k]))
	}
	return strings.Join(parts, ", ")
}

func appendUnique(ss []string, s string) []string {
	for _, existing := range ss {
		if existing == s {
			return ss
		}
	}
	return append(ss, s)
}

func confirm(e Env, question string) (bool, error) {
	fmt.Fprintf(e.Stdout, "\n  %s [y/N] ", question)
	r := bufio.NewReader(e.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
