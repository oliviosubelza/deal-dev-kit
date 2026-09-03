package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

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
		fmt.Fprintf(w, "    %-9s %s\n", a.Kind, a.Path)
	}
	if len(p.Deps) > 0 && !noDeps {
		name := string(manager)
		if !hasManager {
			name = "no package manager detected"
		}
		fmt.Fprintf(w, "    %-9s %s  (%s)\n", "deps", strings.Join(depSpecs(p.Deps), ", "), name)
	}

	if len(blocked) > 0 {
		fmt.Fprintln(w, "\n  needs attention:")
		for _, a := range blocked {
			fmt.Fprintf(w, "    %s\n      %s\n", a.Path, a.Reason)
		}
		fmt.Fprintln(w, "\n  deal-kit does not overwrite these. Bring the change back to the kit,")
		fmt.Fprintln(w, "  or revert the file locally, then run again.")
	}
}

// renderStatus prints one line per installed artifact.
func renderStatus(w io.Writer, artifacts []kit.Artifact, p *plan.Plan) {
	worst := map[string]plan.Kind{}
	detail := map[string]string{}
	for _, a := range p.Actions {
		if rank(a.Kind) > rank(worst[a.ArtifactID]) {
			worst[a.ArtifactID] = a.Kind
			detail[a.ArtifactID] = a.Path
		}
	}

	ids := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)

	for _, id := range ids {
		switch worst[id] {
		case plan.Blocked:
			fmt.Fprintf(w, "  %-24s MODIFIED  %s\n", id, detail[id])
		case plan.Create, plan.Overwrite, plan.Delete:
			fmt.Fprintf(w, "  %-24s OUTDATED  %s\n", id, detail[id])
		default:
			fmt.Fprintf(w, "  %-24s ok\n", id)
		}
	}
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
