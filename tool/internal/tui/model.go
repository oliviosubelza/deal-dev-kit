// Package tui is the interactive front-end for deal-kit. It renders and edits
// a selection, then hands it to internal/plan — the same engine the
// non-interactive flags use. The TUI decides nothing about what a sync does.
package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// screen is which view is on top. Skills and components live on separate
// screens on purpose: mixing a team convention and a Button in one list makes
// it impossible to tell what you are about to install.
type screen int

const (
	screenMenu screen = iota
	screenSkills
	screenComponents
	screenStatus
	screenPlan
	screenApplied
	screenFailed
)

// Config is everything the model needs, resolved before the program starts so
// the TUI never does discovery of its own.
type Config struct {
	ProjectName string
	ProjectType kit.ProjectType
	ProjectRoot string
	KitDir      string
	KitVersion  string // the version available in the kit checkout
	PinnedKit   string // what the project's lockfile pins, if anything
	CLIVersion  string
	Manifest    *kit.Manifest
	Lock        *lockfile.File
	Roots       map[string]string
	Rewrites    map[string]string
	PackageMgr  string
}

// item is one selectable artifact.
//
// explicit and required are tracked separately on purpose: a dependency that
// was pulled in must be released again once nothing requires it, which is
// impossible if "the user chose this" and "something needs this" share a flag.
type item struct {
	id        string
	kind      string // "skill" or "component"
	group     string
	installed bool
	explicit  bool     // the user chose it directly
	required  bool     // pulled in by another selection
	pulledBy  []string // which explicit selections require it, for the label
}

func (i item) selected() bool { return i.explicit || i.required }

// label drops the group prefix the heading already carries.
func (i item) label() string {
	if _, rest, ok := strings.Cut(i.id, "/"); ok {
		return rest
	}
	return i.id
}

// menuEntry is one line on the main menu.
type menuEntry struct {
	title      string
	note       string
	target     screen
	quit       bool
	installAll bool
}

// Model is the Bubble Tea model.
type Model struct {
	cfg    Config
	items  []item
	groups []group // component groups only
	skills []int   // indices into items

	screen     screen
	returnTo   screen // where Back and the plan screen go
	menuCursor int

	cursor    int
	top       int
	height    int
	width     int
	filter    string
	filtering bool

	plan        *plan.Plan
	err         error
	changed     int
	appliedRows []plan.DirSummary // summarised before the plan is released

	quitting bool
}

