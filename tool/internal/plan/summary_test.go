package plan

import "testing"

func act(kind Kind, path string) Action { return Action{Kind: kind, Path: path} }

func TestSummarizeGroupsByDirectory(t *testing.T) {
	got := Summarize([]Action{
		act(Create, "src/shared/ui/button.tsx"),
		act(Create, "src/shared/ui/input.tsx"),
		act(Overwrite, "src/shared/lib/utils.ts"),
		act(Create, "src/theme.css"),
	})

	want := map[string]DirSummary{
		"src/shared/ui":  {Dir: "src/shared/ui", Created: 2},
		"src/shared/lib": {Dir: "src/shared/lib", Overwritten: 1},
		"src":            {Dir: "src", Created: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for _, g := range got {
		w, ok := want[g.Dir]
		if !ok {
			t.Errorf("unexpected row %q", g.Dir)
			continue
		}
		if g != w {
			t.Errorf("%q = %+v, want %+v", g.Dir, g, w)
		}
	}
}

func TestSummarizeIgnoresNonChanges(t *testing.T) {
	got := Summarize([]Action{
		act(Unchanged, "src/a.ts"),
		act(Blocked, "src/b.ts"),
	})
	if len(got) != 0 {
		t.Errorf("got %+v, want nothing: unchanged and blocked files are not changes", got)
	}
}

func TestSummarizeReportsRootFilesAsThemselves(t *testing.T) {
	got := Summarize([]Action{act(Create, "deal-kit.lock")})
	if len(got) != 1 || got[0].Dir != "deal-kit.lock" {
		t.Errorf("got %+v, want the file named directly rather than \".\"", got)
	}
}

func TestSummarizeFoldsUntilItFits(t *testing.T) {
	// One directory per component would produce a report nobody reads.
	var actions []Action
	for _, name := range []string{
		"accordion", "alert", "avatar", "badge", "button", "card",
		"dialog", "drawer", "input", "select", "table", "tabs",
	} {
		actions = append(actions, act(Create, "src/shared/ui/"+name+"/index.tsx"))
	}

	got := Summarize(actions)
	if len(got) > maxSummaryRows {
		t.Fatalf("got %d rows, want at most %d: %+v", len(got), maxSummaryRows, got)
	}
	if len(got) != 1 || got[0].Dir != "src/shared/ui" {
		t.Errorf("got %+v, want everything folded under src/shared/ui", got)
	}
	if got[0].Created != len(actions) {
		t.Errorf("folded count = %d, want %d", got[0].Created, len(actions))
	}
}

func TestSummarizeKeepsDetailWhenItFits(t *testing.T) {
	got := Summarize([]Action{
		act(Create, "src/shared/ui/button.tsx"),
		act(Create, ".claude/skills/web-ui/SKILL.md"),
	})
	if len(got) != 2 {
		t.Fatalf("got %+v, want both directories kept", got)
	}
}

func TestDirSummaryTotal(t *testing.T) {
	d := DirSummary{Created: 2, Overwritten: 3, Deleted: 1}
	if d.Total() != 6 {
		t.Errorf("Total() = %d, want 6", d.Total())
	}
}

func TestSummarizeFoldsSiblingSingleFileDirectories(t *testing.T) {
	// Four skills, one file each, should read as one line about the skills
	// directory rather than four lines that each say "+1".
	got := Summarize([]Action{
		act(Create, ".claude/skills/general-conventions/SKILL.md"),
		act(Create, ".claude/skills/general-pr-workflow/SKILL.md"),
		act(Create, ".claude/skills/web-architecture/SKILL.md"),
		act(Create, ".claude/skills/web-ui/SKILL.md"),
		act(Create, "src/shared/lib/utils.ts"),
		act(Create, "src/shared/lib/storage/adapter.ts"),
	})

	for _, r := range got {
		if r.Dir == ".claude/skills" {
			if r.Created != 4 {
				t.Errorf(".claude/skills = %d created, want 4", r.Created)
			}
			return
		}
	}
	t.Errorf("the skills were not folded into one row: %+v", got)
}

func TestSummarizeKeepsADirectoryThatEarnsItsRow(t *testing.T) {
	// A lone single-file directory has no sibling to merge with, so folding it
	// would move it up to a parent that says less.
	got := Summarize([]Action{
		act(Create, "src/theme.css"),
		act(Create, "src/shared/lib/utils.ts"),
		act(Create, "src/shared/lib/storage/adapter.ts"),
	})
	for _, r := range got {
		if r.Dir == "." || r.Dir == "" {
			t.Errorf("folded up to the root, which names nothing: %+v", got)
		}
	}
	if len(got) != 2 {
		t.Errorf("got %+v, want src and src/shared/lib kept apart", got)
	}
}
