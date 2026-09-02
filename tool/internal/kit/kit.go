// Package kit fetches the deal-dev-kit repository at a pinned tag and parses
// its kit.yaml manifest into a set of installable artifacts.
package kit

import "strings"

// ProjectType is one of the three CRM DEAL repository shapes. The polyrepo has
// one repository per project, and destinations differ per type.
type ProjectType string

const (
	Backend ProjectType = "backend" // crm-deal-<service>-service, NestJS hexagonal
	Web     ProjectType = "web"     // crm-deal-web, React + Vite, feature-based
	Mobile  ProjectType = "mobile"  // crm-deal-mobile, React Native + Expo
)

// Artifact is one installable unit declared in kit.yaml.
type Artifact struct {
	ID        string            // "web/ui", "ui-kit/data-table"
	Type      string            // "skill" | "component" | "config"
	Group     string            // display grouping in the browser
	AppliesTo []ProjectType     // project types this artifact is valid for
	Src       string            // path inside the kit repo
	Dest      string            // destination template, e.g. "{ui}/data-table"
	Requires  []string          // other artifact IDs pulled in transitively
	NPM       map[string]string // npm dependency -> semver range
}

// Manifest is the parsed kit.yaml at a given kit version.
type Manifest struct {
	Version        string
	ProjectTypes   map[ProjectType]ProjectTypeSpec
	Profiles       map[ProjectType][]string
	ImportRewrites map[ProjectType]map[string]string
	Artifacts      []Artifact
}

// Rewrites returns the import rewrites for a project type, or nil.
func (m *Manifest) Rewrites(pt ProjectType) map[string]string {
	return m.ImportRewrites[pt]
}

// ProjectTypeSpec describes how to recognise a project type and where its
// well-known roots live, so kit.yaml never hardcodes a consumer's layout.
type ProjectTypeSpec struct {
	Match string            // repository name glob, e.g. "crm-deal-*-service"
	Roots map[string]string // "ui" -> "src/shared/ui"
}

// InstallName is the directory name a skill gets at its destination.
// Agents discover skills in a flat namespace, so the hierarchy that organises
// this repository is flattened here: "web/ui" -> "web-ui". It must match the
// `name` in the skill's SKILL.md frontmatter.
func (a Artifact) InstallName() string {
	return strings.ReplaceAll(a.ID, "/", "-")
}

// Supports reports whether the artifact may be installed in a project of the
// given type. An artifact with no AppliesTo is valid everywhere.
func (a Artifact) Supports(pt ProjectType) bool {
	if len(a.AppliesTo) == 0 {
		return true
	}
	for _, t := range a.AppliesTo {
		if t == pt {
			return true
		}
	}
	return false
}
