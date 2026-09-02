package kit

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path"
)

// rawManifest mirrors kit.yaml on disk. It is separate from Manifest so the
// wire format can change without leaking yaml tags into the domain types.
type rawManifest struct {
	Version        int                          `yaml:"version"`
	ProjectTypes   map[string]rawProjectType    `yaml:"project_types"`
	Profiles       map[string][]string          `yaml:"profiles"`
	ImportRewrites map[string]map[string]string `yaml:"import_rewrites"`
	Artifacts      []rawArtifact                `yaml:"artifacts"`
}
type rawProjectType struct {
	Match string            `yaml:"match"`
	Roots map[string]string `yaml:"roots"`
}
type rawArtifact struct {
	ID        string            `yaml:"id"`
	Type      string            `yaml:"type"`
	AppliesTo []string          `yaml:"applies_to"`
	Src       string            `yaml:"src"`
	Dest      string            `yaml:"dest"`
	Requires  []string          `yaml:"requires"`
	NPM       map[string]string `yaml:"npm"`
}

// ParseManifest reads and validates a kit.yaml.
func ParseManifest(data []byte) (*Manifest, error) {
	var raw rawManifest
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("kit.yaml: %w", err)
	}
	if raw.Version != 1 {
		return nil, fmt.Errorf("kit.yaml: unsupported version %d (expected 1)", raw.Version)
	}
	m := &Manifest{
		ProjectTypes:   make(map[ProjectType]ProjectTypeSpec, len(raw.ProjectTypes)),
		Profiles:       make(map[ProjectType][]string, len(raw.Profiles)),
		ImportRewrites: make(map[ProjectType]map[string]string, len(raw.ImportRewrites)),
	}
	for name, spec := range raw.ProjectTypes {
		if spec.Match == "" {
			return nil, fmt.Errorf("kit.yaml: project type %q has no match pattern", name)
		}
		m.ProjectTypes[ProjectType(name)] = ProjectTypeSpec{Match: spec.Match, Roots: spec.Roots}
	}
	seen := make(map[string]bool, len(raw.Artifacts))
	for _, ra := range raw.Artifacts {
		if ra.ID == "" {
			return nil, fmt.Errorf("kit.yaml: artifact with no id")
		}
		if seen[ra.ID] {
			return nil, fmt.Errorf("kit.yaml: duplicate artifact %q", ra.ID)
		}
		seen[ra.ID] = true
		if ra.Src == "" {
			return nil, fmt.Errorf("kit.yaml: artifact %q has no src", ra.ID)
		}
		switch ra.Type {
		case "skill", "component", "config":
		default:
			return nil, fmt.Errorf("kit.yaml: artifact %q has unknown type %q", ra.ID, ra.Type)
		}
		// A skill's destination is derived from its flattened ID, so declaring
		// one would create a second, conflicting source of truth.
		if ra.Type == "skill" && ra.Dest != "" {
			return nil, fmt.Errorf("kit.yaml: skill %q must not declare dest", ra.ID)
		}
		if ra.Type == "component" && ra.Dest == "" {
			return nil, fmt.Errorf("kit.yaml: component %q has no dest", ra.ID)
		}
		a := Artifact{
			ID: ra.ID, Type: ra.Type, Src: ra.Src, Dest: ra.Dest,
			Requires: ra.Requires, NPM: ra.NPM,
		}
		for _, t := range ra.AppliesTo {
			pt := ProjectType(t)
			if _, ok := m.ProjectTypes[pt]; !ok {
				return nil, fmt.Errorf("kit.yaml: artifact %q applies_to unknown project type %q", ra.ID, t)
			}
			a.AppliesTo = append(a.AppliesTo, pt)
		}
		m.Artifacts = append(m.Artifacts, a)
	}
	for name, ids := range raw.Profiles {
		pt := ProjectType(name)
		if _, ok := m.ProjectTypes[pt]; !ok {
			return nil, fmt.Errorf("kit.yaml: profile %q is not a known project type", name)
		}
		for _, id := range ids {
			a, ok := m.Artifact(id)
			if !ok {
				return nil, fmt.Errorf("kit.yaml: profile %q references unknown artifact %q", name, id)
			}
			if !a.Supports(pt) {
				return nil, fmt.Errorf("kit.yaml: profile %q includes %q, which does not apply to %s", name, id, name)
			}
		}
		m.Profiles[pt] = ids
	}
	for name, rules := range raw.ImportRewrites {
		pt := ProjectType(name)
		if _, ok := m.ProjectTypes[pt]; !ok {
			return nil, fmt.Errorf("kit.yaml: import_rewrites names unknown project type %q", name)
		}
		for from := range rules {
			if from == "" {
				return nil, fmt.Errorf("kit.yaml: import_rewrites for %q has an empty prefix", name)
			}
		}
		m.ImportRewrites[pt] = rules
	}

	for _, a := range m.Artifacts {
		for _, req := range a.Requires {
			if _, ok := m.Artifact(req); !ok {
				return nil, fmt.Errorf("kit.yaml: artifact %q requires unknown artifact %q", a.ID, req)
			}
		}
	}
	return m, nil
}

// LoadManifest reads kit.yaml from a checked-out kit directory.
func LoadManifest(kitDir string) (*Manifest, error) {
	data, err := os.ReadFile(path.Join(kitDir, "kit.yaml"))
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}

// Artifact looks up an artifact by ID.
func (m *Manifest) Artifact(id string) (Artifact, bool) {
	for _, a := range m.Artifacts {
		if a.ID == id {
			return a, true
		}
	}
	return Artifact{}, false
}

// Resolve expands the given artifact IDs into the full install set for a
// project type, following Requires transitively. The result is ordered
// dependencies-first so a caller can install it as-is.
func (m *Manifest) Resolve(pt ProjectType, ids []string) ([]Artifact, error) {
	var out []Artifact
	done := make(map[string]bool)
	inProgress := make(map[string]bool)
	var visit func(id string, from string) error
	visit = func(id, from string) error {
		if done[id] {
			return nil
		}
		if inProgress[id] {
			return fmt.Errorf("dependency cycle at %q", id)
		}
		a, ok := m.Artifact(id)
		if !ok {
			return fmt.Errorf("unknown artifact %q", id)
		}
		if !a.Supports(pt) {
			if from != "" {
				return fmt.Errorf("%q requires %q, which does not apply to project type %s", from, id, pt)
			}
			return fmt.Errorf("%q does not apply to project type %s", id, pt)
		}
		inProgress[id] = true
		for _, req := range a.Requires {
			if err := visit(req, id); err != nil {
				return err
			}
		}
		delete(inProgress, id)
		done[id] = true
		out = append(out, a)
		return nil
	}
	for _, id := range ids {
		if err := visit(id, ""); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// MatchProjectType returns the project type whose match pattern accepts the
// given repository directory name.
func (m *Manifest) MatchProjectType(repoName string) (ProjectType, bool) {
	for pt, spec := range m.ProjectTypes {
		if ok, _ := path.Match(spec.Match, repoName); ok {
			return pt, true
		}
	}
	return "", false
}

// NPMDeps collects the union of npm dependencies for a set of artifacts.
func NPMDeps(artifacts []Artifact) map[string]string {
	deps := make(map[string]string)
	for _, a := range artifacts {
		for name, rng := range a.NPM {
			deps[name] = rng
		}
	}
	return deps
}

// SkillDir is the destination directory for a skill, relative to the project
// root: .claude/skills/<flattened-id>.
func (a Artifact) SkillDir() string {
	return path.Join(".claude", "skills", a.InstallName())
}
