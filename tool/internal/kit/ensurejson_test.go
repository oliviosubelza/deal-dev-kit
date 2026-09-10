package kit

import (
	"path/filepath"
	"strings"
	"testing"
)

// ensureJSONManifest wraps an ensure_json block in the smallest manifest that
// parses. The artifact declares no src on purpose: an artifact that only
// guarantees keys inside a file the project owns has no file to copy.
func ensureJSONManifest(block string) string {
	return `
version: 2
project_types: { web: { match: crm-deal-web } }
profiles:
  web: [general/attribution]
artifacts:
  - id: general/attribution
    type: config
    applies_to: [web]
    ensure_json:
` + block
}

func TestParseManifestReadsEnsureJSON(t *testing.T) {
	m, err := ParseManifest([]byte(ensureJSONManifest(
		"      file: \".claude/settings.json\"\n" +
			"      values:\n" +
			"        attribution:\n" +
			"          commit: \"\"\n" +
			"          pr: \"\"\n")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, ok := m.Artifact("general/attribution")
	if !ok {
		t.Fatal("general/attribution missing")
	}
	if a.Src != "" {
		t.Errorf("Src = %q, want empty: this artifact copies nothing", a.Src)
	}
	if a.EnsureJSON == nil {
		t.Fatal("EnsureJSON is nil")
	}
	if a.EnsureJSON.File != ".claude/settings.json" {
		t.Errorf("File = %q", a.EnsureJSON.File)
	}
	// The values must come back shaped for JSON, not for YAML: nested maps are
	// map[string]any, which is what the merge compares and writes.
	nested, ok := a.EnsureJSON.Values["attribution"].(map[string]any)
	if !ok {
		t.Fatalf("attribution = %T, want map[string]any", a.EnsureJSON.Values["attribution"])
	}
	if nested["commit"] != "" || nested["pr"] != "" {
		t.Errorf("values = %v, want both keys empty strings", nested)
	}
}

// A YAML integer decodes to int, which JSON cannot compare against a file's
// value until it is normalised. Parsing must do that normalisation.
func TestParseManifestNormalisesEnsureJSONNumbers(t *testing.T) {
	m, err := ParseManifest([]byte(ensureJSONManifest(
		"      file: \".claude/settings.json\"\n      values: { timeout: 30 }\n")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, _ := m.Artifact("general/attribution")
	if got, ok := a.EnsureJSON.Values["timeout"].(float64); !ok || got != 30 {
		t.Errorf("timeout = %#v, want the JSON number 30", a.EnsureJSON.Values["timeout"])
	}
}

func TestParseManifestRejectsAnIncompleteEnsureJSON(t *testing.T) {
	tests := []struct {
		name    string
		block   string
		wantErr string
	}{
		{
			name:    "no file",
			block:   "      values: { attribution: { commit: \"\" } }\n",
			wantErr: `el ensure_json del artefacto "general/attribution" no tiene file`,
		},
		{
			name:    "no values",
			block:   "      file: \".claude/settings.json\"\n",
			wantErr: `el ensure_json del artefacto "general/attribution" no tiene values`,
		},
		{
			name:    "empty values",
			block:   "      file: \".claude/settings.json\"\n      values: {}\n",
			wantErr: `el ensure_json del artefacto "general/attribution" no tiene values`,
		},
		{
			name:    "empty nested object guarantees nothing",
			block:   "      file: \".claude/settings.json\"\n      values: { attribution: {} }\n",
			wantErr: `declara "attribution" sin ninguna clave debajo`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(ensureJSONManifest(tt.block)))
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// An artifact with neither a src nor anything to ensure installs nothing at
// all, which is a manifest mistake and not a valid declaration.
func TestParseManifestStillRequiresSrcWhenThereIsNothingToEnsure(t *testing.T) {
	_, err := ParseManifest([]byte(`
version: 2
project_types: { web: { match: crm-deal-web } }
profiles:
  web: [general/nothing]
artifacts:
  - id: general/nothing
    type: config
    applies_to: [web]
    dest: ".claude/nothing.md"
`))
	if err == nil || !strings.Contains(err.Error(), `el artefacto "general/nothing" no tiene src`) {
		t.Fatalf("error = %v, want it to demand a src", err)
	}
}

// The kit.yaml that actually ships must carry the attribution artifact for
// every project type, or the commit trailer stays on wherever it is missing.
func TestRepositoryManifestShipsAttributionToEveryProjectType(t *testing.T) {
	m, err := LoadManifest(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("repository kit.yaml is invalid: %v", err)
	}
	a, ok := m.Artifact("general/attribution")
	if !ok {
		t.Fatal("kit.yaml does not declare general/attribution")
	}
	if a.EnsureJSON == nil || a.EnsureJSON.File != ".claude/settings.json" {
		t.Fatalf("general/attribution does not ensure .claude/settings.json: %+v", a.EnsureJSON)
	}
	for pt := range m.ProjectTypes {
		found := false
		for _, id := range m.Profiles[pt] {
			if id == "general/attribution" {
				found = true
			}
		}
		if !found {
			t.Errorf("profile %q does not install general/attribution", pt)
		}
	}
}
