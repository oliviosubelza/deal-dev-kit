package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTeamConventionsForbidAIAttribution pins the rule that a commit is
// authored by the person who owns the change and carries no AI attribution.
//
// This test previously asserted the opposite. It was introduced alongside a
// SKILL.md paragraph that *required* a Co-Authored-By trailer, which inverted
// the convention it was meant to protect and then locked the inversion in: CI
// failed if anyone removed it. Both are corrected here.
//
// Production change that makes this fail: reintroducing Co-Authored-By (or any
// other AI attribution) guidance as something to do, rather than to avoid.
func TestTeamConventionsForbidAIAttribution(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	path := filepath.Join(root, "skills", "general", "conventions", "SKILL.md")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read team-conventions skill: %v", err)
	}
	body := string(data)

	// The prohibition has to be stated, not merely implied by silence: an
	// agent that never reads the rule is the one that adds the trailer.
	if !strings.Contains(body, "Never add a `Co-Authored-By` trailer") {
		t.Fatalf("%s no longer forbids AI attribution in commits", path)
	}

	// The rule belongs next to the Conventional Commits guidance, where an
	// agent about to write a commit message is already reading.
	section := strings.Index(body, "**Conventional Commits**")
	if section < 0 {
		t.Fatalf("%s no longer has a Conventional Commits section to anchor the rule to", path)
	}
	if strings.Index(body, "Never add a `Co-Authored-By` trailer") < section {
		t.Errorf("the AI-attribution rule should follow the Conventional Commits guidance, not precede it")
	}

	// The trailer may appear only as the counter-example marked "never". Any
	// other occurrence means the prohibition has drifted back into a recipe.
	const trailer = "Co-Authored-By: Claude"
	if n := strings.Count(body, trailer); n > 1 {
		t.Errorf("%s shows %q %d times; it may appear once, as the marked counter-example", path, trailer, n)
	}
}