// New builds a model with the project's currently installed artifacts
// pre-selected.
func New(cfg Config) Model {
	var items []item
	for _, a := range cfg.Manifest.Artifacts {
		if !a.Supports(cfg.ProjectType) {
			continue
		}
		_, installed := cfg.Lock.Artifact(a.ID)
		items = append(items, item{
			id: a.ID, kind: a.Type, group: a.Group,
			installed: installed, explicit: installed,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })

	m := Model{cfg: cfg, items: items, height: 14, width: 80, screen: screenMenu}

	var components []item
	for i, it := range items {
		if it.kind == "skill" {
			m.skills = append(m.skills, i)
		} else {
			components = append(components, it)
		}
	}
	m.groups = buildGroups(items, components)
	// Always open folded: expanding what is installed reproduces the
	// unreadable flat list that grouping exists to prevent.
	for gi := range m.groups {
		m.groups[gi].collapsed = true
	}
	m.markRequired()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// menu builds the main menu, with a live summary on each line so the state of
// the project is visible before choosing anything.
func (m Model) menu() []menuEntry {
	skillsInstalled := 0
	for _, i := range m.skills {
		if m.items[i].installed {
			skillsInstalled++
		}
	}
	compTotal, compInstalled := 0, 0
	for _, it := range m.items {
		if it.kind == "skill" {
			continue
		}
		compTotal++
		if it.installed {
			compInstalled++
		}
	}

	total := len(m.items)
	pending := total - skillsInstalled - compInstalled

	all := menuEntry{title: "Instalar todo", installAll: true,
		note: itoa(total) + " artefactos · nada pendiente"}
	if pending > 0 {
		all.note = itoa(total) + " artefactos · " + itoa(pending) + " por instalar"
	}

	entries := []menuEntry{
		all,
		{title: "Skills y convenciones", target: screenSkills,
			note: instaladas(skillsInstalled, len(m.skills), "instaladas")},
		{title: "Componentes de UI", target: screenComponents,
			note: instaladas(compInstalled, compTotal, "instalados")},
		{title: "Estado del proyecto", target: screenStatus,
			note: "qué hay instalado y si cambió"},
	}
	if m.updateAvailable() {
		entries = append(entries, menuEntry{
			title: "Actualizar el kit", target: screenStatus,
			note: m.cfg.PinnedKit + " → " + m.cfg.KitVersion})
	}
	return append(entries, menuEntry{title: "Salir", quit: true})
}

// updateAvailable reports whether the kit checkout is newer than the pin.
func (m Model) updateAvailable() bool {
	return m.cfg.PinnedKit != "" && m.cfg.KitVersion != "" &&
		m.cfg.PinnedKit != m.cfg.KitVersion
}

func instaladas(have, total int, word string) string {
	switch {
	case total == 0:
		return "no hay para este proyecto"
	case have == total:
		return "las " + itoa(total) + " " + word
	}
	// "0 de 61 instalados" avoids agreeing a pronoun with a noun the caller
	// chose, which is how "ninguna de 61 componentes" happens.
	return itoa(have) + " de " + itoa(total) + " " + word
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// matches reports whether an item passes the active filter.
func (m Model) matches(i int) bool {
	if m.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(m.items[i].id), strings.ToLower(m.filter))
}

// Update handles one message. All state transitions live here, so they can be
// tested without a terminal.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		if h := msg.Height - 12; h > 3 {
			m.height = h
		}
		return m, nil

	case planBuiltMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenFailed
			return m, nil
		}
		m.plan, m.screen = msg.plan, screenPlan
		return m, nil

	case appliedMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenFailed
			return m, nil
		}
		m.changed, m.screen = msg.changed, screenApplied
		if m.plan != nil {
			// Keep the data, not rendered text: rendering here would bake in
			// whatever config was current at apply time.
			m.appliedRows = plan.Summarize(m.plan.Actions)
		}
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.filtering {
		return m.handleFilterKey(msg, key)
	}
	if key == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.screen {
	case screenMenu:
		return m.handleMenuKey(key)
	case screenSkills, screenComponents:
		return m.handleListKey(key)
	case screenStatus:
		switch key {
		case "esc", "q", "enter", "left", "h":
			m.screen = screenMenu
		case "u":
			return m, m.buildPlanFor(m.installedIDs())
		}
	case screenPlan:
		switch key {
		// Only "y" writes. Enter must never commit changes: it is the key that
		// navigates INTO this screen, and on Windows the Enter that launched
		// the process leaks into it, which walked menu → plan → apply and
		// installed everything without a decision.
		case "y":
			if len(m.plan.Blocked()) > 0 || len(m.plan.Changes()) == 0 {
				m.screen, m.plan = m.returnTo, nil
				return m, nil
			}
			return m, m.apply()
		case "enter", "esc", "left", "h", "n":
			m.screen, m.plan = m.returnTo, nil
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
	case screenApplied, screenFailed:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.filtering, m.filter = false, ""
	case "enter":
		m.filtering = false
	case "backspace":
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
		}
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	default:
		if len(msg.Runes) > 0 {
			m.filter += string(msg.Runes)
		}
	}
	m.clampCursor()
	return m, nil
}

func (m Model) handleMenuKey(key string) (tea.Model, tea.Cmd) {
	entries := m.menu()
	switch key {
	case "up", "k":
		m.menuCursor--
	case "down", "j":
		m.menuCursor++
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "enter", "right", "l", " ":
		e := entries[clamp(m.menuCursor, 0, len(entries)-1)]
		switch {
		case e.quit:
			m.quitting = true
			return m, tea.Quit
		case e.installAll:
			for i := range m.items {
				m.items[i].explicit = true
			}
			m.markRequired()
			m.returnTo = screenMenu
			return m, m.buildPlan()
		}
		m.screen, m.returnTo = e.target, screenMenu
		m.cursor, m.top, m.filter = 0, 0, ""
	}
	m.menuCursor = clamp(m.menuCursor, 0, len(entries)-1)
	return m, nil
}

