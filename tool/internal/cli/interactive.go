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

// Interactive opens the artifact browser. Discovery happens here so the TUI
// receives a fully resolved Config and never guesses at the environment.
func Interactive(e Env, typeOverride string) error {
	// Project discovery runs first: being in the wrong directory is the more
	// actionable error, and its message tells the user where to go.
	root, err := e.ProjectRoot()
	if err != nil {
		return err
	}
	if !tui.IsTerminal(e.Stdout) {
		return fmt.Errorf("not a terminal: use `deal-kit init`, `add` or `status` with --yes for non-interactive use")
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
		fmt.Fprintf(e.Stderr, "no package manager detected; install these yourself:\n  %s\n",
			strings.Join(pm.InstallArgs(pm.NPM, deps)[2:], " "))
		return nil
	}
	fmt.Fprintf(e.Stdout, "\ninstalling dependencies with %s\n", manager)
	return pm.Install(root, manager, deps, e.Stdout)
}
