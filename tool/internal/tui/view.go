package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/engram"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// panelPad is the horizontal padding inside the border, both sides.
const panelPad = 6

// content is the width available for a line of text: the panel width minus its
// own padding. Computing widths from inner() instead overflows by exactly the
// padding and makes lines wrap.
func (m Model) content() int { return m.inner() - panelPad }

// inner is the panel width, excluding the border.
func (m Model) inner() int {
	w := m.width - 10
	if w < 46 {
		return 46
	}
	if w > 84 {
		return 84
	}
	return w
}

// View renders the current screen inside the panel.
func (m Model) View() string {
	var lines []string
	lines = append(lines, m.titleLines()...)

	switch m.screen {
	case screenMenu:
		lines = append(lines, m.menuLines()...)
	case screenSkills, screenComponents:
		lines = append(lines, m.listLines()...)
	case screenStatus:
		lines = append(lines, m.statusLines()...)
	case screenEngram:
		lines = append(lines, m.engramLines()...)
	case screenPlan:
		lines = append(lines, m.planLines()...)
	case screenApplied:
		lines = append(lines, "",
			goodText.Render("✓ Aplicado")+subtle.Render(fmt.Sprintf("   %d archivo(s)", m.changed)), "")
		lines = append(lines, m.changeTree(m.appliedRows)...)
		lines = append(lines, "",
			subtle.Render("deal-kit.lock registra qué archivos son del kit."))
	case screenFailed:
		lines = append(lines, "", badText.Render("✗ "+m.err.Error()))
	}

	return panel.Width(m.inner()).Render(strings.Join(lines, "\n")) + "\n"
}

func (m Model) titleLines() []string {
	version := m.cfg.CLIVersion
	if version == "" {
		version = "dev"
	}
	title := titleText.Render("deal-kit "+version) +
		subtle.Render("  —  kit de desarrollo compartido CRM DEAL")

	crumbs := subtle.Render(m.cfg.ProjectName+"  ·  "+string(m.cfg.ProjectType)) +
		faintText.Render("  ·  kit ")
	if m.cfg.PinnedKit != "" {
		crumbs += subtle.Render(m.cfg.PinnedKit)
	} else {
		crumbs += faintText.Render("sin fijar")
	}
	if m.updateAvailable() {
		crumbs += warnText.Render("  ↑ " + m.cfg.KitVersion + " disponible")
	}
	// Always show which directory is being written to. "crm-deal-web" alone is
	// not enough when several checkouts share a name.
	where := faintText.Render(shortenPath(m.cfg.ProjectRoot, m.content()))
	return []string{title, crumbs, where, ""}
}

// --- menu ---

func (m Model) menuLines() []string {
	entries := m.menu()
	out := []string{section.Render("Menú"), ""}

	for i, e := range entries {
		cursor := i == m.menuCursor
		st := rowStyle(cursor)

		marker := st.Foreground(colMagenta).Bold(true).Render(" ▸ ")
		if !cursor {
			marker = st.Render("   ")
		}
		title := st.Foreground(colText).Bold(cursor).Width(26).Render(e.title)
		note := st.Foreground(colFaint).Width(m.content() - 29).Render(e.note)
		out = append(out, marker+title+note)
	}

	return append(out, "", keys("↑/↓", "mover", "enter", "elegir", "q", "salir"))
}

// --- artifact lists ---

func (m Model) listLines() []string {
	rows := m.rows()
	var out []string

	switch m.screen {
	case screenSkills:
		out = append(out, section.Render("Skills y convenciones"), "",
			subtle.Render("Reglas que sigue el agente de IA en este repositorio."),
			subtle.Render("Se commitean, así que el equipo las recibe con un git pull."), "")
	case screenComponents:
		out = append(out, section.Render("Componentes de UI"), "",
			subtle.Render("Se copian como código fuente en "+m.uiRoot()+"."),
			subtle.Render("Las dependencias entre componentes se agregan solas."), "")
	}

	if len(rows) == 0 {
		out = append(out, faintText.Render(fmt.Sprintf("   nada coincide con %q", m.filter)))
	}

	end := m.top + m.height
	if end > len(rows) {
		end = len(rows)
	}
	for i := m.top; i < end; i++ {
		out = append(out, m.row(rows[i], i == m.cursor))
	}
	for i := end - m.top; i < m.height; i++ {
		out = append(out, "")
	}

	if m.top > 0 || end < len(rows) {
		out = append(out, faintText.Render(fmt.Sprintf("   %d–%d de %d", m.top+1, end, len(rows))))
	} else {
		out = append(out, "")
	}

	return append(out, m.listFooter()...)
}

