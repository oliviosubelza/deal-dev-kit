package tui

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/engram"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// hostGOOS is the platform the warnings on the Engram screen are written for.
// It is runtime.GOOS behind a variable for one reason: a golden snapshot must
// not differ between a Linux and a Windows developer, so the golden tests pin
// it. Nothing in production ever assigns it.
var hostGOOS = runtime.GOOS

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

	end := min(m.top+m.height, len(rows))
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
		created, overwritten, deleted, lines, keys := countKinds(changes)
		// Say the scope out loud: the plan covers everything selected across
		// both screens, not just the one the user came from.
		out = append(out,
			subtle.Render("Todo el proyecto  ·  "+summary(created, overwritten, deleted, lines, keys)), "")

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
		label := a.Path
		if a.Kind == plan.AppendLine {
			// The file is the project's; only the line is ours. Say so, or the
			// row reads as "deal-kit is going to write your CLAUDE.md".
			label += "  " + a.Line
		}
		if a.Kind == plan.MergeJSON {
			// Same: the file is the project's, only these keys are ours.
			label += "  " + strings.Join(a.Keys, ", ")
		}
		out = append(out, "  "+kindGlyph(a.Kind)+bodyText.Render("  "+label))
	}
	if len(actions) > len(shown) {
		out = append(out, faintText.Render(
			fmt.Sprintf("     … y %d archivo(s) más", len(actions)-len(shown))))
	}
	return append(out, ""), budget - len(shown)
}

func countKinds(actions []plan.Action) (created, overwritten, deleted, lines, keys int) {
	for _, a := range actions {
		switch a.Kind {
		case plan.Create:
			created++
		case plan.Overwrite:
			overwritten++
		case plan.Delete:
			deleted++
		case plan.AppendLine:
			lines++
		case plan.MergeJSON:
			keys++
		}
	}
	return created, overwritten, deleted, lines, keys
}

// summary counts ensured lines and JSON keys apart from files. Folding them
// into "nuevos" would claim deal-kit created a file it only appended one line,
// or two settings, to.
func summary(created, overwritten, deleted, lines, keys int) string {
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
	if lines > 0 {
		word := "líneas"
		if lines == 1 {
			word = "línea"
		}
		parts = append(parts, fmt.Sprintf("%d %s", lines, word))
	}
	if keys > 0 {
		word := "ajustes"
		if keys == 1 {
			word = "ajuste"
		}
		parts = append(parts, fmt.Sprintf("%d %s", keys, word))
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
	case plan.AppendLine, plan.MergeJSON:
		return goodText.Render("+")
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
		"Memoria persistente para el agente de IA: guarda decisiones, bugs y convenciones, y las recupera en la sesión siguiente.",
		"Instala los hooks de sesión y la skill \"memory\" en la configuración global de Claude Code.")...)
	out = append(out, "")

	out = append(out, m.field("repo", engram.MarketplaceRepo, colText))
	out = append(out, m.field("marketplace", engramMarketplaceField(st), colText))
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

	// The download step's own line already says the asset, the exact target
	// and whether it replaces a file that is there; restating any of it below
	// the command only teaches the reader that this screen repeats itself.
	dl, hasDownload := engram.Download{}, false
	if !m.cfg.EngramPlan.Empty() {
		out = append(out, section.Render("Comandos"), "")
		for _, line := range m.cfg.EngramPlan.Lines() {
			out = append(out, m.commandLines(line)...)
		}
		out = append(out, "")
		// The destination is named before the user consents: this writes an
		// executable outside the project.
		if d, ok := m.cfg.EngramPlan.Binary(); ok {
			dl, hasDownload = d, true
			out = append(out, m.field("destino", d.Dir, colText), "")
		}
	}

	out = append(out, m.warnings(st)...)
	// Last, right above the keys: it is the only thing on this screen the
	// install cannot do for the user, so it is the one that must not be
	// crowded out by the warnings above it.
	out = append(out, m.pathAction(st, dl, hasDownload)...)

	if reason := m.engramBlocked(); reason != "" {
		out = append(out, warnText.Render(clip(reason, m.content())), "")
		return append(out, m.keyLines("esc", "volver", "q", "salir")...)
	}
	return append(out, m.keyLines("y", "instalar", "n", "cancelar", "esc", "volver", "q", "salir")...)
}

// engramMarketplaceField is the pinned tag and, when the marketplace already
// registered on this machine sits at another one, what it is actually at. Same
// repository, so it is not a conflict; it is the only thing that explains a
// plugin version that does not match the pinned tag.
func engramMarketplaceField(st engram.Status) string {
	line := engram.MarketplaceTag + "  (fijado)"
	if st.RefMismatch() {
		line += " · registrado en " + st.FoundRef
	}
	return line
}

