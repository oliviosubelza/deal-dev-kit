// Package tui is the interactive front-end for deal-kit. It renders and edits
// a selection, then hands it to internal/plan — the same engine the
// non-interactive flags use. The TUI decides nothing about what a sync does.
package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// stage is which screen the user is on.
type stage int

const (
	stageSelect stage = iota
	stagePlan
	stageApplied
	stageFailed
)

// Config is everything the model needs, resolved before the program starts so
// the TUI never does discovery of its own.
type Config struct {
	ProjectName string
	ProjectType kit.ProjectType
	ProjectRoot string
	KitDir      string
	KitVersion  string
	Manifest    *kit.Manifest
	Lock        *lockfile.File
	Roots       map[string]string
	Rewrites    map[string]string // import prefixes to rewrite on install
	PackageMgr  string            // empty when none was detected
}

// item is one selectable artifact.
//
// explicit and required are tracked separately on purpose: a dependency that
// was pulled in must be released again once nothing requires it, which is
// impossible if "the user chose this" and "something needs this" share a flag.
type item struct {
	id        string
	kind      string // "skill" or "component"
	installed bool
	explicit  bool // the user chose it directly
	required  bool // pulled in by another selection
}

// selected reports whether the artifact will be part of the plan.
func (i item) selected() bool { return i.explicit || i.required }

// Model is the Bubble Tea model.
type Model struct {
	cfg    Config
	items  []item
	cursor int
	stage  stage

	plan    *plan.Plan
	err     error
	changed int

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
			id: a.ID, kind: a.Type,
			installed: installed, explicit: installed,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })

	m := Model{cfg: cfg, items: items}
	m.markRequired()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// Update handles one message. All state transitions live here, so they can be
// tested without a terminal.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case planBuiltMsg:
		if msg.err != nil {
			m.err, m.stage = msg.err, stageFailed
			return m, nil
		}
		m.plan, m.stage = msg.plan, stagePlan
		return m, nil

	case appliedMsg:
		if msg.err != nil {
			m.err, m.stage = msg.err, stageFailed
			return m, nil
		}
		m.changed, m.stage = msg.changed, stageApplied
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.stage {
	case stageSelect:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ", "x":
			m.toggle(m.cursor)
		case "a":
			m.setAll(true)
		case "n":
			m.setAll(false)
		case "enter":
			return m, m.buildPlan()
		}

	case stagePlan:
		switch key {
		case "y", "enter":
			if len(m.plan.Blocked()) > 0 || len(m.plan.Changes()) == 0 {
				m.quitting = true
				return m, tea.Quit
			}
			return m, m.apply()
		case "esc", "left", "h":
			m.stage, m.plan = stageSelect, nil
		}

	case stageApplied, stageFailed:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// toggle flips the user's own choice. An item another selection requires stays
// in the plan regardless: to drop it, deselect whatever pulls it in.
func (m *Model) toggle(i int) {
	if i < 0 || i >= len(m.items) {
		return
	}
	m.items[i].explicit = !m.items[i].explicit
	m.markRequired()
}

func (m *Model) setAll(v bool) {
	for i := range m.items {
		m.items[i].explicit = v
	}
	m.markRequired()
}

// markRequired recomputes which items are only present as dependencies of
// another selection, and selects them so the plan is always consistent.
func (m *Model) markRequired() {
	directly := map[string]bool{}
	for _, it := range m.items {
		if it.explicit {
			directly[it.id] = true
		}
	}

	required := map[string]bool{}
	var ids []string
	for id := range directly {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if resolved, err := m.cfg.Manifest.Resolve(m.cfg.ProjectType, ids); err == nil {
		for _, a := range resolved {
			if !directly[a.ID] {
				required[a.ID] = true
			}
		}
	}

	for i := range m.items {
		m.items[i].required = required[m.items[i].id]
	}
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

// Result reports what the session did, for the caller to finish up (installing
// npm dependencies with the project's own package manager, where its streamed
// output belongs in the normal terminal rather than inside a TUI).
func (m Model) Result() (applied bool, deps map[string]string) {
	if m.stage != stageApplied || m.plan == nil {
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

func (m Model) buildPlan() tea.Cmd {
	cfg, ids := m.cfg, m.SelectedIDs()
	return func() tea.Msg {
		artifacts, err := cfg.Manifest.Resolve(cfg.ProjectType, ids)
		if err != nil {
			return planBuiltMsg{err: err}
		}
		p, err := plan.Build(plan.Input{
			Artifacts: artifacts, Lock: cfg.Lock,
			KitDir: cfg.KitDir, ProjectDir: cfg.ProjectRoot, Roots: cfg.Roots,
			Rewrites: cfg.Rewrites,
		})
		return planBuiltMsg{plan: p, err: err}
	}
}

func (m Model) apply() tea.Cmd {
	cfg, p := m.cfg, m.plan
	return func() tea.Msg {
		changed := len(p.Changes())
		// Artifacts the user deselected are no longer in the plan, so drop
		// their records; the files themselves stay, now owned by the project.
		keep := map[string]bool{}
		for _, a := range p.Actions {
			keep[a.ArtifactID] = true
		}
		if err := p.Apply(cfg.ProjectRoot, cfg.Lock); err != nil {
			return appliedMsg{err: err}
		}
		var drop []string
		for _, in := range cfg.Lock.Artifacts {
			if !keep[in.ID] {
				drop = append(drop, in.ID)
			}
		}
		for _, id := range drop {
			cfg.Lock.Remove(id)
		}
		if err := cfg.Lock.Save(cfg.ProjectRoot); err != nil {
			return appliedMsg{err: err}
		}
		return appliedMsg{changed: changed}
	}
}

// countSelected is used by the view and by tests.
func (m Model) countSelected() int {
	n := 0
	for _, it := range m.items {
		if it.selected() {
			n++
		}
	}
	return n
}