func (m Model) uiRoot() string {
	if r, ok := m.cfg.Roots["ui"]; ok {
		return r
	}
	return "el proyecto"
}

func (m Model) row(r row, cursor bool) string {
	st := rowStyle(cursor)
	w := m.content()

	marker := st.Render("   ")
	if cursor {
		marker = st.Foreground(colMagenta).Bold(true).Render(" ▸ ")
	}

	if r.isHeading() {
		g := m.groups[r.group]
		arrow := "▸"
		if !g.collapsed || m.filter != "" {
			arrow = "▾"
		}
		total, sel := m.groupCounts(g)
		name := st.Foreground(colText).Bold(true).
			Width(w - markerW - boxW - countW).Render(arrow + "  " + g.name)
		count := st.Foreground(colMuted).Width(countW).Align(lipgloss.Right).
			Render(fmt.Sprintf("%d/%d", sel, total))
		return marker + checkbox(m.groupState(g), cursor) + name + count
	}

	it := m.items[r.item]
	indent := ""
	nameW := w - markerW - boxW - noteW
	if m.screen == screenComponents {
		indent = st.Render("  ")
		nameW -= 2
	}

	fg := colMuted
	if it.selected() {
		fg = colText
	}
	name := st.Foreground(fg).Width(nameW).Render(it.label())

	note, noteColor := "", colFaint
	switch {
	case it.required && !it.explicit:
		note = pulledByLabel(it.pulledBy, noteW)
	case it.installed && !it.selected():
		note, noteColor = "se va a soltar", colWarn
	case it.installed:
		note = "instalado"
	}
	// On the skills screen the group is the useful context, not "installed".
	if m.screen == screenSkills && note == "" {
		note = it.group
	}
	return marker + indent + checkbox(boolCheck(it.selected()), cursor) +
		name + st.Foreground(noteColor).Width(noteW).Render(note)
}

// Column widths for a list line. They must sum to exactly the content width,
// or a row overflows the panel and lipgloss wraps it onto a second line.
const (
	markerW = 3  // " ▸ "
	boxW    = 4  // "[x] "
	countW  = 8  // "12/18"
	noteW   = 18 // "will be released"
)

// pulledByLabel names what requires a dependency. A bare "dependency" leaves
// the user unable to tell why it is there or what to deselect to drop it.
func pulledByLabel(by []string, width int) string {
	if len(by) == 0 {
		return "requerido"
	}
	first := by[0]
	if i := strings.LastIndex(first, "/"); i >= 0 {
		first = first[i+1:]
	}
	label := "← " + first
	if len(by) > 1 {
		label += fmt.Sprintf(" +%d", len(by)-1)
	}
	if len(label) > width {
		return fmt.Sprintf("← %d que lo usan", len(by))
	}
	return label
}

// checkbox renders a tri-state box. The glyphs differ, not only the colour.
func checkbox(c check, cursor bool) string {
	st := rowStyle(cursor)
	switch c {
	case checked:
		return st.Foreground(colPurple).Bold(true).Render("[x] ")
	case partial:
		return st.Foreground(colMagenta).Bold(true).Render("[~] ")
	default:
		return st.Foreground(colFaint).Render("[ ] ")
	}
}

func boolCheck(b bool) check {
	if b {
		return checked
	}
	return unchecked
}

func (m Model) listFooter() []string {
	if m.filtering {
		return []string{
			keyCap.Render("filtro ") + bodyText.Render(m.filter) +
				lipgloss.NewStyle().Background(colPurple).Render(" "),
			keys("enter", "aplicar filtro", "esc", "limpiar"),
		}
	}

	summary := subtle.Render(fmt.Sprintf("%d elegidos en esta pantalla", m.selectedHere()))
	if m.filter != "" {
		summary += faintText.Render(fmt.Sprintf("   filtro %q", m.filter))
	}

	if m.screen == screenComponents {
		return []string{
			summary,
			keys("↑/↓", "mover", "espacio", "marcar", "←/→", "plegar", "tab", "plegar todo"),
			keys("a", "todos", "n", "ninguno", "/", "filtrar", "enter", "revisar", "esc", "volver"),
		}
	}
	return []string{
		summary,
		keys("↑/↓", "mover", "espacio", "marcar", "a", "todas", "n", "ninguna"),
		keys("/", "filtrar", "enter", "revisar", "esc", "volver", "q", "salir"),
	}
}

