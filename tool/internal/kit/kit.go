// Package kit fetches the deal-dev-kit repository at a pinned tag and parses
// its kit.yaml manifest into a set of installable artifacts.
package kit

import (
	"path"
	"slices"
	"sort"
	"strings"
)

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
	Type      string            // "skill" | "component" | "config" | "command" | "agent"
	Group     string            // display grouping in the browser
	AppliesTo []ProjectType     // project types this artifact is valid for
	Src       string            // path inside the kit repo
	Dest      string            // destination template, e.g. "{ui}/data-table"
	Requires  []string          // other artifact IDs pulled in transitively
	NPM       map[string]string // npm dependency -> semver range

	// EnsureLine, when set, is a single line the artifact guarantees exists
	// in a file the kit does NOT own. Nil for every other artifact.
	EnsureLine *EnsuredLine

	// EnsureJSON, when set, are the JSON keys the artifact guarantees inside a
	// JSON file the kit does NOT own. Nil for every other artifact.
	EnsureJSON *EnsuredJSON
}

// EnsuredLine is the kit.yaml `ensure_line` block: one exact line that must be
// present in File. It exists because some artifacts only take effect once a
// project-owned file references them — the persona is copied to
// .claude/persona.md, but Claude Code never loads it until CLAUDE.md imports
// it. Copying a whole CLAUDE.md is not an option: the project owns that file.
//
// The surface is deliberately one file and one literal line. It is not a
// templating system: anything richer would make the kit a second author of a
// file it does not own, which is exactly what the ownership rules forbid.
type EnsuredLine struct {
	File string // destination template, resolved like any other dest
	Line string // the exact line, matched and written verbatim
}

// EnsuredJSON is the kit.yaml `ensure_json` block: the keys that must be
// present, with those values, inside a JSON file the kit does NOT own. It is
// the same idea as EnsuredLine, for a file whose format is JSON rather than
// lines of text.
//
// It exists because .claude/settings.json belongs to the project — it carries
// its permissions, hooks and model — while some kit behaviour is only
// configurable there. Claude Code appends a `Co-Authored-By: Claude` trailer
// to every commit unless `attribution` turns it off in settings.json, and no
// amount of documentation overrides it: the harness injects that instruction
// itself. Copying a whole settings.json would destroy the project's own keys,
// and refusing to touch the file would leave the trailer on forever.
//
// Values is a nested map matching the shape of the JSON to guarantee. Only
// its leaves are compared and written; every sibling key in the file is left
// exactly where it was.
type EnsuredJSON struct {
	File   string         // destination template, resolved like any other dest
	Values map[string]any // nested; normalised through JSON at parse time
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

// ProjectTypeNames is every project type kit.yaml declares, sorted. It is the
// single source for "which types exist": a second hardcoded list next to this
// one is how the two drift apart.
func (m *Manifest) ProjectTypeNames() []string {
	out := make([]string, 0, len(m.ProjectTypes))
	for pt := range m.ProjectTypes {
		out = append(out, string(pt))
	}
	sort.Strings(out)
	return out
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

// LeafName is the last segment of the artifact's ID, dropping the group
// prefix entirely: "web/generate-schema" -> "generate-schema". Placed beside
// InstallName because both derive an install-facing string from the ID, and
// keeping them together documents that installation naming has two distinct
// rules (flatten vs. leaf) depending on artifact type — not one rule with an
// exception. Used by CommandFile, since a command's filename is the string a
// human types to invoke it, unlike a skill (loaded by description) or an
// agent (referenced by the orchestrator).
func (a Artifact) LeafName() string {
	return path.Base(a.ID)
}

// Supports reports whether the artifact may be installed in a project of the
// given type. An artifact with no AppliesTo is valid everywhere.
func (a Artifact) Supports(pt ProjectType) bool {
	if len(a.AppliesTo) == 0 {
		return true
	}
	return slices.Contains(a.AppliesTo, pt)
}
