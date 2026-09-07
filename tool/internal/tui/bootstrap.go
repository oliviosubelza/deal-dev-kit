package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

// The bootstrap screen is what `deal-kit` with no flags shows when the current
// directory is not a project. It collects only the two answers the user has
// not already given — whether to work here, and which project type — and hands
// them back so internal/cli drives the same create path `deal-kit new` uses.
// It creates nothing itself: the doctor gate and the official generator stay in
// one place, and the TUI stays a layer over the shared code.

// BootstrapConfig is resolved before the program starts, like Config.
type BootstrapConfig struct {
	Dir        string   // absolute path of the directory the user is standing in
	Markers    []string // the project markers that were looked for and not found
	Types      []string // project types, taken from the manifest
	Selected   string   // --type, if the user already named one
	CLIVersion string
}

// BootstrapResult is what the session decided.
type BootstrapResult struct {
	// Confirmed is true only once the user has both agreed to work in this
	// directory and chosen a type. Anything else leaves it false, and the
	// caller must create nothing.
	Confirmed   bool
	ProjectType string
}

type bootstrapStep int

const (
	bootstrapConfirm bootstrapStep = iota
	bootstrapType
)

// BootstrapModel is the Bubble Tea model for the screen.
type BootstrapModel struct {
	cfg    BootstrapConfig
	step   bootstrapStep
	cursor int
	width  int

	confirmed bool
	chosen    string
	quitting  bool
}

// NewBootstrap builds the screen with the question unanswered.
func NewBootstrap(cfg BootstrapConfig) BootstrapModel {
	m := BootstrapModel{cfg: cfg, width: 80}
	return m
}

func (m BootstrapModel) Init() tea.Cmd { return nil }

// Result reports what the user chose, for the caller to act on.
func (m BootstrapModel) Result() BootstrapResult {
	if !m.confirmed || m.chosen == "" {
		return BootstrapResult{}
	}
	return BootstrapResult{Confirmed: true, ProjectType: m.chosen}
}

// confirmRows are the two answers to "create the project here?". They are
// spelled out rather than a bare y/n: this is the interactive equivalent of
// --here, which is opt-in because a mistyped cd must not grow a source tree.
var confirmRows = []string{
	"Sí, crear el proyecto acá",
	"No, salir sin crear nada",
}

func (m BootstrapModel) rowCount() int {
	if m.step == bootstrapType {
		return len(m.cfg.Types)
	}
	return len(confirmRows)
}

func (m BootstrapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}
	return m, nil
}

func (m BootstrapModel) handleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		m.cursor = clamp(m.cursor-1, 0, m.rowCount()-1)
		return m, nil
	case "down", "j":
		m.cursor = clamp(m.cursor+1, 0, m.rowCount()-1)
		return m, nil
	case "esc", "left", "h":
		if m.step == bootstrapType {
			// Back to the question, and drop the agreement with it: the user
			// has not decided to write here until they pick a type.
			m.step, m.confirmed, m.cursor = bootstrapConfirm, false, 0
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
	case "enter", "right", "l", " ":
		return m.choose()
	}
	return m, nil
}

func (m BootstrapModel) choose() (tea.Model, tea.Cmd) {
	if m.step == bootstrapConfirm {
		if m.cursor != 0 {
			m.quitting = true
			return m, tea.Quit
		}
		m.confirmed, m.step = true, bootstrapType
		m.cursor = indexOf(m.cfg.Types, m.cfg.Selected)
		return m, nil
	}
	if len(m.cfg.Types) == 0 {
		m.quitting = true
		return m, tea.Quit
	}
	m.chosen = m.cfg.Types[clamp(m.cursor, 0, len(m.cfg.Types)-1)]
	m.quitting = true
	return m, tea.Quit
}

// indexOf preselects the row the user already named with --type, and 0 when
// they named nothing or named something this manifest does not declare.
func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return 0
}

// --- view ---

func (m BootstrapModel) inner() int {
	w := m.width - 10
	if w < 46 {
		return 46
	}
	if w > 84 {
		return 84
	}
	return w
}

