package tui

import (
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/plan"
)

// A merged JSON key is a change, so the plan screen has to count it. A plan
// that only sets the attribution keys would otherwise show an empty scope
// line.
func TestCountKindsCountsAMergedKeyApartFromFiles(t *testing.T) {
	created, overwritten, deleted, lines, keys := countKinds([]plan.Action{
		{Kind: plan.Create, Path: ".claude/persona.md"},
		{Kind: plan.MergeJSON, Path: ".claude/settings.json", Keys: []string{"attribution.commit"}},
	})
	if created != 1 || overwritten != 0 || deleted != 0 || lines != 0 {
		t.Errorf("files = %d/%d/%d, lines = %d, want 1/0/0 and 0", created, overwritten, deleted, lines)
	}
	if keys != 1 {
		t.Errorf("keys = %d, want 1", keys)
	}

	got := summary(created, overwritten, deleted, lines, keys)
	if !strings.Contains(got, "1 nuevos") || !strings.Contains(got, "1 ajuste") {
		t.Errorf("summary = %q, want both the file and the setting", got)
	}
	if got := summary(0, 0, 0, 0, 2); got != "2 ajustes" {
		t.Errorf("a settings-only plan summarises as %q, want %q", got, "2 ajustes")
	}
}

// The row must say which keys land, or it reads as "deal-kit is going to
// rewrite your settings.json".
func TestThePlanRowNamesTheKeysNotJustTheFile(t *testing.T) {
	out, _ := appendChangeSection(nil, "Skills y convenciones", []plan.Action{
		{Kind: plan.MergeJSON, Path: ".claude/settings.json",
			Keys: []string{"attribution.commit", "attribution.pr"}},
	}, 12)

	joined := ansi.ReplaceAllString(strings.Join(out, "\n"), "")
	if !strings.Contains(joined, ".claude/settings.json") ||
		!strings.Contains(joined, "attribution.commit") ||
		!strings.Contains(joined, "attribution.pr") {
		t.Errorf("the row does not name both the file and the keys:\n%s", joined)
	}
	if kindGlyph(plan.MergeJSON) == kindGlyph(plan.Unchanged) {
		t.Error("a merged setting renders with the same glyph as no change at all")
	}
}
