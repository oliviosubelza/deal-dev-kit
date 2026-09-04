package cli

import (
	"fmt"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

// Update moves the project's kit pin forward and re-syncs what is installed.
// It never adds or removes artifacts: use `add` for that.
func Update(e Env) error {
	root, err := e.ProjectRoot()
	if err != nil {
		return err
	}
	lock, existed, err := lockfile.Load(root)
	if err != nil {
		return err
	}
	if !existed {
		return fmt.Errorf("este proyecto todavía no tiene %s: ejecutar `deal-kit init` primero", lockfile.Name)
	}

	ck, err := e.kit()
	if err != nil {
		return err
	}
	m, err := kit.LoadManifest(ck.Dir)
	if err != nil {
		return fmt.Errorf("no se pudo leer el kit: %w", err)
	}

	pt := kit.ProjectType(lock.ProjectType)
	if _, ok := m.ProjectTypes[pt]; !ok {
		return fmt.Errorf("%s registra un tipo de proyecto desconocido %q", lockfile.Name, lock.ProjectType)
	}

	ids, orphans := m.PartitionInstalled(lockIDs(lock))
	if len(ids) == 0 && len(orphans) == 0 {
		return fmt.Errorf("no hay nada instalado; ejecutar `deal-kit init` primero")
	}

	from := lock.KitVersion
	if from == "" {
		from = "(sin fijar)"
	}
	fmt.Fprintf(e.Stdout, "  kit        %s → %s\n\n", from, ck.Version)

	return syncArtifacts(e, root, ck, m, lock, pt, ids, orphans)
}
