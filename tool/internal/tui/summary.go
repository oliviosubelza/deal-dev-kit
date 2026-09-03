package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// shortenPath replaces the home directory with ~ and, when still too long,
// keeps the tail: the last segments are what identify the project, and the
// leading ones are the same for every repository on the machine.
func shortenPath(p string, width int) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = "~" + string(filepath.Separator) + rel
		}
	}
	if len([]rune(p)) <= width || width < 8 {
		return p
	}
	runes := []rune(p)
	return "…" + string(runes[len(runes)-(width-1):])
}

// changeTree renders where a set of changes landed, as a short tree rather
// than a list of every path. The reader wants to know where the work went,
// not to scroll past 74 filenames.
func (m Model) changeTree(rows []plan.DirSummary) []string {
	if len(rows) == 0 {
		return nil
	}

	// Align the counts into a column, sized to the longest directory so the
	// numbers can be compared down the page instead of zig-zagging.
	width := 0
	for _, r := range rows {
		if n := len(r.Dir); n > width {
			width = n
		}
	}

	out := []string{bodyText.Render(filepath.Base(m.cfg.ProjectRoot))}
	for i, r := range rows {
		branch := "├── "
		if i == len(rows)-1 {
			branch = "└── "
		}
		out = append(out, faintText.Render(branch)+
			bodyText.Width(width+3).Render(r.Dir)+countsOf(r))
	}
	return out
}

// countsOf spells the counts out with words as well as signs, so the line
// still reads without colour.
func countsOf(r plan.DirSummary) string {
	var parts []string
	if r.Created > 0 {
		parts = append(parts, goodText.Render(fmt.Sprintf("+%d %s", r.Created, plural(r.Created, "nuevo", "nuevos"))))
	}
	if r.Overwritten > 0 {
		parts = append(parts, warnText.Render(fmt.Sprintf("~%d %s", r.Overwritten, plural(r.Overwritten, "actualizado", "actualizados"))))
	}
	if r.Deleted > 0 {
		parts = append(parts, badText.Render(fmt.Sprintf("−%d %s", r.Deleted, plural(r.Deleted, "eliminado", "eliminados"))))
	}
	return strings.Join(parts, subtle.Render("  "))
}

// plural picks the Spanish singular or plural form for a count.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
