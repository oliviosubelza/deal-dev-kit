package kit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRepositoryManifestIsValid parses the kit.yaml that actually ships in this
// repository. It fails CI on a malformed manifest before anyone installs it.
func TestRepositoryManifestIsValid(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	m, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("repository kit.yaml is invalid: %v", err)
	}

	if len(m.ProjectTypes) == 0 {
		t.Fatal("no project types declared")
	}

	// Every project type must have a profile, or `deal-kit init` has nothing
	// to install for a repository of that type.
	for pt := range m.ProjectTypes {
		if len(m.Profiles[pt]) == 0 {
			t.Errorf("project type %q has no profile", pt)
		}
	}

	// Every profile must resolve cleanly: this is exactly what `init` runs.
	for pt, ids := range m.Profiles {
		resolved, err := m.Resolve(pt, ids)
		if err != nil {
			t.Errorf("profile %q does not resolve: %v", pt, err)
			continue
		}
		t.Logf("profile %-8s resolves to %d artifacts: %v", pt, len(resolved), ids2(resolved))
	}

	// Every artifact's src must exist in the repository.
	for _, a := range m.Artifacts {
		if _, err := os.Stat(filepath.Join(root, a.Src)); err != nil {
			t.Errorf("artifact %q: src %q does not exist", a.ID, a.Src)
		}
	}

	// A skill's frontmatter name must match its flattened install name, or it
	// lands in a directory the agent looks up under a different name.
	for _, a := range m.Artifacts {
		if a.Type != "skill" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, a.Src, "SKILL.md"))
		if err != nil {
			t.Errorf("artifact %q: %v", a.ID, err)
			continue
		}
		want := "name: " + a.InstallName()
		if !contains(string(data), want) {
			t.Errorf("artifact %q: SKILL.md frontmatter must declare %q", a.ID, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func ids2(as []Artifact) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}
