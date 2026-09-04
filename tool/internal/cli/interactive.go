package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/engram"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/pm"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/tui"
)

// runBootstrap is the create screen, indirected so the routing into it can be
// tested without a terminal.
var runBootstrap = tui.RunBootstrap

// The Engram entry points are indirected the same way, so the routing into
// them can be exercised without a `claude` on the machine and without ever
// touching the user's global configuration.
var (
	engramDetect               = engram.Detect
	engramApply                = engram.Apply
	engramRunner engram.Runner = engram.ExecRunner{}
	engramLook   engram.Lookup = exec.LookPath
)

// Interactive opens the artifact browser. Discovery happens here so the TUI
// receives a fully resolved Config and never guesses at the environment.
func Interactive(e Env, typeOverride string) error {
	root, err := e.ProjectRoot()
	if err != nil {
		// Not being in a project used to end here, with a message that told
		// the user to learn a flag and come back. On a terminal it is a
		// question the tool can just ask. Off a terminal nothing can answer a
		// screen, so the message stands exactly as it was.
		if !IsNotAProject(err) || !tui.IsTerminal(e.Stdout) {
			return err
		}
		return createInteractively(e, typeOverride)
	}
	if !tui.IsTerminal(e.Stdout) {
		return fmt.Errorf("no es una terminal: usar `deal-kit init`, `add` o `status` con --yes para uso no interactivo")
	}
	return browse(e, root, typeOverride)
}

// createInteractively asks the two questions the user has not answered — work
// here, and which type — then hands both to `deal-kit new`. Creation is never
// reimplemented here: New already resolves the target, refuses a non-empty
// directory and gates on the doctor before writing anything, and a second
// route would drift from it.
func createInteractively(e Env, typeOverride string) error {
	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("no se pudo leer el kit: %w", err)
	}

	dir, err := filepath.Abs(e.Cwd)
	if err != nil {
		return err
	}
	res, err := runBootstrap(tui.BootstrapConfig{
		Dir:        dir,
		Markers:    projectMarkers,
		Types:      m.ProjectTypeNames(),
		Selected:   typeOverride,
		CLIVersion: e.Version,
	})
	if err != nil {
		return err
	}
	if !res.Confirmed {
		// The confirmation is the interactive form of --here, which is opt-in
		// so a mistyped cd cannot grow a source tree. Declining writes nothing.
		fmt.Fprintf(e.Stderr, "no se creó nada en %s.\n"+
			"  Hacer cd al directorio del proyecto y volver a ejecutar deal-kit.\n", dir)
		return nil
	}

	// "." rather than the absolute path: create-vite joins its argument onto
	// the working directory even when it is already absolute.
	if err := New(e, ".", res.ProjectType); err != nil {
		return err
	}

	// The generator wrote the markers, so discovery now succeeds and the
	// session continues in the browser like any other project.
	root, err := e.ProjectRoot()
	if err != nil {
		return err
	}
	return browse(e, root, res.ProjectType)
}

// browse resolves everything the TUI needs and runs the artifact browser.
func browse(e Env, root, typeOverride string) error {
	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("no se pudo leer el kit: %w", err)
	}
	lock, existed, err := lockfile.Load(root)
	if err != nil {
		return err
	}
	pt, err := resolveProjectType(m, root, typeOverride, lock, existed)
	if err != nil {
		return err
	}

	roots := m.ProjectTypes[pt].Roots
	if existed && len(lock.Roots) > 0 {
		roots = lock.Roots
	}
	lock.ProjectType = string(pt)
	lock.Roots = roots

	manager, _ := pm.Detect(root)
	// Resolve the Claude Code plugin state here, like everything else the TUI
	// receives: the screen renders what it is given and never queries.
	engramStatus, engramPlan := resolveEngram()

	final, err := tui.Run(tui.Config{
		ProjectName: filepath.Base(root),
		ProjectType: pt,
		ProjectRoot: root,
		KitDir:      ck.Dir,
		KitVersion:  ck.Version,
		PinnedKit:   lock.KitVersion,
		CLIVersion:  e.Version,
		Manifest:    m,
		Lock:        lock,
		Roots:       roots,
		Rewrites:    m.Rewrites(pt),
		PackageMgr:  string(manager),
		Engram:      engramStatus,
		EngramPlan:  engramPlan,
		DryRun:      e.DryRun,
		Offline:     e.Offline,
	})
	if err != nil {
		return err
	}

	// The plugin install runs out here for the same reason the dependency
	// install does: claude streams its own progress, and that belongs in the
	// normal terminal rather than inside an alternate screen. --dry-run is
	// re-checked rather than trusted to the screen: a mutation this far from
	// the flag deserves two gates.
	if final.EngramIntent() && !e.DryRun {
		return installEngram(e, engramPlan)
	}

	// Dependency installs run outside the TUI: the package manager's streamed
	// output belongs in the normal terminal, not inside an alternate screen.
	applied, deps := final.Result()
	if !applied || len(deps) == 0 || e.NoDeps {
		return nil
	}
	if manager == "" {
		fmt.Fprintf(e.Stderr, "no se detectó ningún package manager; instalar estas dependencias a mano:\n  %s\n",
			strings.Join(pm.InstallArgs(pm.NPM, deps)[2:], " "))
		return nil
	}
	fmt.Fprintf(e.Stdout, "\ninstalando dependencias con %s\n", manager)
	return pm.Install(root, manager, deps, e.Stdout)
}

