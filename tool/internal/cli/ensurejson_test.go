package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// attributionManifest declares one config artifact with no src at all: it
// copies nothing and only guarantees two keys inside the project's own
// .claude/settings.json.
const attributionManifest = `
version: 2
project_types:
  web:
    match: "crm-deal-web"
    roots: { src: src, ui: src/shared/ui }
profiles:
  web: [general/attribution]
artifacts:
  - id: general/attribution
    type: config
    applies_to: [web]
    ensure_json:
      file: ".claude/settings.json"
      values: { attribution: { commit: "", pr: "" } }
`

func attributionProject(t *testing.T, settings string) (kitDir, projectDir string) {
	t.Helper()
	kitDir, projectDir = t.TempDir(), t.TempDir()
	write(t, filepath.Join(kitDir, "kit.yaml"), attributionManifest)
	write(t, filepath.Join(projectDir, "package.json"), "{}\n")
	if settings != "" {
		write(t, filepath.Join(projectDir, ".claude", "settings.json"), settings)
	}
	return kitDir, projectDir
}

func settingsJSON(t *testing.T, projectDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A project with no settings.json gets one holding only the kit's keys.
func TestInitCreatesSettingsWhenTheProjectHasNone(t *testing.T) {
	kitDir, projectDir := attributionProject(t, "")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true

	if err := Init(e, "web"); err != nil {
		t.Fatalf("Init() = %v", err)
	}
	want := "{\n  \"attribution\": {\n    \"commit\": \"\",\n    \"pr\": \"\"\n  }\n}\n"
	if got := settingsJSON(t, projectDir); got != want {
		t.Errorf("settings.json =\n%s\nwant\n%s", got, want)
	}
}

// The project's own permissions block is the reason ensure_json exists: a
// plain dest would block on this file forever, and overwriting it would throw
// the block away.
func TestInitKeepsTheProjectsOwnSettings(t *testing.T) {
	original := "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(git status:*)\"\n    ]\n  }\n}\n"
	kitDir, projectDir := attributionProject(t, original)
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true

	if err := Init(e, "web"); err != nil {
		t.Fatalf("Init() = %v\n%s", err, out)
	}
	got := settingsJSON(t, projectDir)
	if !strings.Contains(got, `"Bash(git status:*)"`) {
		t.Errorf("the project's permissions did not survive:\n%s", got)
	}
	if !strings.Contains(got, `"commit": ""`) || !strings.Contains(got, `"pr": ""`) {
		t.Errorf("the attribution keys were not merged in:\n%s", got)
	}
}

// --dry-run covers this mutation too, and it must name the keys rather than
// only the file, or the row reads as "deal-kit is going to rewrite your
// settings.json".
func TestDryRunNamesTheKeysAndWritesNothing(t *testing.T) {
	original := "{\n  \"model\": \"opus\"\n}\n"
	kitDir, projectDir := attributionProject(t, original)
	e, out := env(t, kitDir, projectDir)
	e.DryRun = true

	if err := Init(e, "web"); err != nil {
		t.Fatalf("Init() = %v", err)
	}
	if !strings.Contains(out.String(), "ajustar json") {
		t.Errorf("the plan does not name the action:\n%s", out)
	}
	if !strings.Contains(out.String(), "attribution.commit") {
		t.Errorf("the plan does not name the keys it would set:\n%s", out)
	}
	if got := settingsJSON(t, projectDir); got != original {
		t.Errorf("--dry-run wrote to settings.json: %q", got)
	}
}

// A second run must converge: no work planned, and status reports ok.
func TestASecondInitLeavesSettingsAloneAndReportsOK(t *testing.T) {
	kitDir, projectDir := attributionProject(t, "{\n  \"model\": \"opus\"\n}\n")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, "web"); err != nil {
		t.Fatal(err)
	}
	after := settingsJSON(t, projectDir)

	e2, out := env(t, kitDir, projectDir)
	e2.AssumeYes = true
	if err := Init(e2, "web"); err != nil {
		t.Fatal(err)
	}
	if got := settingsJSON(t, projectDir); got != after {
		t.Errorf("the second init changed settings.json:\n got %s\nwant %s", got, after)
	}
	if strings.Contains(out.String(), "ajustar json") {
		t.Errorf("the second init still plans the merge:\n%s", out)
	}

	e3, statusOut := env(t, kitDir, projectDir)
	if err := Status(e3); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusOut.String(), "general/attribution      ok") {
		t.Errorf("status should report the artifact ok:\n%s", statusOut)
	}
}

// The team edits its own settings all day. None of that is drift.
func TestStatusIgnoresEditsToTheRestOfSettings(t *testing.T) {
	kitDir, projectDir := attributionProject(t, "{\n  \"model\": \"opus\"\n}\n")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, "web"); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(projectDir, ".claude", "settings.json"),
		"{\n  \"model\": \"haiku\",\n  \"hooks\": {},\n  \"attribution\": {\n    \"commit\": \"\",\n    \"pr\": \"\"\n  }\n}\n")

	e2, out := env(t, kitDir, projectDir)
	if err := Status(e2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "general/attribution      ok") {
		t.Errorf("an unrelated edit to settings.json must not read as drift:\n%s", out)
	}
}

// Deleting the setting is the failure this artifact exists to make visible:
// without it every commit carries the Co-Authored-By trailer again.
func TestStatusReportsAMissingSettingAndInitRestoresIt(t *testing.T) {
	kitDir, projectDir := attributionProject(t, "{\n  \"model\": \"opus\"\n}\n")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, "web"); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(projectDir, ".claude", "settings.json"), "{\n  \"model\": \"opus\"\n}\n")

	e2, out := env(t, kitDir, projectDir)
	if err := Status(e2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "FALTA AJUSTE") {
		t.Errorf("status does not notice the missing setting:\n%s", out)
	}
	if !strings.Contains(out.String(), ".claude/settings.json") {
		t.Errorf("status does not name the file:\n%s", out)
	}

	e3, _ := env(t, kitDir, projectDir)
	e3.AssumeYes = true
	if err := Init(e3, "web"); err != nil {
		t.Fatal(err)
	}
	if got := settingsJSON(t, projectDir); !strings.Contains(got, `"pr": ""`) {
		t.Errorf("init did not restore the setting:\n%s", got)
	}
}

// A value the project deliberately set is not overwritten: the run stops and
// says which key needs a human.
func TestInitRefusesToOverwriteASettingTheProjectChose(t *testing.T) {
	original := "{\n  \"attribution\": {\n    \"commit\": \"Co-Authored-By: Claude\"\n  }\n}\n"
	kitDir, projectDir := attributionProject(t, original)
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true

	if err := Init(e, "web"); err == nil {
		t.Fatalf("Init succeeded over a value the project set:\n%s", out)
	}
	if !strings.Contains(out.String(), "attribution.commit") {
		t.Errorf("the report does not name the conflicting key:\n%s", out)
	}
	if got := settingsJSON(t, projectDir); got != original {
		t.Errorf("settings.json was written anyway:\n%s", got)
	}
}
