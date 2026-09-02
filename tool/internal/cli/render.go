package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/deal/deal-dev-kit/tool/internal/kit"
	"github.com/deal/deal-dev-kit/tool/internal/plan"
	"github.com/deal/deal-dev-kit/tool/internal/pm"
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