// warnings are the things that do not stop the install but make it not work.
func (m Model) warnings(st engram.Status) []string {
	var out []string
	// Reported, never acted on: re-pointing the marketplace means removing one
	// the user registered, and deal-kit does not remove what it did not put
	// there. Said before the install so the version that lands is not a
	// surprise.
	if st.RefMismatch() {
		out = append(out, m.prose("El marketplace registrado está en "+st.FoundRef+
			", no en "+engram.MarketplaceTag+". Es el mismo repositorio, así que deal-kit no lo toca: "+
			"para moverlo hay que quitarlo y volver a agregarlo a mano.")...)
	}
	// The hooks are shell scripts. cmd.exe cannot run them, so on Windows the
	// plugin installs and then silently never fires. Shown on Windows only:
	// internal/cli already gates the same sentence that way, and a warning
	// about another operating system is noise that trains the reader to skip
	// the ones that do apply.
	if hostGOOS == "windows" {
		out = append(out, m.prose(
			"En Windows los hooks necesitan Git Bash o WSL: sin uno de los dos se instalan pero nunca se ejecutan.")...)
	}
	// A missing binary is not warned about here: pathAction says it once,
	// with the command that fixes it. Nothing about `engram setup
	// claude-code` either — the plugin ships its own .mcp.json and the
	// install registers the MCP server, so telling the user it is still
	// pending sent them to run a command they did not need.
	if len(out) == 0 {
		return nil
	}
	return append(out, "")
}

// pathAction is the one thing on this screen deal-kit cannot do for the user,
// said once and with the exact command.
//
// deal-kit still does not edit PATH: that is the same global mutation it
// refuses when it finds a foreign marketplace, and a shell's configuration is
// its owner's. Refusing to run the command is not a reason to make the reader
// work out what it is.
func (m Model) pathAction(st engram.Status, d engram.Download, hasDownload bool) []string {
	if st.EngramBinaryFound() || st.State == engram.StateClaudeMissing {
		return nil
	}
	if hasDownload && d.OnPath {
		// The binary lands in a directory the shell already searches, so
		// there is nothing to do after pressing y.
		return nil
	}
	if !hasDownload {
		// No destination to name: `go install` writes to GOBIN, and a blocked
		// or empty plan has no directory at all. The consequence is still
		// worth one line, since the table row above only reports the fact.
		return append(m.prose(
			"engram no está en el PATH: sin él fallan los hooks y el servidor MCP."), "")
	}
	out := m.warn("Falta: engram no está en el PATH — el servidor MCP no va a arrancar.")
	out = append(out, "")
	cmds, after := PathHint(hostGOOS, d.Dir)
	for _, c := range cmds {
		out = append(out, m.commandLines(c)...)
	}
	out = append(out, "")
	out = append(out, m.prose(after)...)
	return append(out, "")
}

// PathHint is the command that puts dir on PATH on goos, and what to do after
// running it. It is exported because internal/cli prints the same advice after
// a non-interactive install: this screen and that output disagreeing is a bug
// this repository has already shipped once, with the Windows hooks warning.
//
// The POSIX form is the line itself plus where it goes, rather than an edit to
// a specific rc file: deal-kit cannot tell which shell is in use and must not
// name a file it never looked at.
//
// The Windows form is deliberately three lines and not `setx`. `setx PATH
// "%PATH%;<dir>"` fits on one line and is what a short panel invites, but in
// cmd.exe %PATH% expands to the system and user paths merged, so it copies the
// system half into the user's, and setx truncates the result at 1024
// characters without saying so. A command that can quietly break the user's
// environment is not made acceptable by fitting the layout.
func PathHint(goos, dir string) (cmds []string, after string) {
	if goos == "windows" {
		return []string{
			`$d = "` + dir + `"`,
			`$u = [Environment]::GetEnvironmentVariable('Path','User')`,
			`[Environment]::SetEnvironmentVariable('Path',"$u;$d",'User')`,
		}, "Después reiniciar la terminal y Claude Code."
	}
	return []string{`export PATH="$PATH:` + dir + `"`},
		"Agregar esa línea al arranque de la shell (~/.zshrc, ~/.bashrc) y reiniciar Claude Code."
}

// warn renders a sentence in the warning colour, wrapped like prose. The one
// line the reader must act on cannot be the same weight as the notes around
// it.
func (m Model) warn(s string) []string {
	var out []string
	for _, l := range wrapWords(s, m.content()) {
		out = append(out, warnText.Render(clip(l, m.content())))
	}
	return out
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
	first := true
	for _, l := range wrapWords(line, m.content()-4) {
		// A command is never clipped. Everywhere else clip() is right: a
		// truncated sentence still reads as a sentence. A truncated command
		// reads as a whole command and is not one, so a reader copies
		// something that does something else — and the PATH command this
		// exists for is one whose broken form damages the environment. Words
		// that do not fit are broken by character instead.
		for _, part := range hardWrap(l, m.content()-4) {
			prefix := "  "
			if !first {
				prefix = "    "
			}
			first = false
			out = append(out, faintText.Render(prefix+part))
		}
	}
	return out
}

// hardWrap splits s into runs of at most width runes, breaking mid-word when a
// single token is longer than the line. It counts runes, not bytes: a byte
// split would cut a multi-byte character in half.
func hardWrap(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	r := []rune(s)
	if len(r) <= width {
		return []string{s}
	}
	var out []string
	for len(r) > width {
		out = append(out, string(r[:width]))
		r = r[width:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
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