func (m BootstrapModel) content() int { return m.inner() - panelPad }

func (m BootstrapModel) View() string {
	return panel.Width(m.inner()).Render(strings.Join(m.lines(), "\n")) + "\n"
}

// lines is the screen, one rendered line per entry. It is separate from View
// so a test can measure each line against the content width: once the panel
// has rendered them, an overflowed line is indistinguishable from an intended
// one, because the padding makes the wrapped remnant look deliberate.
func (m BootstrapModel) lines() []string {
	version := m.cfg.CLIVersion
	if version == "" {
		version = "dev"
	}
	lines := []string{
		m.title(version),
		// The absolute path, not a ~-shortened one: the user is about to let
		// deal-kit write here, and the full path is what identifies where.
		bodyText.Render(fitTail(m.cfg.Dir, m.content())),
		"",
		section.Render("Este directorio no es un proyecto"),
		"",
	}
	lines = append(lines, m.prose(
		"No se encontró ninguna marca de proyecto acá ni por encima.",
		"Se buscó: "+strings.Join(m.cfg.Markers, ", "))...)
	lines = append(lines, "")

	switch m.step {
	case bootstrapConfirm:
		lines = append(lines, bodyText.Render("¿Crear un proyecto nuevo acá?"), "")
		lines = append(lines, m.rows(confirmRows)...)
		lines = append(lines, "")
		lines = append(lines, m.prose(
			"Cancelar no escribe nada: se puede hacer cd al directorio correcto y volver a ejecutar deal-kit.")...)
		lines = append(lines, "")
		lines = append(lines, m.keyLines("↑/↓", "mover", "enter", "elegir", "esc", "cancelar")...)
	case bootstrapType:
		lines = append(lines, bodyText.Render("¿Qué tipo de proyecto es?"), "")
		lines = append(lines, m.rows(m.cfg.Types)...)
		lines = append(lines, "")
		lines = append(lines, m.prose(
			"Se ejecuta el generador oficial del tipo elegido, después de verificar que las herramientas necesarias estén instaladas.")...)
		lines = append(lines, "")
		lines = append(lines, m.keyLines("↑/↓", "mover", "enter", "crear", "esc", "volver")...)
	}
	return lines
}

// title drops the subtitle when the panel is too narrow to hold both, rather
// than letting lipgloss break the product name across two lines.
func (m BootstrapModel) title(version string) string {
	name, sub := "deal-kit "+version, "  —  kit de desarrollo compartido CRM DEAL"
	if lipgloss.Width(name+sub) > m.content() {
		return titleText.Render(name)
	}
	return titleText.Render(name) + subtle.Render(sub)
}

func (m BootstrapModel) prose(sentences ...string) []string {
	return proseAt(m.content(), sentences...)
}

func (m BootstrapModel) keyLines(pairs ...string) []string {
	return keyLinesAt(m.content(), pairs...)
}

// wrapWords breaks text on spaces so no line exceeds width.
func wrapWords(s string, width int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = word
		case len([]rune(cur))+1+len([]rune(word)) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// rows renders a single-column choice list. The marker and the label must sum
// to exactly the content width, or lipgloss wraps the row onto a second line.
func (m BootstrapModel) rows(labels []string) []string {
	out := make([]string, 0, len(labels))
	for i, label := range labels {
		cursor := i == m.cursor
		st := rowStyle(cursor)
		marker := st.Render("   ")
		if cursor {
			marker = st.Foreground(colMagenta).Bold(true).Render(" ▸ ")
		}
		out = append(out, marker+
			st.Foreground(colText).Bold(cursor).Width(m.content()-markerW).Render(label))
	}
	return out
}

// RunBootstrap starts the screen and returns what the user decided.
func RunBootstrap(cfg BootstrapConfig) (BootstrapResult, error) {
	p := tea.NewProgram(NewBootstrap(cfg))
	final, err := p.Run()
	if err != nil {
		return BootstrapResult{}, err
	}
	m, ok := final.(BootstrapModel)
	if !ok {
		return BootstrapResult{}, nil
	}
	return m.Result(), nil
}