// selectedHere counts only what the current screen shows, so the number always
// matches what the user is looking at.
func (m Model) selectedHere() int {
	n := 0
	for _, r := range m.rows() {
		if !r.isHeading() && m.items[r.item].selected() {
			n++
		}
	}
	return n
}

// --- status ---

func (m Model) statusLines() []string {
	out := []string{section.Render("Estado del proyecto"), ""}

	out = append(out, m.field("kit fijado", orDash(m.cfg.PinnedKit), colText))
	if m.updateAvailable() {
		out = append(out, m.field("kit disponible", m.cfg.KitVersion, colWarn))
	}
	out = append(out, m.field("tipo de proyecto", string(m.cfg.ProjectType), colText))
	out = append(out, m.field("package manager", orDash(m.cfg.PackageMgr), colText))
	out = append(out, "")

	out = append(out, section.Render("Instalado"), "")
	installed := m.cfg.Lock.Artifacts
	if len(installed) == 0 {
		out = append(out, faintText.Render("   todavía nada — instalar desde el menú"))
	}
	shown := 0
	for _, in := range installed {
		if shown == 10 {
			out = append(out, faintText.Render(fmt.Sprintf("   … y %d más", len(installed)-shown)))
			break
		}
		out = append(out, m.field(in.ID, fmt.Sprintf("%d archivo(s)", len(in.Files)), colMuted))
		shown++
	}

	out = append(out, "")
	if m.updateAvailable() {
		out = append(out, keys("u", "actualizar el kit", "esc", "volver", "q", "salir"))
	} else {
		out = append(out, keys("esc", "volver", "q", "salir"))
	}
	return out
}

func (m Model) field(name, value string, c lipgloss.Color) string {
	return subtle.Width(fieldNameW).Render("  "+name) +
		bg.Foreground(c).Render(clip(value, m.content()-fieldNameW))
}

// fieldNameW is the label column of a name/value line.
const fieldNameW = 22

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// --- plan ---

func (m Model) planLines() []string {
	changes := m.plan.Changes()
	blocked := m.plan.Blocked()
	out := []string{section.Render("Revisar cambios"), ""}

	if len(changes) == 0 && len(blocked) == 0 {
		return append(out,
			goodText.Render("✓ Ya está todo al día"), "",
			subtle.Render("No hay nada que escribir."), "",
			keys("esc", "volver", "q", "salir"))
	}

	if len(changes) > 0 {
		created, overwritten, deleted := countKinds(changes)
		// Say the scope out loud: the plan covers everything selected across
		// both screens, not just the one the user came from.
		out = append(out,
			subtle.Render("Todo el proyecto  ·  "+summary(created, overwritten, deleted)), "")

		skills, components := m.splitChanges(changes)
		budget := 12
		out, budget = appendChangeSection(out, "Skills y convenciones", skills, budget)
		out, _ = appendChangeSection(out, "Componentes de UI", components, budget)
	}

	if len(m.plan.Deps) > 0 {
		mgr := m.cfg.PackageMgr
		if mgr == "" {
			mgr = "sin package manager detectado"
		}
		out = append(out, "", section.Render("Dependencias npm")+subtle.Render("   "+mgr), "")
		for _, l := range wrap(depSpecs(m.plan.Deps), m.content()-2) {
			out = append(out, subtle.Render("  "+l))
		}
	}

	if len(blocked) > 0 {
		out = append(out, "", badText.Render("Requiere atención"), "")
		for _, a := range blocked {
			out = append(out, bodyText.Render("  "+a.Path), faintText.Render("    "+a.Reason))
		}
		out = append(out, "",
			subtle.Render("deal-kit no sobrescribe estos archivos. Llevar el cambio al kit,"),
			subtle.Render("o revertir el archivo localmente, y volver a intentar."), "",
			keys("esc", "volver", "q", "salir"))
		return out
	}

	return append(out, "", keys("y", "aplicar", "n", "cancelar", "esc", "volver", "q", "salir"))
}

// splitChanges separates the plan by what the user recognises: the rules the
// agent follows, and the code that lands in the project.
func (m Model) splitChanges(actions []plan.Action) (skills, components []plan.Action) {
	kind := map[string]string{}
	for _, it := range m.items {
		kind[it.id] = it.kind
	}
	for _, a := range actions {
		if kind[a.ArtifactID] == "skill" {
			skills = append(skills, a)
		} else {
			components = append(components, a)
		}
	}
	return skills, components
}

