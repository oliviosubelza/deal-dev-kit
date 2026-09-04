package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/doctor"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/pm"
)

// renderPlan prints what a sync would do, in the order it would do it.
func renderPlan(w io.Writer, p *plan.Plan, manager pm.Manager, hasManager, noDeps bool) {
	changes := p.Changes()
	blocked := p.Blocked()

	if len(changes) == 0 && len(blocked) == 0 {
		return
	}

	fmt.Fprintln(w, "  plan:")
	for _, a := range changes {
		fmt.Fprintf(w, "    %-*s %s\n", kindW, kindLabel(a.Kind), a.Path)
	}
	if len(p.Deps) > 0 && !noDeps {
		name := string(manager)
		if !hasManager {
			name = "sin package manager detectado"
		}
		fmt.Fprintf(w, "    %-*s %s  (%s)\n", kindW, "dependencias", strings.Join(depSpecs(p.Deps), ", "), name)
	}

	if len(blocked) > 0 {
		fmt.Fprintln(w, "\n  requiere atención:")
		for _, a := range blocked {
			fmt.Fprintf(w, "    %s\n      %s\n", a.Path, a.Reason)
		}
		fmt.Fprintln(w, "\n  deal-kit no sobrescribe estos archivos. Llevar el cambio al kit,")
		fmt.Fprintln(w, "  o revertir el archivo localmente, y volver a ejecutar.")
	}
}

// kindW is the width of the action column in the plan. It is sized to the
// longest label a change can carry ("sobrescribir", and "dependencias" on the
// deps line), so the paths line up in one column. Every label is ASCII, so
// fmt's byte-counting padding is correct here.
const kindW = 12

// kindLabel is the Spanish word shown for an action. The plan.Kind constants
// stay in English: they are internal domain values, never printed directly.
func kindLabel(k plan.Kind) string {
	switch k {
	case plan.Create:
		return "crear"
	case plan.Overwrite:
		return "sobrescribir"
	case plan.Delete:
		return "borrar"
	case plan.Blocked:
		return "bloqueado"
	case plan.Unchanged:
		return "sin cambios"
	}
	return string(k)
}

// renderStatus prints one line per installed artifact. orphans are ids the
// lockfile records that the manifest no longer declares: they have no
// artifact to report against, so they are named on their own terms.
func renderStatus(w io.Writer, artifacts []kit.Artifact, orphans []string, p *plan.Plan) {
	worst := map[string]plan.Kind{}
	detail := map[string]string{}
	for _, a := range p.Actions {
		if rank(a.Kind) > rank(worst[a.ArtifactID]) {
			worst[a.ArtifactID] = a.Kind
			detail[a.ArtifactID] = a.Path
		}
	}

	ids := make([]string, 0, len(artifacts)+len(orphans))
	for _, a := range artifacts {
		ids = append(ids, a.ID)
	}
	orphaned := make(map[string]bool, len(orphans))
	for _, id := range orphans {
		orphaned[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if orphaned[id] {
			// An orphan whose files are already gone has no path to name, and
			// a dangling column reads as truncated output.
			line := statusLine(id, "HUÉRFANO", detail[id])
			fmt.Fprintln(w, strings.TrimRight(line, " "))
			continue
		}
		switch worst[id] {
		case plan.Blocked:
			fmt.Fprintln(w, statusLine(id, "MODIFICADO", detail[id]))
		case plan.Create, plan.Overwrite, plan.Delete:
			fmt.Fprintln(w, statusLine(id, "DESACTUALIZADO", detail[id]))
		default:
			fmt.Fprintln(w, strings.TrimRight(statusLine(id, "ok", ""), " "))
		}
	}
}

// Column widths for a status line. idW holds the longest artifact id with room
// to spare; statusW is sized to "DESACTUALIZADO", the longest label.
const (
	idW     = 24
	statusW = 14
)

// statusLine lays out one status row. Padding is measured in runes because
// "HUÉRFANO" is one byte longer than it is wide, and fmt's %-Ns counts bytes.
func statusLine(id, status, path string) string {
	return "  " + padRight(id, idW) + " " + padRight(status, statusW) + "  " + path
}

// padRight pads s with spaces to a rune width.
func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// rank orders states by how much they need a human's attention.
func rank(k plan.Kind) int {
	switch k {
	case plan.Blocked:
		return 3
	case plan.Create, plan.Overwrite, plan.Delete:
		return 2
	case plan.Unchanged:
		return 1
	}
	return 0
}

func depSpecs(deps map[string]string) []string {
	out := make([]string, 0, len(deps))
	for name, rng := range deps {
		out = append(out, name+"@"+rng)
	}
	sort.Strings(out)
	return out
}

// renderSummary prints where a sync landed, as a short per-directory tree.
// Echoing every path is unreadable at 74 files and tells the reader less than
// knowing which directories were touched.
func renderSummary(w io.Writer, projectRoot string, p *plan.Plan) {
	rows := plan.Summarize(p.Actions)
	if len(rows) == 0 {
		return
	}

	// Size the column to the content: a fixed width either truncates a long
	// path or leaves a gap wide enough to lose the eye across.
	width := 0
	for _, r := range rows {
		if n := len(r.Dir); n > width {
			width = n
		}
	}

	fmt.Fprintf(w, "\n  %s\n", filepath.Base(projectRoot))
	for i, r := range rows {
		branch := "├──"
		if i == len(rows)-1 {
			branch = "└──"
		}
		fmt.Fprintf(w, "  %s %-*s  %s\n", branch, width, r.Dir, counts(r))
	}
}

// counts spells each number out, so the line reads without colour.
func counts(r plan.DirSummary) string {
	var parts []string
	if r.Created > 0 {
		parts = append(parts, fmt.Sprintf("+%d %s", r.Created, plural(r.Created, "nuevo", "nuevos")))
	}
	if r.Overwritten > 0 {
		parts = append(parts, fmt.Sprintf("~%d %s", r.Overwritten, plural(r.Overwritten, "actualizado", "actualizados")))
	}
	if r.Deleted > 0 {
		parts = append(parts, fmt.Sprintf("-%d %s", r.Deleted, plural(r.Deleted, "eliminado", "eliminados")))
	}
	return strings.Join(parts, "  ")
}

// plural picks the Spanish singular or plural form for a count.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// doctorStatusW is the width of the status column in the doctor table.
const doctorStatusW = 24

// renderDoctor prints one line per tool. Status is a word, not a colour, so it
// survives a pipe and a monochrome terminal.
func renderDoctor(w io.Writer, rep doctor.Report) {
	fmt.Fprintln(w, "  herramientas")
	for _, r := range rep.Results {
		status := "no encontrado"
		if r.Found {
			status = "encontrado"
			if r.Version != "" {
				status = r.Version
			}
		} else if !r.Required {
			status = "no encontrado (opcional)"
		}
		// doctorStatusW is sized to "no encontrado (opcional)", the longest
		// label; every one of them is ASCII, so %-*s pads correctly.
		fmt.Fprintf(w, "    %-6s %-*s %s\n", r.Name, doctorStatusW, status, r.Purpose)
	}
	if missing := rep.Missing(); len(missing) > 0 {
		fmt.Fprintln(w)
		for _, r := range missing {
			fmt.Fprintf(w, "  %s es necesario para %s\n", r.Name, r.Purpose)
		}
	}
}