func (m Model) handleListKey(key string) (tea.Model, tea.Cmd) {
	rows := m.rows()

	switch key {
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup", "ctrl+u":
		m.cursor -= m.height
	case "pgdown", "ctrl+d":
		m.cursor += m.height
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(rows) - 1

	case " ", "x":
		m.toggleRow(rows)
	case "right", "l":
		m.setCollapsed(rows, false)
	case "tab":
		m.toggleAllCollapsed()
	case "a":
		m.setAll(true)
	case "n":
		m.setAll(false)
	case "/":
		m.filtering = true

	case "left", "h":
		// Left folds an open group, and otherwise steps back to the menu.
		if m.screen == screenComponents && len(rows) > 0 && m.cursor < len(rows) {
			g := m.groups[rows[m.cursor].group]
			if !g.collapsed {
				m.setCollapsed(rows, true)
				break
			}
		}
		m.screen = screenMenu
	case "esc":
		if m.filter != "" {
			m.filter = ""
			break
		}
		m.screen = screenMenu
	case "q":
		m.quitting = true
		return m, tea.Quit

	case "enter":
		if m.screen == screenComponents && len(rows) > 0 && m.cursor < len(rows) && rows[m.cursor].isHeading() {
			gi := rows[m.cursor].group
			m.groups[gi].collapsed = !m.groups[gi].collapsed
			m.clampCursor()
			return m, nil
		}
		m.returnTo = m.screen
		return m, m.buildPlan()
	case "p":
		m.returnTo = m.screen
		return m, m.buildPlan()
	}

	m.clampCursor()
	return m, nil
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampCursor keeps the cursor inside the visible rows and scrolls to follow.
func (m *Model) clampCursor() {
	n := len(m.rows())
	if n == 0 {
		m.cursor, m.top = 0, 0
		return
	}
	m.cursor = clamp(m.cursor, 0, n-1)
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+m.height {
		m.top = m.cursor - m.height + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

// toggleRow toggles an artifact, or every artifact under a heading.
func (m *Model) toggleRow(rows []row) {
	if len(rows) == 0 || m.cursor >= len(rows) {
		return
	}
	r := rows[m.cursor]
	if !r.isHeading() {
		m.items[r.item].explicit = !m.items[r.item].explicit
		m.markRequired()
		return
	}
	g := m.groups[r.group]
	want := m.groupState(g) != checked
	for _, i := range g.items {
		if m.matches(i) {
			m.items[i].explicit = want
		}
	}
	m.markRequired()
}

func (m *Model) setCollapsed(rows []row, collapsed bool) {
	if len(rows) == 0 || m.cursor >= len(rows) || m.screen != screenComponents {
		return
	}
	m.groups[rows[m.cursor].group].collapsed = collapsed
	m.clampCursor()
}

func (m *Model) toggleAllCollapsed() {
	anyOpen := false
	for _, g := range m.groups {
		if !g.collapsed {
			anyOpen = true
			break
		}
	}
	for gi := range m.groups {
		m.groups[gi].collapsed = anyOpen
	}
	m.clampCursor()
}

// setAll applies to the current screen only, so "select all" on the components
// screen cannot silently change which conventions the project follows.
func (m *Model) setAll(v bool) {
	for _, r := range m.rows() {
		if !r.isHeading() {
			m.items[r.item].explicit = v
		}
	}
	m.markRequired()
}

// markRequired recomputes which items are only present as dependencies of
// another selection, and selects them so the plan is always consistent.
func (m *Model) markRequired() {
	directly := map[string]bool{}
	var ids []string
	for _, it := range m.items {
		if it.explicit {
			directly[it.id] = true
			ids = append(ids, it.id)
		}
	}
	sort.Strings(ids)

	// Resolve each selection on its own, so a dependency can name what pulled
	// it in. "dependency" with no source leaves the user unable to act on it.
	pulledBy := map[string][]string{}
	for _, id := range ids {
		deps, err := m.cfg.Manifest.Resolve(m.cfg.ProjectType, []string{id})
		if err != nil {
			continue
		}
		for _, d := range deps {
			if d.ID == id || directly[d.ID] {
				continue
			}
			pulledBy[d.ID] = appendUnique(pulledBy[d.ID], id)
		}
	}

	for i := range m.items {
		by := pulledBy[m.items[i].id]
		sort.Strings(by)
		m.items[i].required = len(by) > 0
		m.items[i].pulledBy = by
	}
}

func appendUnique(ss []string, s string) []string {
	for _, existing := range ss {
		if existing == s {
			return ss
		}
	}
	return append(ss, s)
}

// SelectedIDs returns every artifact that will be part of the plan.
func (m Model) SelectedIDs() []string {
	var out []string
	for _, it := range m.items {
		if it.selected() {
			out = append(out, it.id)
		}
	}
	return out
}

func (m Model) installedIDs() []string {
	var out []string
	for _, in := range m.cfg.Lock.Artifacts {
		out = append(out, in.ID)
	}
	return out
}

// Result reports what the session did, for the caller to finish up.
func (m Model) Result() (applied bool, deps map[string]string) {
	if m.screen != screenApplied || m.plan == nil {
		return false, nil
	}
	return true, m.plan.Deps
}

// Err returns a failure from planning or applying.
func (m Model) Err() error { return m.err }

type planBuiltMsg struct {
	plan *plan.Plan
	err  error
}

type appliedMsg struct {
	changed int
	err     error
}

func (m Model) buildPlan() tea.Cmd { return m.buildPlanFor(m.SelectedIDs()) }

func (m Model) buildPlanFor(ids []string) tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		// The lockfile can name an artifact the manifest no longer declares.
		// Resolve would reject it, so it is split off here and removed by the
		// plan instead — the same split the flags make, since the TUI must
		// never decide anything about a sync on its own.
		selected, _ := cfg.Manifest.PartitionInstalled(ids)
		_, orphans := cfg.Manifest.PartitionInstalled(lockIDs(cfg.Lock))
		artifacts, err := cfg.Manifest.Resolve(cfg.ProjectType, selected)
		if err != nil {
			return planBuiltMsg{err: err}
		}
		p, err := plan.Build(plan.Input{
			Artifacts: artifacts, Orphans: orphans, Lock: cfg.Lock,
			KitDir: cfg.KitDir, ProjectDir: cfg.ProjectRoot, Roots: cfg.Roots,
			Rewrites: cfg.Rewrites,
		})
		return planBuiltMsg{plan: p, err: err}
	}
}

// lockIDs is every artifact id the project's lockfile records.
func lockIDs(lock *lockfile.File) []string {
	out := make([]string, 0, len(lock.Artifacts))
	for _, a := range lock.Artifacts {
		out = append(out, a.ID)
	}
	return out
}

func (m Model) apply() tea.Cmd {
	cfg, p := m.cfg, m.plan
	return func() tea.Msg {
		changed := len(p.Changes())
		keep := map[string]bool{}
		for _, a := range p.Actions {
			keep[a.ArtifactID] = true
		}
		if err := p.Apply(cfg.ProjectRoot, cfg.Lock); err != nil {
			return appliedMsg{err: err}
		}
		// Artifacts the user deselected leave the lockfile; their files stay,
		// now owned by the project.
		var drop []string
		for _, in := range cfg.Lock.Artifacts {
			if !keep[in.ID] {
				drop = append(drop, in.ID)
			}
		}
		for _, id := range drop {
			cfg.Lock.Remove(id)
		}
		if cfg.KitVersion != "" && cfg.KitVersion != "local" {
			cfg.Lock.KitVersion = cfg.KitVersion
		}
		if err := cfg.Lock.Save(cfg.ProjectRoot); err != nil {
			return appliedMsg{err: err}
		}
		return appliedMsg{changed: changed}
	}
}

func (m Model) countSelected() int {
	n := 0
	for _, it := range m.items {
		if it.selected() {
			n++
		}
	}
	return n
}
