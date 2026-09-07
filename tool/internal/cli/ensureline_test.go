package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// personaManifest declares one config artifact that both copies a file and
// guarantees an import line inside the project's own CLAUDE.md.
const personaManifest = `
version: 2
project_types:
  web:
    match: "crm-deal-web"
    roots: { src: src, ui: src/shared/ui }
profiles:
  web: [general/persona]
artifacts:
  - id: general/persona
    type: config
    applies_to: [web]
    src: config/persona.md
    dest: ".claude/persona.md"
    ensure_line: { file: "CLAUDE.md", line: "@.claude/persona.md" }
`

const personaImport = "@.claude/persona.md"

// personaProject builds a kit checkout and a bare project. claudeMD, when
// non-empty, becomes the project's pre-existing CLAUDE.md.
func personaProject(t *testing.T, claudeMD string) (kitDir, projectDir string) {
	t.Helper()
	kitDir, projectDir = t.TempDir(), t.TempDir()
	write(t, filepath.Join(kitDir, "kit.yaml"), personaManifest)
	write(t, filepath.Join(kitDir, "config", "persona.md"), "be brief\n")
	write(t, filepath.Join(projectDir, "package.json"), "{}\n")
	if claudeMD != "" {
		write(t, filepath.Join(projectDir, "CLAUDE.md"), claudeMD)
	}
	return kitDir, projectDir
}

func claudeMD(t *testing.T, projectDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The import is what makes the installed persona take effect, so init must not
// leave it to a human to remember.
func TestInitAddsTheImportToAnExistingClaudeMD(t *testing.T) {
	original := "# CRM DEAL\n\n- usar pnpm\n"
	kitDir, projectDir := personaProject(t, original)
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true

	if err := Init(e, "web"); err != nil {
		t.Fatalf("Init() = %v", err)
	}
	if got := claudeMD(t, projectDir); got != original+personaImport+"\n" {
		t.Errorf("CLAUDE.md = %q, want the original plus the import\noutput:\n%s", got, out)
	}
}

func TestInitCreatesClaudeMDWhenTheProjectHasNone(t *testing.T) {
	kitDir, projectDir := personaProject(t, "")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true

	if err := Init(e, "web"); err != nil {
		t.Fatalf("Init() = %v", err)
	}
	if got := claudeMD(t, projectDir); got != personaImport+"\n" {
		t.Errorf("CLAUDE.md = %q, want just the import", got)
	}
}

// --dry-run has to cover this mutation too: it lands in a file the project
// owns, which is the one place a surprise write is least recoverable.
func TestDryRunShowsTheImportAndWritesNothing(t *testing.T) {
	kitDir, projectDir := personaProject(t, "# CRM DEAL\n")
	e, out := env(t, kitDir, projectDir)
	e.DryRun = true

	if err := Init(e, "web"); err != nil {
		t.Fatalf("Init() = %v", err)
	}
	if !strings.Contains(out.String(), "agregar línea") || !strings.Contains(out.String(), personaImport) {
		t.Errorf("the plan does not name the line it would add:\n%s", out)
	}
	if got := claudeMD(t, projectDir); got != "# CRM DEAL\n" {
		t.Errorf("--dry-run wrote to CLAUDE.md: %q", got)
	}
}

// Running init twice must converge, and the second run must not report work.
func TestASecondInitLeavesClaudeMDAloneAndReportsOK(t *testing.T) {
	kitDir, projectDir := personaProject(t, "# CRM DEAL\n")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, "web"); err != nil {
		t.Fatal(err)
	}
	after := claudeMD(t, projectDir)

	e2, out := env(t, kitDir, projectDir)
	e2.AssumeYes = true
	if err := Init(e2, "web"); err != nil {
		t.Fatal(err)
	}
	if got := claudeMD(t, projectDir); got != after {
		t.Errorf("the second init changed CLAUDE.md:\n got %q\nwant %q", got, after)
	}
	if strings.Contains(out.String(), "agregar línea") {
		t.Errorf("the second init still plans the line:\n%s", out)
	}

	e3, statusOut := env(t, kitDir, projectDir)
	if err := Status(e3); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusOut.String(), "general/persona          ok") {
		t.Errorf("status should report the artifact ok:\n%s", statusOut)
	}
}

// The team edits CLAUDE.md all day. None of that is drift, and a status that
// cried "changed" on every run would train people to stop reading it.
func TestStatusIgnoresEditsToTheRestOfClaudeMD(t *testing.T) {
	kitDir, projectDir := personaProject(t, "# CRM DEAL\n")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, "web"); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(projectDir, "CLAUDE.md"),
		"# CRM DEAL\n"+personaImport+"\n\n## Reglas nuevas que escribió el equipo\n")

	e2, out := env(t, kitDir, projectDir)
	if err := Status(e2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "general/persona          ok") {
		t.Errorf("an unrelated edit to CLAUDE.md must not read as drift:\n%s", out)
	}
}

// Deleting the import is the failure this feature exists to make visible.
func TestStatusReportsAMissingImportAndInitRestoresIt(t *testing.T) {
	kitDir, projectDir := personaProject(t, "# CRM DEAL\n")
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, "web"); err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(projectDir, "CLAUDE.md"), "# CRM DEAL\n\n## sin el import\n")

	e2, out := env(t, kitDir, projectDir)
	if err := Status(e2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "FALTA IMPORT") {
		t.Errorf("status does not notice the missing import:\n%s", out)
	}
	if !strings.Contains(out.String(), "CLAUDE.md") {
		t.Errorf("status does not name the file:\n%s", out)
	}

	e3, _ := env(t, kitDir, projectDir)
	e3.AssumeYes = true
	if err := Init(e3, "web"); err != nil {
		t.Fatal(err)
	}
	if got, want := claudeMD(t, projectDir), "# CRM DEAL\n\n## sin el import\n"+personaImport+"\n"; got != want {
		t.Errorf("CLAUDE.md = %q, want %q", got, want)
	}
}
