package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = `
version: 1
project_types:
  web:
    match: "crm-deal-web"
    roots: { src: src, ui: src/shared/ui }
  backend:
    match: "crm-deal-*-service"
    roots: { src: src }
profiles:
  web: [web/ui, ui-kit/button]
artifacts:
  - { id: web/ui, type: skill, applies_to: [web], src: skills/web/ui }
  - { id: backend/architecture, type: skill, applies_to: [backend], src: skills/backend/architecture }
  - id: ui-kit/base
    type: component
    applies_to: [web]
    src: ui-kit/lib
    dest: "{src}/shared/lib"
    npm: { clsx: ^2.1.1 }
  - id: ui-kit/button
    type: component
    applies_to: [web]
    src: ui-kit/components/ui/button.tsx
    dest: "{ui}/button.tsx"
    requires: [ui-kit/base]
    npm: { class-variance-authority: ^0.7.1 }
`

func TestParseManifestRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "unsupported version",
			yaml:    "version: 2",
			wantErr: "unsupported version",
		},
		{
			name: "duplicate artifact id",
			yaml: `
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: web/ui, type: skill, src: a }
  - { id: web/ui, type: skill, src: b }`,
			wantErr: "duplicate artifact",
		},
		{
			name: "component without dest",
			yaml: `
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: ui-kit/base, type: component, src: ui-kit/lib }`,
			wantErr: "has no dest",
		},
		{
			name: "skill declaring dest",
			yaml: `
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: web/ui, type: skill, src: skills/web/ui, dest: somewhere }`,
			wantErr: "must not declare dest",
		},
		{
			name: "unknown artifact type",
			yaml: `
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: web/ui, type: recipe, src: a }`,
			wantErr: "unknown type",
		},
		{
			name: "applies_to unknown project type",
			yaml: `
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: web/ui, type: skill, applies_to: [desktop], src: a }`,
			wantErr: "unknown project type",
		},
		{
			name: "requires unknown artifact",
			yaml: `
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: ui-kit/button, type: component, src: a, dest: b, requires: [ui-kit/ghost] }`,
			wantErr: "requires unknown artifact",
		},
		{
			name: "profile references artifact of another project type",
			yaml: `
version: 1
project_types:
  web: { match: crm-deal-web }
  backend: { match: "crm-deal-*-service" }
profiles:
  web: [backend/architecture]
artifacts:
  - { id: backend/architecture, type: skill, applies_to: [backend], src: a }`,
			wantErr: "does not apply to web",
		},
		{
			name: "project type without match pattern",
			yaml: `
version: 1
project_types: { web: {} }`,
			wantErr: "no match pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseManifestAcceptsValidInput(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(m.Artifacts); got != 4 {
		t.Errorf("artifacts = %d, want 4", got)
	}
	if got := len(m.ProjectTypes); got != 2 {
		t.Errorf("project types = %d, want 2", got)
	}
	if got := m.Profiles[Web]; len(got) != 2 {
		t.Errorf("web profile = %v, want 2 entries", got)
	}
}

func TestResolveOrdersDependenciesFirst(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}

	got, err := m.Resolve(Web, []string{"ui-kit/button"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"ui-kit/base", "ui-kit/button"}
	if len(got) != len(want) {
		t.Fatalf("resolved %d artifacts, want %d: %v", len(got), len(want), ids(got))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Errorf("position %d = %q, want %q (full order %v)", i, got[i].ID, want[i], ids(got))
		}
	}
}

func TestResolveDeduplicatesSharedDependencies(t *testing.T) {
	m, _ := ParseManifest([]byte(validManifest))
	got, err := m.Resolve(Web, []string{"ui-kit/base", "ui-kit/button"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("resolved %v, want ui-kit/base once", ids(got))
	}
}

func TestResolveRejectsArtifactFromAnotherProjectType(t *testing.T) {
	m, _ := ParseManifest([]byte(validManifest))
	_, err := m.Resolve(Web, []string{"backend/architecture"})
	if err == nil {
		t.Fatal("expected an error installing a backend skill into a web project")
	}
	if !strings.Contains(err.Error(), "does not apply to project type web") {
		t.Errorf("error = %q, want it to name the project type mismatch", err)
	}
}

func TestMatchProjectType(t *testing.T) {
	m, _ := ParseManifest([]byte(validManifest))
	tests := []struct {
		repoName string
		want     ProjectType
		wantOK   bool
	}{
		{"crm-deal-web", Web, true},
		{"crm-deal-orders-service", Backend, true},
		{"crm-deal-invoice-service", Backend, true},
		{"crm-deal-mobile", "", false},
		{"some-other-repo", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.repoName, func(t *testing.T) {
			got, ok := m.MatchProjectType(tt.repoName)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("MatchProjectType(%q) = (%q, %v), want (%q, %v)", tt.repoName, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestInstallNameFlattensHierarchy(t *testing.T) {
	tests := []struct{ id, want string }{
		{"web/ui", "web-ui"},
		{"general/pr-workflow", "general-pr-workflow"},
		{"ui-kit/data-table", "ui-kit-data-table"},
	}
	for _, tt := range tests {
		if got := (Artifact{ID: tt.id}).InstallName(); got != tt.want {
			t.Errorf("InstallName(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestSkillDir(t *testing.T) {
	a := Artifact{ID: "web/ui", Type: "skill"}
	if got, want := a.SkillDir(), ".claude/skills/web-ui"; got != want {
		t.Errorf("SkillDir() = %q, want %q", got, want)
	}
}

func TestNPMDepsUnionsAcrossArtifacts(t *testing.T) {
	m, _ := ParseManifest([]byte(validManifest))
	resolved, _ := m.Resolve(Web, []string{"ui-kit/button"})
	deps := NPMDeps(resolved)
	for _, name := range []string{"clsx", "class-variance-authority"} {
		if _, ok := deps[name]; !ok {
			t.Errorf("missing %q in %v", name, deps)
		}
	}
}

func TestLoadManifestFromDisk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kit.yaml"), []byte(validManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Artifacts) != 4 {
		t.Errorf("artifacts = %d, want 4", len(m.Artifacts))
	}
}

func ids(as []Artifact) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}

func TestGroupDefaultsToTheIDPrefix(t *testing.T) {
	m, err := ParseManifest([]byte(`
version: 1
project_types: { web: { match: crm-deal-web } }
artifacts:
  - { id: web/ui, type: skill, src: a }
  - { id: web/architecture, type: skill, src: b, group: "Web" }`))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.Artifact("web/ui")
	if a.Group != "web" {
		t.Errorf("group = %q, want the ID prefix %q", a.Group, "web")
	}
	b, _ := m.Artifact("web/architecture")
	if b.Group != "Web" {
		t.Errorf("group = %q, want the declared %q", b.Group, "Web")
	}
}