// appendChangeSection renders one titled block of actions, spending from a
// shared row budget so a long plan cannot push the keys off the screen.
func appendChangeSection(out []string, title string, actions []plan.Action, budget int) ([]string, int) {
	if len(actions) == 0 || budget <= 0 {
		return out, budget
	}
	out = append(out, faintText.Render(title+"  ("+itoa(len(actions))+")"))

	shown := actions
	if len(shown) > budget {
		shown = shown[:budget]
	}
	for _, a := range shown {
		out = append(out, "  "+kindGlyph(a.Kind)+bodyText.Render("  "+a.Path))
	}
	if len(actions) > len(shown) {
		out = append(out, faintText.Render(
			fmt.Sprintf("     … y %d archivo(s) más", len(actions)-len(shown))))
	}
	return append(out, ""), budget - len(shown)
}

func countKinds(actions []plan.Action) (created, overwritten, deleted int) {
	for _, a := range actions {
		switch a.Kind {
		case plan.Create:
			created++
		case plan.Overwrite:
			overwritten++
		case plan.Delete:
			deleted++
		}
	}
	return created, overwritten, deleted
}

func summary(created, overwritten, deleted int) string {
	var parts []string
	if created > 0 {
		parts = append(parts, fmt.Sprintf("%d nuevos", created))
	}
	if overwritten > 0 {
		parts = append(parts, fmt.Sprintf("%d actualizados", overwritten))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("%d eliminados", deleted))
	}
	return strings.Join(parts, "  ·  ")
}

// kindGlyph marks an action with a symbol as well as a colour.
func kindGlyph(k plan.Kind) string {
	switch k {
	case plan.Create:
		return goodText.Render("+")
	case plan.Overwrite:
		return warnText.Render("~")
	case plan.Delete:
		return badText.Render("−")
	}
	return bg.Render(" ")
}

// keys renders alternating key/description pairs.
func keys(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString(keyText.Render("  •  "))
		}
		b.WriteString(keyCap.Render(pairs[i]))
		b.WriteString(keyText.Render(": " + pairs[i+1]))
	}
	return b.String()
}

