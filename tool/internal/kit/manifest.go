package kit

import (
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
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
	Group     string            `yaml:"group"`
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
		case "skill", "component", "config", "command", "agent":
		default:
			return nil, fmt.Errorf("kit.yaml: artifact %q has unknown type %q", ra.ID, ra.Type)
		}
		// A skill's, command's, or agent's destination is derived from its
		// flattened ID, so declaring one would create a second, conflicting
		// source of truth.
		if (ra.Type == "skill" || ra.Type == "command" || ra.Type == "agent") && ra.Dest != "" {
			return nil, fmt.Errorf("kit.yaml: %s %q must not declare dest", ra.Type, ra.ID)
		}
		if ra.Type == "component" && ra.Dest == "" {
			return nil, fmt.Errorf("kit.yaml: component %q has no dest", ra.ID)
		}
		a := Artifact{
			ID: ra.ID, Type: ra.Type, Group: ra.Group, Src: ra.Src, Dest: ra.Dest,
			Requires: ra.Requires, NPM: ra.NPM,
		}
		if a.Group == "" {
			// Fall back to the ID's first segment so an artifact always lands
			// under some heading rather than silently disappearing.
			a.Group = strings.SplitN(a.ID, "/", 2)[0]
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

// DuplicateCommandLeaf reports the first two installable command artifacts
// for the given project type whose leaf name collides, or ok=false if none
// collide. Leaf names can collide where flattened ids cannot: two commands
// declared under different groups that both apply to the same project type
// would otherwise silently overwrite each other's file at install time.
func (m *Manifest) DuplicateCommandLeaf(pt ProjectType) (first, second Artifact, ok bool) {
	seen := make(map[string]Artifact)
	for _, a := range m.Artifacts {
		if a.Type != "command" || !a.Supports(pt) {
			continue
		}
		leaf := a.LeafName()
		if prev, exists := seen[leaf]; exists {
			return prev, a, true
		}
		seen[leaf] = a
	}
	return Artifact{}, Artifact{}, false
}

// SkillDir is the destination directory for a skill, relative to the project
// root: .claude/skills/<flattened-id>.
func (a Artifact) SkillDir() string {
	return path.Join(".claude", "skills", a.InstallName())
}

// CommandFile is the destination file for a command, relative to the project
// root: .claude/commands/<leaf-name>.md. Unlike SkillDir and AgentFile, this
// uses the artifact's leaf name, not its flattened id: a command's filename
// is the string a human types to invoke it (e.g. "/generate-schema"), and a
// group prefix like "web-" would contradict that string everywhere it is
// already documented and typed.
func (a Artifact) CommandFile() string {
	return path.Join(".claude", "commands", a.LeafName()+".md")
}

// AgentFile is the destination file for an agent, relative to the project
// root: .claude/agents/<flattened-id>.md.
func (a Artifact) AgentFile() string {
	return path.Join(".claude", "agents", a.InstallName()+".md")
}
