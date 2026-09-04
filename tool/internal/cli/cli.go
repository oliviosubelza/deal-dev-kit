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
	Here    bool   // --here: treat Cwd as the project root

	// Where the kit comes from. KitDir short-circuits the fetch and points at
	// a local checkout, which is how the kit itself is developed.
	KitDir  string
	Repo    string
	Ref     string
	Offline bool

	// ReleaseRepo is where the CLI's own releases are published.
	ReleaseRepo string

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

	// --here bootstraps a directory that has no project markers yet. It stays
	// opt-in: without it, a mistyped path would quietly grow a source tree.
	if e.Here {
		if _, err := os.Stat(filepath.Join(dir, "kit.yaml")); err == nil {
			return "", fmt.Errorf("%s is the kit itself: --here cannot install the kit into the kit", dir)
		}
		return dir, nil
	}
	for {
		// Check for the kit BEFORE the project markers. The kit is itself a
		// git repository, so testing .git first matches it and then fails
		// later with a confusing "what kind of project is this?".
		if _, err := os.Stat(filepath.Join(dir, "kit.yaml")); err == nil {
			// Do not suggest a concrete sibling directory: naming one that may
			// not exist turns the message into a guess the user then chases.
			return "", fmt.Errorf("%s is the kit itself, not a project.\n"+
				"  The kit is what you install FROM. Change into the project you\n"+
				"  want to install INTO, then run deal-kit there.", dir)
		}
		for _, marker := range []string{lockfile.Name, "package.json", ".git"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Name --here on its own, the way the user just invoked the tool:
			// pointing only at `init --here` reads as "this is not for you"
			// when the browser accepts the same flag.
			return "", fmt.Errorf("not inside a project: no package.json, .git or %s found above %s.\n"+
				"  To work in this directory anyway, add --here:\n"+
				"    deal-kit --here\n"+
				"  Or start the project first, then run deal-kit again:\n"+
				"    git init  ·  pnpm init  ·  pnpm create vite",
				lockfile.Name, e.Cwd)
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
	// An existing lockfile keeps whatever it already had installed, minus
	// anything the manifest no longer declares.
	installed, orphans := m.PartitionInstalled(lockIDs(lock))
	for _, id := range installed {
		ids = appendUnique(ids, id)
	}

	roots := spec.Roots
	if existed && len(lock.Roots) > 0 {
		roots = lock.Roots // a project may override the standard layout
	}

	fmt.Fprintf(e.Stdout, "  project   %s\n", root)
	fmt.Fprintf(e.Stdout, "  detected  %s\n", pt)
	fmt.Fprintf(e.Stdout, "  kit       %s\n", ck.Version)
	fmt.Fprintf(e.Stdout, "  roots     %s\n", formatRoots(roots))
	fmt.Fprintf(e.Stdout, "  profile   %s\n\n", pt)

	lock.ProjectType = string(pt)
	lock.Roots = roots
	return syncArtifacts(e, root, ck, m, lock, pt, ids, orphans)
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

	// The ids the user named go through unfiltered: a typo there must fail,
	// and the manifest is the authority on what can be added.
	want := ids
	installed, orphans := m.PartitionInstalled(lockIDs(lock))
	for _, id := range installed {
		want = appendUnique(want, id)
	}
	return syncArtifacts(e, root, ck, m, lock, pt, want, orphans)
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
	// The lockfile describes history: an id it records that the manifest no
	// longer declares is orphaned, and reporting it is the only way the user
	// can act on it. Passing it to Resolve would abort the one command that
	// could tell them about it.
	ids, orphans := m.PartitionInstalled(lockIDs(lock))
	sort.Strings(ids)
	sort.Strings(orphans)

	pinned := lock.KitVersion
	if pinned == "" {
		pinned = "(unpinned)"
	}
	fmt.Fprintf(e.Stdout, "  project   %s\n", root)
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
		Artifacts: artifacts, Orphans: orphans, Lock: lock,
		KitDir: ck.Dir, ProjectDir: root, Roots: lock.Roots,
		Rewrites: m.Rewrites(pt),
	})
	if err != nil {
		return err
	}
	renderStatus(e.Stdout, artifacts, orphans, p)
	return nil
}

// syncArtifacts resolves, plans, confirms and applies. Every command that
// changes the project goes through here, so the TUI and the flags can never
// diverge in what they actually do.
// orphans are ids the lockfile records that the manifest no longer declares;
// they are removed rather than installed. They must arrive already separated
// from ids, which every caller does with kit.Manifest.PartitionInstalled.
func syncArtifacts(e Env, root string, ck kit.Checkout, m *kit.Manifest, lock *lockfile.File, pt kit.ProjectType, ids, orphans []string) error {
	artifacts, err := m.Resolve(pt, ids)
	if err != nil {
		return err
	}
	p, err := plan.Build(plan.Input{
		Artifacts: artifacts, Orphans: orphans, Lock: lock,
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
	renderSummary(e.Stdout, root, p)
	fmt.Fprintf(e.Stdout, "\n  listo: %d archivo(s)\n", len(p.Changes()))
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

// lockIDs is every artifact id the project's lockfile records, in file order.
func lockIDs(lock *lockfile.File) []string {
	out := make([]string, 0, len(lock.Artifacts))
	for _, a := range lock.Artifacts {
		out = append(out, a.ID)
	}
	return out
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
