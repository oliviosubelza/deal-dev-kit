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

	// Every skill, command and agent artifact's frontmatter must be valid, or
	// it lands under one identity but is looked up (or invoked) under
	// another. Skills and agents declare `name`; commands have no `name`
	// field (their filename IS the identity) so they only need `description`.
	for _, a := range m.Artifacts {
		switch a.Type {
		case "skill":
			data, err := os.ReadFile(filepath.Join(root, a.Src, "SKILL.md"))
			if err != nil {
				t.Errorf("artifact %q: %v", a.ID, err)
				continue
			}
			if err := CheckFrontmatterName(data, a); err != nil {
				t.Error(err)
			}
		case "agent":
			data, err := os.ReadFile(filepath.Join(root, a.Src))
			if err != nil {
				t.Errorf("artifact %q: %v", a.ID, err)
				continue
			}
			if err := CheckFrontmatterName(data, a); err != nil {
				t.Error(err)
			}
		case "command":
			data, err := os.ReadFile(filepath.Join(root, a.Src))
			if err != nil {
				t.Errorf("artifact %q: %v", a.ID, err)
				continue
			}
			if err := CheckDescriptionPresent(data); err != nil {
				t.Errorf("artifact %q: %v", a.ID, err)
			}
		}
	}

	// Leaf names can collide where flattened ids cannot: two commands in
	// different groups that both apply to the same project type would
	// silently overwrite each other's installed file. This holds trivially
	// today (one generate-schema per project type) — TestDuplicateCommandLeaf*
	// in manifest_test.go proves the check itself fires on a synthetic
	// colliding pair, so this loop is not the only place the invariant is
	// exercised.
	for pt := range m.ProjectTypes {
		if first, second, collides := m.DuplicateCommandLeaf(pt); collides {
			t.Errorf("project type %q: commands %q and %q both install as %q", pt, first.ID, second.ID, second.LeafName())
		}
	}

	// The generalized loop above only exercises artifacts kit.yaml actually
	// declares. Assert the first-slice command/agent artifacts are wired in,
	// so the frontmatter checks above are proven against real content, not
	// just left dead until someone remembers to add them.
	wantMinTypes := map[string]int{"command": 3, "agent": 1}
	gotTypes := map[string]int{}
	for _, a := range m.Artifacts {
		gotTypes[a.Type]++
	}
	for typ, min := range wantMinTypes {
		if gotTypes[typ] < min {
			t.Errorf("expected at least %d artifact(s) of type %q wired into kit.yaml, got %d", min, typ, gotTypes[typ])
		}
	}
}

func ids2(as []Artifact) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}
