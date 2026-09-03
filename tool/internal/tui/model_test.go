package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
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

// onScreen puts the model on a screen the way the menu would, so tests start
// where the user would be rather than at the menu every time.
func onScreen(m Model, s screen) Model {
	m.screen, m.returnTo = s, screenMenu
	m.cursor, m.top = 0, 0
	return m
}

// expandAll opens every group so item rows are reachable.
func expandAll(m Model) Model {
	m = onScreen(m, screenComponents)
	for gi := range m.groups {
		m.groups[gi].collapsed = false
	}
	return m
}

// cursorTo moves the cursor onto the row for an artifact, expanding groups.
func cursorTo(t *testing.T, m Model, id string) Model {
	t.Helper()
	m = expandAll(m)
	// Skills live on their own screen.
	for _, it := range m.items {
		if it.id == id && it.kind == "skill" {
			m = onScreen(m, screenSkills)
		}
	}
	for i, r := range m.rows() {
		if !r.isHeading() && m.items[r.item].id == id {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("no row for %q", id)
	return m
}

// cursorToGroup moves the cursor onto a group heading.
func cursorToGroup(t *testing.T, m Model, name string) Model {
	t.Helper()
	for i, r := range m.rows() {
		if r.isHeading() && m.groups[r.group].name == name {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("no heading for %q", name)
	return m
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
	m := expandAll(New(testConfig(t, nil)))

	m = send(t, m, "up", "up", "up")
	if m.cursor != 0 {
		t.Errorf("cursor = %d after moving up from the top, want 0", m.cursor)
	}

	rows := len(m.rows())
	for i := 0; i < rows+5; i++ {
		m = send(t, m, "down")
	}
	if want := rows - 1; m.cursor != want {
		t.Errorf("cursor = %d after moving past the end, want %d", m.cursor, want)
	}
}

func TestToggleSelection(t *testing.T) {
	m := cursorTo(t, New(testConfig(t, nil)), "web/ui")
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

func TestSelectAllAppliesToTheCurrentScreenOnly(t *testing.T) {
	// "all" on the components screen must not silently change which team
	// conventions the project follows.
	m := expandAll(New(testConfig(t, nil)))
	m = send(t, m, "a")

	for _, it := range m.items {
		if it.kind == "skill" && it.selected() {
			t.Errorf("selecting all components also selected the skill %q", it.id)
		}
		if it.kind == "component" && !it.selected() {
			t.Errorf("component %q was not selected", it.id)
		}
	}

	m = send(t, m, "n")
	if m.selectedHere() != 0 {
		t.Errorf("%d still selected after 'n'", m.selectedHere())
	}
}

func TestMenuNavigatesToAScreen(t *testing.T) {
	m := New(testConfig(t, nil))
	if m.screen != screenMenu {
		t.Fatal("the browser must open on the menu")
	}
	m = send(t, m, "down", "enter") // second entry: skills
	if m.screen != screenSkills {
		t.Errorf("screen = %v, want screenSkills", m.screen)
	}
	m = send(t, m, "esc")
	if m.screen != screenMenu {
		t.Errorf("esc did not return to the menu")
	}
	m = send(t, m, "down", "enter")
	if m.screen != screenComponents {
		t.Errorf("screen = %v, want screenComponents", m.screen)
	}
}

func TestInstallEverythingSelectsAllAndGoesStraightToReview(t *testing.T) {
	// The first menu entry is the one most people will use, so it must not
	// require walking two screens and pressing "a" on each.
	m := send(t, New(testConfig(t, nil)), "enter")

	if m.screen != screenPlan {
		t.Fatalf("screen = %v, want screenPlan (err: %v)", m.screen, m.err)
	}
	for _, it := range m.items {
		if !it.selected() {
			t.Errorf("%q was left out of \"install everything\"", it.id)
		}
	}
	if len(m.plan.Changes()) == 0 {
		t.Error("the plan is empty for a project with nothing installed")
	}
}

func TestDependencyLabelNamesWhatPullsItIn(t *testing.T) {
	m := cursorTo(t, New(testConfig(t, nil)), "ui-kit/button")
	m = send(t, m, " ")

	base, _ := itemByID(m, "ui-kit/base")
	if !base.required {
		t.Fatal("ui-kit/base should be required by ui-kit/button")
	}
	if len(base.pulledBy) != 1 || base.pulledBy[0] != "ui-kit/button" {
		t.Errorf("pulledBy = %v, want [ui-kit/button]", base.pulledBy)
	}
	if got := pulledByLabel(base.pulledBy, 18); got != "← button" {
		t.Errorf("label = %q, want %q", got, "← button")
	}
}

func TestDependencyLabelWithSeveralSources(t *testing.T) {
	tests := []struct {
		name  string
		by    []string
		width int
		want  string
	}{
		{"one", []string{"ui-kit/button"}, 18, "← button"},
		{"two", []string{"ui-kit/button", "ui-kit/card"}, 18, "← button +1"},
		{"too narrow to name", []string{"ui-kit/dropdown-menu", "ui-kit/card"}, 12, "← 2 que lo usan"},
		{"none", nil, 18, "requerido"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pulledByLabel(tt.by, tt.width); got != tt.want {
				t.Errorf("pulledByLabel(%v) = %q, want %q", tt.by, got, tt.want)
			}
		})
	}
}

func TestSkillsAndComponentsAreSeparateLists(t *testing.T) {
	m := onScreen(New(testConfig(t, nil)), screenSkills)
	for _, r := range m.rows() {
		if r.isHeading() {
			t.Error("the skills screen must be a flat list, not a tree")
		}
		if m.items[r.item].kind != "skill" {
			t.Errorf("%q is a %s on the skills screen", m.items[r.item].id, m.items[r.item].kind)
		}
	}

	m = expandAll(m)
	for _, r := range m.rows() {
		if r.isHeading() {
			continue
		}
		if m.items[r.item].kind != "component" {
			t.Errorf("%q is a %s on the components screen", m.items[r.item].id, m.items[r.item].kind)
		}
	}
}

func TestSelectingAComponentMarksItsDependenciesRequired(t *testing.T) {
	m := New(testConfig(t, nil))

	m = cursorTo(t, m, "ui-kit/button")
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
	m := cursorTo(t, New(testConfig(t, nil)), "ui-kit/button")
	m = send(t, m, " ")

	// Now try to turn off ui-kit/base, which button pulls in.
	m = cursorTo(t, m, "ui-kit/base")
	m = send(t, m, " ")

	base, _ := itemByID(m, "ui-kit/base")
	if !base.selected() {
		t.Error("a required dependency left the plan, which would produce a broken install")
	}
}

func TestDeselectingTheDependentReleasesTheDependency(t *testing.T) {
	m := cursorTo(t, New(testConfig(t, nil)), "ui-kit/button")
	m = send(t, m, " ", " ") // select, then deselect

	base, _ := itemByID(m, "ui-kit/base")
	if base.selected() || base.required {
		t.Errorf("ui-kit/base = %+v, want released once nothing requires it", base)
	}
}

func TestPlanKeyBuildsThePlanAndAdvances(t *testing.T) {
	m := expandAll(New(testConfig(t, nil)))
	m = send(t, m, "a", "p")

	if m.screen != screenPlan {
		t.Fatalf("stage = %v, want screenPlan", m.screen)
	}
	if m.plan == nil {
		t.Fatal("plan is nil")
	}
	if len(m.plan.Changes()) == 0 {
		t.Error("a fresh project with everything selected must have changes")
	}
}

func TestEscapeReturnsToSelection(t *testing.T) {
	m := expandAll(New(testConfig(t, nil)))
	m = send(t, m, "a", "p", "esc")

	if m.screen != screenComponents {
		t.Errorf("stage = %v, want screenComponents", m.screen)
	}
	if m.plan != nil {
		t.Error("the stale plan must be discarded when going back")
	}
}

func TestApplyWritesTheFilesAndTheLockfile(t *testing.T) {
	cfg := testConfig(t, nil)
	// Select on both screens: "all" is deliberately per-screen.
	m := send(t, onScreen(New(cfg), screenSkills), "a")
	m = send(t, expandAll(m), "a", "p", "y")

	if m.screen != screenApplied {
		t.Fatalf("stage = %v (err: %v), want screenApplied", m.screen, m.err)
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

	m := send(t, expandAll(New(cfg)), "a", "p")
	if len(m.plan.Blocked()) == 0 {
		t.Fatal("expected a blocked action")
	}

	m = send(t, m, "y")
	if m.screen == screenApplied {
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

	m := send(t, expandAll(New(cfg)), "a", "p")
	if m.screen != screenFailed {
		t.Fatalf("stage = %v, want screenFailed", m.screen)
	}
	if m.Err() == nil {
		t.Error("Err() is nil on a failed plan")
	}
}

func TestQuitFromAnyStage(t *testing.T) {
	for _, keys := range [][]string{{"q"}, {"a", "p", "q"}} {
		m := send(t, expandAll(New(testConfig(t, nil))), keys...)
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
	m := cursorTo(t, New(testConfig(t, nil)), "ui-kit/button")
	m = send(t, m, " ")
	assertGolden(t, "select", m.View())
}

func TestViewMenu(t *testing.T) {
	// The browser opens on a menu, so skills and components are never mixed
	// into one list the user cannot interpret.
	assertGolden(t, "menu", New(testConfig(t, nil)).View())
}

func TestTabFoldsAndUnfolds(t *testing.T) {
	m := expandAll(New(testConfig(t, nil)))
	for gi := range m.groups {
		m.groups[gi].collapsed = true
	}
	if !m.groups[0].collapsed {
		t.Fatal("a fresh project should open folded")
	}
	m = send(t, m, "tab")
	if m.groups[0].collapsed {
		t.Error("tab did not unfold")
	}
	m = send(t, m, "tab")
	if !m.groups[0].collapsed {
		t.Error("tab did not fold again")
	}
}

func TestGroupHeadingTogglesEveryItem(t *testing.T) {
	m := cursorToGroup(t, expandAll(New(testConfig(t, nil))), "ui-kit")
	m = send(t, m, " ")

	g := m.groups[m.rows()[m.cursor].group]
	if got := m.groupState(g); got != checked {
		t.Errorf("group state = %v, want checked after toggling the heading", got)
	}
	m = send(t, m, " ")
	if got := m.groupState(m.groups[0]); got != unchecked {
		t.Errorf("group state = %v, want unchecked after toggling again", got)
	}
}

func TestGroupShowsPartialState(t *testing.T) {
	m := cursorTo(t, New(testConfig(t, nil)), "ui-kit/base")
	m = send(t, m, " ")

	for _, g := range m.groups {
		if g.name != "ui-kit" {
			continue
		}
		if got := m.groupState(g); got != partial {
			t.Errorf("group state = %v, want partial with one of two selected", got)
		}
	}
}

func TestFilterHidesNonMatchingGroups(t *testing.T) {
	m := send(t, expandAll(New(testConfig(t, nil))), "/", "b", "u", "t")

	for _, r := range m.rows() {
		if r.isHeading() {
			continue
		}
		if id := m.items[r.item].id; id != "ui-kit/button" {
			t.Errorf("filter %q left %q visible", m.filter, id)
		}
	}
	// Escaping restores the full tree.
	m = send(t, m, "esc")
	if m.filter != "" {
		t.Error("esc did not clear the filter")
	}
}

func TestFilterTypingDoesNotTriggerCommands(t *testing.T) {
	// "a" means select-all outside the filter, and a literal "a" inside it.
	m := send(t, expandAll(New(testConfig(t, nil))), "/", "a")
	if m.countSelected() != 0 {
		t.Errorf("typing in the filter selected %d items", m.countSelected())
	}
	if m.filter != "a" {
		t.Errorf("filter = %q, want %q", m.filter, "a")
	}
}

func TestViewFilter(t *testing.T) {
	m := New(testConfig(t, nil))
	m = send(t, m, "/", "b", "u", "t")
	assertGolden(t, "filter", m.View())
}

func TestViewPlanStage(t *testing.T) {
	m := send(t, expandAll(New(testConfig(t, nil))), "a", "p")
	assertGolden(t, "plan", m.View())
}

func TestEnterNeverApplies(t *testing.T) {
	// Enter navigates into the plan screen, so accepting it as confirmation
	// lets a single repeated keypress install everything. On Windows the Enter
	// that launched the process arrives as input and did exactly that.
	cfg := testConfig(t, nil)
	m := send(t, New(cfg), "enter") // menu: "Instalar todo"
	if m.screen != screenPlan {
		t.Fatalf("screen = %v, want screenPlan", m.screen)
	}

	m = send(t, m, "enter")
	if m.screen == screenApplied {
		t.Fatal("a second Enter applied the plan")
	}
	if _, err := os.Stat(filepath.Join(cfg.ProjectRoot, lockfile.Name)); err == nil {
		t.Error("Enter wrote the lockfile")
	}
	entries, _ := os.ReadDir(cfg.ProjectRoot)
	if len(entries) != 0 {
		t.Errorf("Enter wrote %d entries into the project", len(entries))
	}
}

func TestOnlyYApplies(t *testing.T) {
	cfg := testConfig(t, nil)
	m := send(t, New(cfg), "enter", "y")

	if m.screen != screenApplied {
		t.Fatalf("screen = %v (err: %v), want screenApplied", m.screen, m.err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ProjectRoot, lockfile.Name)); err != nil {
		t.Errorf("the lockfile was not written: %v", err)
	}
}

func TestEnterOnThePlanReturnsToWhereItCameFrom(t *testing.T) {
	m := send(t, expandAll(New(testConfig(t, nil))), "a", "p")
	if m.screen != screenPlan {
		t.Fatalf("screen = %v, want screenPlan", m.screen)
	}
	m = send(t, m, "enter")
	if m.screen != screenComponents {
		t.Errorf("screen = %v, want to return to the component list", m.screen)
	}
}
