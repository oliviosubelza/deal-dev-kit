package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

// TestRealCatalogIsNavigable renders this repository's actual catalog. A flat
// list of 61 components is what prompted the grouping, so this asserts the
// browser opens small and every group is reachable and named.
func TestRealCatalogIsNavigable(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	m, err := kit.LoadManifest(root)
	if err != nil {
		t.Fatalf("repository kit.yaml: %v", err)
	}

	model := New(Config{
		ProjectName: "crm-deal-web",
		ProjectType: kit.Web,
		ProjectRoot: t.TempDir(),
		KitVersion:  "kit-v0.1.0",
		Manifest:    m,
		Lock:        &lockfile.File{Roots: map[string]string{}},
		Roots:       map[string]string{"src": "src", "ui": "src/shared/ui", "hooks": "src/shared/hooks"},
		PackageMgr:  "pnpm",
	})
	next, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	model = next.(Model)

	if len(model.items) < 50 {
		t.Fatalf("only %d artifacts apply to web; expected the full catalog", len(model.items))
	}

	// The menu must stay short enough to take in at a glance.
	if n := len(model.menu()); n > 6 {
		t.Errorf("the menu has %d entries; keep it scannable", n)
	}

	// Folded, the whole component catalog must fit on one screen.
	model = onScreen(model, screenComponents)
	rows := model.rows()
	if len(rows) != len(model.groups) {
		t.Errorf("opened with %d rows for %d groups; it should open folded", len(rows), len(model.groups))
	}
	if len(rows) > 12 {
		t.Errorf("%d groups is too many to scan at a glance", len(rows))
	}

	for _, g := range model.groups {
		if g.name == "" {
			t.Error("a group has no name")
		}
		if len(g.items) == 0 {
			t.Errorf("group %q is empty", g.name)
		}
	}

	// No screen may overflow the terminal width, or the panel border breaks.
	for _, s := range []screen{screenMenu, screenSkills, screenComponents, screenStatus, screenEngram} {
		view := ansi.ReplaceAllString(onScreen(model, s).View(), "")
		for _, line := range strings.Split(view, "\n") {
			if len([]rune(line)) > 80 {
				t.Errorf("screen %v: line is %d columns wide, over the 80 available:\n%s",
					s, len([]rune(line)), line)
			}
		}
		t.Logf("\n%s", view)
	}
}
