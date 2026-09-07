package tui

import (
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// An ensured line is a change, so the plan screen has to count it. A plan that
// only adds the persona import would otherwise show an empty scope line.
func TestCountKindsCountsAnEnsuredLineApartFromFiles(t *testing.T) {
	created, overwritten, deleted, lines := countKinds([]plan.Action{
		{Kind: plan.Create, Path: ".claude/persona.md"},
		{Kind: plan.AppendLine, Path: "CLAUDE.md", Line: "@.claude/persona.md"},
	})
	if created != 1 || overwritten != 0 || deleted != 0 {
		t.Errorf("files = %d/%d/%d, want 1/0/0", created, overwritten, deleted)
	}
	if lines != 1 {
		t.Errorf("lines = %d, want 1", lines)
	}

	got := summary(created, overwritten, deleted, lines)
	if !strings.Contains(got, "1 nuevos") || !strings.Contains(got, "1 línea") {
		t.Errorf("summary = %q, want both the file and the line", got)
	}
	if got := summary(0, 0, 0, 1); got != "1 línea" {
		t.Errorf("a line-only plan summarises as %q, want %q", got, "1 línea")
	}
}

// The row must say which line lands, or it reads as "deal-kit is going to
// write your CLAUDE.md".
func TestThePlanRowNamesTheLineNotJustTheFile(t *testing.T) {
	out, _ := appendChangeSection(nil, "Skills y convenciones", []plan.Action{
		{Kind: plan.AppendLine, Path: "CLAUDE.md", Line: "@.claude/persona.md"},
	}, 12)

	joined := ansi.ReplaceAllString(strings.Join(out, "\n"), "")
	if !strings.Contains(joined, "CLAUDE.md") || !strings.Contains(joined, "@.claude/persona.md") {
		t.Errorf("the row does not name both the file and the line:\n%s", joined)
	}
	if kindGlyph(plan.AppendLine) == kindGlyph(plan.Unchanged) {
		t.Error("an ensured line renders with the same glyph as no change at all")
	}
}
