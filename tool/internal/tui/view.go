package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deal/deal-dev-kit/tool/internal/plan"
)

// Styles use adaptive colours so the output stays legible on both light and
// dark terminals, and never relies on colour alone to convey state.
var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "244", Dark: "245"})
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "78"})
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "214"})
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"})
	cursorStyle = lipgloss.NewStyle().Bold(true)
)

// View renders the current stage.
func (m Model) View() string {
	var b strings.Builder

	kitVersion := m.cfg.KitVersion
	if kitVersion == "" {
		kitVersion = "unpinned"
	}
	b.WriteString(titleStyle.Render("deal-kit"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %s · %s · kit %s",
		m.cfg.ProjectName, m.cfg.ProjectType, kitVersion)))
	b.WriteString("\n\n")

	switch m.stage {
	case stageSelect:
		m.viewSelect(&b)
	case stagePlan:
		m.viewPlan(&b)
	case stageApplied:
		b.WriteString(okStyle.Render(fmt.Sprintf("  applied: %d file(s) changed", m.changed)) + "\n")
	case stageFailed:
		b.WriteString(errStyle.Render(fmt.Sprintf("  error: %v", m.err)) + "\n")
	}
	return b.String()
}

func (m Model) viewSelect(b *strings.Builder) {
	for i, it := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("› ")
		}

		box := "[ ]"
		if it.selected() {
			box = "[x]"
		}

		line := fmt.Sprintf("%s%s %-24s %s", cursor, box, it.id, dimStyle.Render(it.kind))
		switch {
		case it.required && !it.explicit:
			line += dimStyle.Render("  (required)")
		case it.installed && !it.selected():
			line += warnStyle.Render("  (will be released)")
		case it.installed:
			line += dimStyle.Render("  installed")
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d selected", m.countSelected())) + "\n")
	b.WriteString(dimStyle.Render("  ↑↓ move · space toggle · a all · n none · enter plan · q quit") + "\n")
}

func (m Model) viewPlan(b *strings.Builder) {
	changes := m.plan.Changes()
	blocked := m.plan.Blocked()

	if len(changes) == 0 && len(blocked) == 0 {
		b.WriteString(okStyle.Render("  already up to date") + "\n\n")
		b.WriteString(dimStyle.Render("  esc back · q quit") + "\n")
		return
	}

	if len(changes) > 0 {
		b.WriteString("  plan:\n")
		for _, a := range changes {
			style := dimStyle
			if a.Kind == plan.Delete {
				style = warnStyle
			}
			b.WriteString(fmt.Sprintf("    %s %s\n", style.Render(pad(string(a.Kind), 9)), a.Path))
		}
	}

	if len(m.plan.Deps) > 0 {
		mgr := m.cfg.PackageMgr
		if mgr == "" {
			mgr = "no package manager detected"
		}
		b.WriteString(fmt.Sprintf("    %s %s %s\n",
			dimStyle.Render(pad("deps", 9)),
			strings.Join(depSpecs(m.plan.Deps), ", "),
			dimStyle.Render("("+mgr+")")))
	}

	if len(blocked) > 0 {
		b.WriteString("\n" + warnStyle.Render("  needs attention:") + "\n")
		for _, a := range blocked {
			b.WriteString(fmt.Sprintf("    %s\n      %s\n", a.Path, dimStyle.Render(a.Reason)))
		}
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  deal-kit will not overwrite these. Bring the change back to the") + "\n")
		b.WriteString(dimStyle.Render("  kit, or revert the file locally, then run again.") + "\n\n")
		b.WriteString(dimStyle.Render("  esc back · q quit") + "\n")
		return
	}

	b.WriteString("\n" + dimStyle.Render("  y apply · esc back · q quit") + "\n")
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func depSpecs(deps map[string]string) []string {
	out := make([]string, 0, len(deps))
	for name, rng := range deps {
		out = append(out, name+"@"+rng)
	}
	sortStrings(out)
	return out
}