// wrap splits items onto lines that fit the given width.
func wrap(items []string, width int) []string {
	var lines []string
	cur := ""
	for _, s := range items {
		switch {
		case cur == "":
			cur = s
		case len(cur)+2+len(s) <= width:
			cur += ", " + s
		default:
			lines = append(lines, cur)
			cur = s
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func depSpecs(deps map[string]string) []string {
	out := make([]string, 0, len(deps))
	for name, rng := range deps {
		out = append(out, name+"@"+rng)
	}
	sortStrings(out)
	return out
}

// --- engram ---

// engramLines is the consent screen for the Claude Code plugin. It renders the
// state that was resolved before the program started and the exact commands
// that would run; nothing here queries or executes anything, because a
// streamed install belongs in the normal terminal, not the alternate screen.
func (m Model) engramLines() []string {
	st := m.cfg.Engram
	out := []string{section.Render("Engram para Claude Code"), ""}
	out = append(out, m.prose(
		"Memoria persistente para el agente de IA: guarda decisiones, bugs y convenciones, y las recupera en la sesión siguiente.")...)
	out = append(out, "")

	out = append(out, m.field("repo", engram.MarketplaceRepo, colText))
	out = append(out, m.field("marketplace", engram.MarketplaceTag+"  (fijado)", colText))
	out = append(out, m.field("plugin", engram.PluginID+"  "+orDash(st.Version), colText))
	// Say the scope out loud: this writes to the user's global Claude Code
	// configuration, not to the project deal-kit is otherwise working on.
	out = append(out, m.field("alcance", "usuario · GLOBAL, no toca el proyecto", colWarn))
	out = append(out, m.field("claude", orDash(st.ClaudePath), colText))
	engramWhere, engramColor := st.EngramPath, colText
	if engramWhere == "" {
		engramWhere, engramColor = "no está en el PATH", colWarn
	}
	out = append(out, m.field("engram", engramWhere, engramColor))
	out = append(out, "")

	out = append(out, section.Render("Estado"), "")
	out = append(out, m.engramStateLines()...)
	out = append(out, "")

	out = append(out, section.Render("Qué se instala"), "")
	out = append(out, m.prose(
		"Los hooks de sesión, los scripts que los ejecutan y la skill \"memory\", dentro de la configuración global de Claude Code.")...)
	out = append(out, "")

	if !m.cfg.EngramPlan.Empty() {
		out = append(out, section.Render("Comandos"), "")
		for _, line := range m.cfg.EngramPlan.Lines() {
			out = append(out, m.commandLines(line)...)
		}
		out = append(out, "")
	}

	out = append(out, m.warnings(st)...)

	if reason := m.engramBlocked(); reason != "" {
		out = append(out, warnText.Render(clip(reason, m.content())), "")
		return append(out, m.keyLines("esc", "volver", "q", "salir")...)
	}
	return append(out, m.keyLines("y", "instalar", "n", "cancelar", "esc", "volver", "q", "salir")...)
}

// warnings are the things that do not stop the install but make it not work.
func (m Model) warnings(st engram.Status) []string {
	var out []string
	// The hooks are shell scripts. cmd.exe cannot run them, so on Windows the
	// plugin installs and then silently never fires.
	out = append(out, m.prose(
		"En Windows los hooks necesitan Git Bash o WSL: sin uno de los dos se instalan pero nunca se ejecutan.")...)
	if !st.EngramBinaryFound() && st.State != engram.StateClaudeMissing {
		out = append(out, m.prose(
			"El binario engram no está en el PATH. El plugin se instala igual, pero los hooks fallan hasta que esté.")...)
	}
	out = append(out, m.prose(
		"Después queda pendiente `engram setup claude-code`, que registra el servidor MCP. deal-kit no lo ejecuta: cambia permisos y otros archivos globales.")...)
	return append(out, "")
}

// engramStateLines is the one message that says what was found.
func (m Model) engramStateLines() []string {
	style, text := engramStateMessage(m.cfg.Engram)
	var out []string
	for _, l := range wrapWords(text, m.content()) {
		out = append(out, style.Render(clip(l, m.content())))
	}
	return out
}

func engramStateMessage(st engram.Status) (lipgloss.Style, string) {
	switch st.State {
	case engram.StateReady:
		return goodText, "✓ El marketplace es el correcto y el plugin está habilitado."
	case engram.StatePluginDisabled:
		return warnText, "El plugin está instalado pero deshabilitado."
	case engram.StatePluginMissing:
		return subtle, "El marketplace ya está registrado; falta instalar el plugin."
	case engram.StateMarketplaceMissing:
		return subtle, "El marketplace engram todavía no está registrado."
	case engram.StateMarketplaceConflict:
		return badText, "Ya hay un marketplace llamado engram que apunta a " +
			orDash(st.FoundRepo) + ". deal-kit no lo modifica: resolverlo a mano."
	case engram.StateClaudeMissing:
		return badText, "No se encontró claude en el PATH: instalar Claude Code primero."
	default:
		msg := "No se pudo leer el estado del plugin."
		if st.Err != nil {
			msg = "No se pudo leer el estado del plugin: " + st.Err.Error()
		}
		return badText, msg
	}
}

// commandLines renders one command, wrapped and indented. The marketplace URL
// is a single 60-character token, so it is clipped rather than allowed to
// overflow: the repo and the pinned tag are already named as fields above.
func (m Model) commandLines(line string) []string {
	var out []string
	for i, l := range wrapWords(line, m.content()-4) {
		prefix := "  "
		if i > 0 {
			prefix = "    "
		}
		out = append(out, faintText.Render(clip(prefix+l, m.content())))
	}
	return out
}

// --- shared layout helpers ---

// proseAt word-wraps sentences to a width and renders each resulting line on
// its own. A \n inside a single Render would be padded with spaces on every
// line, and an over-wide line would be wrapped by the panel instead.
func proseAt(width int, sentences ...string) []string {
	var out []string
	for _, s := range sentences {
		for _, line := range wrapWords(s, width) {
			out = append(out, subtle.Render(clip(line, width)))
		}
	}
	return out
}

// keyLinesAt lays out key/description pairs across as many lines as the panel
// needs. A single keys() call is one string that the panel would wrap
// mid-legend on a narrow terminal.
func keyLinesAt(width int, pairs ...string) []string {
	var out []string
	var chunk []string
	flush := func() {
		if len(chunk) > 0 {
			out = append(out, keys(chunk...))
			chunk = nil
		}
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		next := append(append([]string{}, chunk...), pairs[i], pairs[i+1])
		if len(chunk) > 0 && lipgloss.Width(keys(next...)) > width {
			flush()
			next = []string{pairs[i], pairs[i+1]}
		}
		chunk = next
	}
	flush()
	return out
}

func (m Model) prose(sentences ...string) []string { return proseAt(m.content(), sentences...) }

func (m Model) keyLines(pairs ...string) []string { return keyLinesAt(m.content(), pairs...) }

// clip truncates to a width, marking the cut. A token longer than the panel
// cannot be word-wrapped, and letting it through makes the panel wrap it into
// a padded remnant that looks like a line someone intended.
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}
