package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deal/deal-dev-kit/tool/internal/kit"
	"github.com/deal/deal-dev-kit/tool/internal/lockfile"
)

var update = flag.Bool("update", false, "update golden files")

const manifestYAML = `
version: 1
project_types:
  web:
    match: crm-deal-web
    roots: { src: src, ui: src/shared/ui }
profiles:
  web: [web/ui]
artifacts:
  - { id: web/architecture, type: skill, applies_to: [web], src: skills/web/architecture }
  - { id: web/ui, type: skill, applies_to: [web], src: skills/web/ui }
  - id: ui-kit/base
    type: component
    applies_to: [web]
    src: ui-kit/lib
    dest: "{src}/shared/lib"
    npm: { clsx: ^2.1.1 }
  - id: ui-kit/button
    type: component
    applies_to: [web]
    src: ui-kit/button.tsx
    dest: "{ui}/button.tsx"
    requires: [ui-kit/base]
`

// testConfig builds a kit checkout on disk plus a matching Config.
func testConfig(t *testing.T, lock *lockfile.File) Config {
	t.Helper()
	kitDir := t.TempDir()
	files := map[string]string{
		"skills/web/architecture/SKILL.md": "---\nname: web-architecture\n---\n",
		"skills/web/ui/SKILL.md":           "---\nname: web-ui\n---\n",
		"ui-kit/lib/utils.ts":              "export const cn = 1\n",
		"ui-kit/button.tsx":                "export const Button = 1\n",
	}
	for rel, content := range files {
		abs := filepath.Join(kitDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m, err := kit.ParseManifest([]byte(manifestYAML))
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil {
		lock = &lockfile.File{Roots: map[string]string{}}
	}
	return Config{
		ProjectName: "crm-deal-web",
		ProjectType: kit.Web,
		ProjectRoot: t.TempDir(),
		KitDir:      kitDir,
		Manifest:    m,
		Lock:        lock,
		Roots:       map[string]string{"src": "src", "ui": "src/shared/ui"},
		PackageMgr:  "pnpm",
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// send applies a sequence of keys, executing any command each one returns so
// state that arrives via a message is reflected in the returned model.
func send(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	var model tea.Model = m
	for _, k := range keys {
		next, cmd := model.Update(key(k))
		model = next
		for cmd != nil {
			msg := cmd()
			if _, isQuit := msg.(tea.QuitMsg); isQuit || msg == nil {
				break
			}
			next, cmd = model.Update(msg)
			model = next
		}
	}
	return model.(Model)
}

func itemByID(m Model, id string) (item, bool) {
	for _, it := range m.items {
		if it.id == id {
			return it, true
		}
	}
	return item{}, false
}

func TestNewPreselectsInstalledArtifacts(t *testing.T) {
	lock := &lockfile.File{Roots: map[string]string{}}
	lock.Set(lockfile.Installed{ID: "web/ui", Files: []lockfile.OwnedFile{{Path: ".claude/skills/web-ui/SKILL.md", Hash: "x"}}})

	m := New(testConfig(t, lock))

	got, ok := itemByID(m, "web/ui")
	if !ok {
		t.Fatal("web/ui missing from the catalog")
	}
	if !got.installed || !got.selected() {
		t.Errorf("web/ui = %+v, want installed and selected", got)
	}
	if other, _ := itemByID(m, "web/architecture"); other.selected() {
		t.Error("web/architecture must not be selected when it is not installed")
	}
}

func TestCatalogExcludesOtherProjectTypes(t *testing.T) {
	cfg := testConfig(t, nil)
	cfg.ProjectType = kit.Backend
	cfg.Manifest.ProjectTypes[kit.Backend] = kit.ProjectTypeSpec{Match: "crm-deal-*-service"}

	m := New(cfg)
	if len(m.items) != 0 {
		t.Errorf("catalog = %d items, want 0 for a project type nothing applies to", len(m.items))
	}
}

func TestCursorStaysInBounds(t *testing.T) {
	m := New(testConfig(t, nil))

	m = send(t, m, "up", "up", "up")
	if m.cursor != 0 {
		t.Errorf("cursor = %d after moving up from the top, want 0", m.cursor)
	}

	for i := 0; i < len(m.items)+5; i++ {
		m = send(t, m, "down")
	}
	if want := len(m.items) - 1; m.cursor != want {
		t.Errorf("cursor = %d after moving past the end, want %d", m.cursor, want)
	}
}

func TestToggleSelection(t *testing.T) {
	m := New(testConfig(t, nil))
	if m.countSelected() != 0 {
		t.Fatalf("expected an empty selection, got %d", m.countSelected())
	}

	m = send(t, m, " ")
	if m.countSelected() == 0 {
		t.Error("space did not select the item under the cursor")
	}
	m = send(t, m, " ")
	if m.countSelected() != 0 {
		t.Error("space did not deselect")
	}
}

func TestSelectAllAndNone(t *testing.T) {
	m := New(testConfig(t, nil))

	m = send(t, m, "a")
	if m.countSelected() != len(m.items) {
		t.Errorf("selected %d of %d after 'a'", m.countSelected(), len(m.items))
	}
	m = send(t, m, "n")
	if m.countSelected() != 0 {
		t.Errorf("selected %d after 'n', want 0", m.countSelected())
	}
}

func TestSelectingAComponentMarksItsDependenciesRequired(t *testing.T) {
	m := New(testConfig(t, nil))

	// Move to ui-kit/button and select it; ui-kit/base is its dependency.
	for i, it := range m.items {
		if it.id == "ui-kit/button" {
			m.cursor = i
			break
		}
	}
	m = send(t, m, " ")

	base, ok := itemByID(m, "ui-kit/base")
	if !ok {
		t.Fatal("ui-kit/base missing")
	}
	if !base.selected() || !base.required {
		t.Errorf("ui-kit/base = %+v, want selected and required", base)
	}
}

func TestARequiredDependencyStaysInThePlan(t *testing.T) {
	m := New(testConfig(t, nil))
	for i, it := range m.items {
		if it.id == "ui-kit/button" {
			m.cursor = i
		}
	}
	m = send(t, m, " ")

	// Now try to turn off ui-kit/base, which button pulls in.
	for i, it := range m.items {
		if it.id == "ui-kit/base" {
			m.cursor = i
		}
	}
	m = send(t, m, " ")

	base, _ := itemByID(m, "ui-kit/base")
	if !base.selected() {
		t.Error("a required dependency left the plan, which would produce a broken install")
	}
}

func TestDeselectingTheDependentReleasesTheDependency(t *testing.T) {
	m := New(testConfig(t, nil))
	for i, it := range m.items {
		if it.id == "ui-kit/button" {
			m.cursor = i
		}
	}
	m = send(t, m, " ", " ") // select, then deselect

	base, _ := itemByID(m, "ui-kit/base")
	if base.selected() || base.required {
		t.Errorf("ui-kit/base = %+v, want released once nothing requires it", base)
	}
}

func TestEnterBuildsThePlanAndAdvances(t *testing.T) {
	m := New(testConfig(t, nil))
	m = send(t, m, "a", "enter")

	if m.stage != stagePlan {
		t.Fatalf("stage = %v, want stagePlan", m.stage)
	}
	if m.plan == nil {
		t.Fatal("plan is nil")
	}
	if len(m.plan.Changes()) == 0 {
		t.Error("a fresh project with everything selected must have changes")
	}
}

func TestEscapeReturnsToSelection(t *testing.T) {
	m := New(testConfig(t, nil))
	m = send(t, m, "a", "enter", "esc")

	if m.stage != stageSelect {
		t.Errorf("stage = %v, want stageSelect", m.stage)
	}
	if m.plan != nil {
		t.Error("the stale plan must be discarded when going back")
	}
}

func TestApplyWritesTheFilesAndTheLockfile(t *testing.T) {
	cfg := testConfig(t, nil)
	m := send(t, New(cfg), "a", "enter", "y")

	if m.stage != stageApplied {
		t.Fatalf("stage = %v (err: %v), want stageApplied", m.stage, m.err)
	}
	applied, deps := m.Result()
	if !applied {
		t.Error("Result() reports not applied")
	}
	if deps["clsx"] != "^2.1.1" {
		t.Errorf("deps = %v, want clsx", deps)
	}
	for _, rel := range []string{
		".claude/skills/web-ui/SKILL.md",
		"src/shared/lib/utils.ts",
		"src/shared/ui/button.tsx",
		lockfile.Name,
	} {
		if _, err := os.Stat(filepath.Join(cfg.ProjectRoot, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s was not written: %v", rel, err)
		}
	}
}

func TestPlanStageRefusesToApplyWhenBlocked(t *testing.T) {
	cfg := testConfig(t, nil)
	// A file the project already owns sits at a destination.
	dest := filepath.Join(cfg.ProjectRoot, "src", "shared", "lib", "utils.ts")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("// mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := send(t, New(cfg), "a", "enter")
	if len(m.plan.Blocked()) == 0 {
		t.Fatal("expected a blocked action")
	}

	m = send(t, m, "y")
	if m.stage == stageApplied {
		t.Error("apply must not run while something is blocked")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "// mine\n" {
		t.Errorf("the project's own file was overwritten: %q", got)
	}
}

func TestPlanErrorMovesToFailedStage(t *testing.T) {
	cfg := testConfig(t, nil)
	cfg.Roots = map[string]string{} // no roots: {src} cannot resolve

	m := send(t, New(cfg), "a", "enter")
	if m.stage != stageFailed {
		t.Fatalf("stage = %v, want stageFailed", m.stage)
	}
	if m.Err() == nil {
		t.Error("Err() is nil on a failed plan")
	}
}

func TestQuitFromAnyStage(t *testing.T) {
	for _, keys := range [][]string{{"q"}, {"a", "enter", "q"}} {
		m := send(t, New(testConfig(t, nil)), keys...)
		if !m.quitting {
			t.Errorf("keys %v did not quit", keys)
		}
	}
}

// --- golden rendering ---

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// assertGolden compares ANSI-stripped output, so the assertion is about layout
// and wording rather than the terminal's colour profile.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	got = ansi.ReplaceAllString(got, "")
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file (run with -update): %v", err)
	}
	if got != string(want) {
		t.Errorf("view mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestViewSelectStage(t *testing.T) {
	m := New(testConfig(t, nil))
	for i, it := range m.items {
		if it.id == "ui-kit/button" {
			m.cursor = i
		}
	}
	m = send(t, m, " ")
	assertGolden(t, "select", m.View())
}

func TestViewPlanStage(t *testing.T) {
	m := send(t, New(testConfig(t, nil)), "a", "enter")
	assertGolden(t, "plan", m.View())
}
