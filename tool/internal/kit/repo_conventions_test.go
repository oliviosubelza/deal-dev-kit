package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTeamConventionsRequireACoAuthorTrailer pins the owner-added rule that an
// AI agent attributes its commits with a Co-Authored-By trailer. This
// convention is not from the Aug 2026 coordinator briefing — it is a deliberate
// repo-owner addition, and is the explicit exception to "only what the briefing
// states" (see HANDOFF.md).
//
// Production change that makes this fail: deleting (or never adding) the
// Co-Authored-By guidance from skills/general/conventions/SKILL.md. The
// placement check fails if the rule is moved above the Conventional Commits
// section it belongs next to.
func TestTeamConventionsRequireACoAuthorTrailer(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "skills", "general", "conventions", "SKILL.md")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read team-conventions skill: %v", err)
	}
	body := string(data)

	// The git trailer key, shown as a trailer (with its colon), is the
	// falsifiable heart of the rule.
	if !strings.Contains(body, "Co-Authored-By:") {
		t.Fatalf("%s does not carry the Co-Authored-By trailer rule for AI commits", path)
	}

	// The rule lives next to the Conventional Commits guidance. Anchor on the
	// bold heading in the section body, not the plain mention in the
	// frontmatter description.
	section := strings.Index(body, "**Conventional Commits**")
	if section < 0 {
		t.Fatalf("%s no longer has a Conventional Commits section to anchor the rule to", path)
	}
	if strings.Index(body, "Co-Authored-By:") < section {
		t.Errorf("the Co-Authored-By rule should follow the Conventional Commits guidance, not precede it")
	}
}