// resolveEngram queries the Claude Code plugin state. Both queries are reads,
// so they run even with --offline; only the mutating steps are held back.
func resolveEngram() (engram.Status, engram.Plan) {
	ctx, cancel := context.WithTimeout(context.Background(), engram.QueryTimeout)
	defer cancel()
	st := engramDetect(ctx, engramRunner, engramLook)
	return st, engram.PlanFor(st)
}

// installEngram runs the confirmed plan and reports what the machine looks
// like afterwards. The commands are printed before they run, the way
// `deal-kit new` prints the generator it is about to invoke.
func installEngram(e Env, p engram.Plan) error {
	if p.Empty() {
		return nil
	}
	// The second gate for --offline, matching the one --dry-run already gets
	// in browse(). The TUI refuses this today, but that is a single check on
	// the far side of a screen: any future change to engramBlocked() would let
	// a network clone run under --offline with nothing downstream to stop it.
	// Enabling a plugin already on disk downloads nothing, so it stays allowed
	// — the same rule the screen encodes.
	if e.Offline && p.NeedsDownload() {
		return fmt.Errorf("--offline: instalar el plugin Engram requiere descargar el marketplace")
	}
	fmt.Fprintf(e.Stdout, "\ninstalando el plugin Engram en Claude Code (alcance %s)\n\n", engram.Scope)
	for _, line := range p.Lines() {
		fmt.Fprintf(e.Stdout, "  %s\n", line)
	}

	ctx, cancel := engramInstallContext()
	defer cancel()
	// e.Stdout is handed to Apply so the mutating commands stream into the
	// normal terminal. `marketplace add` clones a repository; buffering it
	// leaves the user staring at nothing for as long as that takes.
	out := engramApply(ctx, engramRunner, engramLook, p, e.Stdout)

	renderEngram(e.Stdout, e.Stderr, p, out)
	if !out.Applied() {
		return fmt.Errorf("no se pudo instalar el plugin Engram: %w", out.Err)
	}
	// Applied is not success. If the re-query after the last command could not
	// read the machine back, what landed is unknown, and exiting 0 would tell
	// the user it is done. Reported as a failure of verification, not of the
	// install, and never silently.
	if !out.Verified() {
		return fmt.Errorf("los comandos se ejecutaron pero no se pudo confirmar el estado del plugin Engram: %s",
			engramStateLabel(out.Status))
	}
	return nil
}

// engramInstallContext carries the install budget and a cancel wired to
// SIGINT/SIGTERM. Without it Go's default disposition kills deal-kit on the
// first Ctrl+C, and engram.Apply's graceful path — stop, re-query on a
// detached context, report what actually landed on the machine — never runs.
// The interrupt reaches the whole foreground process group, so `claude` and
// its `git` usually die on their own; what this buys is that deal-kit outlives
// them long enough to say what happened. Deliberately scoped to this call:
// signal handling for the rest of the CLI is a separate decision.
func engramInstallContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithTimeout(ctx, engram.InstallTimeout)
	return ctx, func() {
		cancel()
		stop()
	}
}
