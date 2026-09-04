package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/pm"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/tui"
)

// runBootstrap is the create screen, indirected so the routing into it can be
// tested without a terminal.
var runBootstrap = tui.RunBootstrap

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
	})
	if err != nil {
		return err
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
