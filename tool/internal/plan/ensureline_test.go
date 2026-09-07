package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

const importLine = "@.claude/persona.md"

// personaArtifact mirrors the real general/persona: a config file copied
// verbatim, plus one line guaranteed inside the project's own CLAUDE.md.
func personaArtifact() kit.Artifact {
	return kit.Artifact{
		ID: "general/persona", Type: "config",
		Src: "config/persona.md", Dest: ".claude/persona.md",
		EnsureLine: &kit.EnsuredLine{File: "CLAUDE.md", Line: importLine},
	}
}

func personaFixture(t *testing.T) (kitDir, projectDir string) {
	t.Helper()
	return fixture(t, map[string]string{"config/persona.md": "be brief\n"})
}

func buildPersona(t *testing.T, kitDir, projectDir string, lock *lockfile.File) *Plan {
	t.Helper()
	p, err := Build(Input{
		Artifacts: []kit.Artifact{personaArtifact()},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Case 1: the destination file does not exist. It is created holding the line.
func TestEnsureLineCreatesTheFileWhenItIsAbsent(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	lock := &lockfile.File{Roots: roots}

	p := buildPersona(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, "CLAUDE.md"); k != AppendLine {
		t.Fatalf("CLAUDE.md: kind = %q (%s), want %q", k, r, AppendLine)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if got := read(t, projectDir, "CLAUDE.md"); got != importLine+"\n" {
		t.Errorf("CLAUDE.md = %q, want %q", got, importLine+"\n")
	}
}

// Case 3: the file exists and lacks the line. Every original byte survives and
// the line lands at the end.
func TestEnsureLineAppendsWithoutTouchingWhatIsAlreadyThere(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	original := "# CRM DEAL\n\n## Reglas\n\n- correr los tests\n"
	write(t, projectDir, "CLAUDE.md", original)
	lock := &lockfile.File{Roots: roots}

	p := buildPersona(t, kitDir, projectDir, lock)
	if k, _ := kindOf(p, "CLAUDE.md"); k != AppendLine {
		t.Fatalf("CLAUDE.md: kind = %q, want %q", k, AppendLine)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	got := read(t, projectDir, "CLAUDE.md")
	if !strings.HasPrefix(got, original) {
		t.Errorf("the original content was not preserved verbatim:\n got %q\nwant prefix %q", got, original)
	}
	if got != original+importLine+"\n" {
		t.Errorf("CLAUDE.md = %q, want the original plus the line", got)
	}
}

// A file with no trailing newline must not have the import welded onto its
// last line.
func TestEnsureLineDoesNotJoinTheProjectsLastLine(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	write(t, projectDir, "CLAUDE.md", "no trailing newline")
	lock := &lockfile.File{Roots: roots}

	p := buildPersona(t, kitDir, projectDir, lock)
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, projectDir, "CLAUDE.md"), "no trailing newline\n"+importLine+"\n"; got != want {
		t.Errorf("CLAUDE.md = %q, want %q", got, want)
	}
}

// Case 2: the line is already there. Nothing is planned and nothing is written.
func TestEnsureLineIsANoOpWhenTheLineIsAlreadyPresent(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	original := "# CRM DEAL\n" + importLine + "\n\nmore\n"
	write(t, projectDir, "CLAUDE.md", original)
	lock := &lockfile.File{Roots: roots}

	p := buildPersona(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, "CLAUDE.md"); k != Unchanged {
		t.Fatalf("CLAUDE.md: kind = %q (%s), want %q", k, r, Unchanged)
	}
	for _, a := range p.Changes() {
		if a.Path == "CLAUDE.md" {
			t.Errorf("CLAUDE.md is reported as a change: %+v", a)
		}
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if got := read(t, projectDir, "CLAUDE.md"); got != original {
		t.Errorf("CLAUDE.md = %q, want it untouched (%q)", got, original)
	}
}

// Running init twice must converge: the second plan has nothing to do, and a
// third apply cannot duplicate the import.
func TestEnsureLineConvergesAcrossRepeatedRuns(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	write(t, projectDir, "CLAUDE.md", "# CRM DEAL\n")
	lock := &lockfile.File{Roots: roots}

	for i := 0; i < 3; i++ {
		p := buildPersona(t, kitDir, projectDir, lock)
		if err := p.Apply(projectDir, lock); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if got, want := read(t, projectDir, "CLAUDE.md"), "# CRM DEAL\n"+importLine+"\n"; got != want {
		t.Errorf("CLAUDE.md = %q, want %q", got, want)
	}
	if n := strings.Count(read(t, projectDir, "CLAUDE.md"), importLine); n != 1 {
		t.Errorf("the import appears %d times, want exactly 1", n)
	}
}

// The project's own file is never taken over: the CLI adds a line to it, so it
// must not appear in the lockfile's Files, where a hash would then report the
// team's own edits as drift.
func TestEnsureLineIsRecordedAsAPresenceNotAHash(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	lock := &lockfile.File{Roots: roots}
	if err := buildPersona(t, kitDir, projectDir, lock).Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	rec, ok := lock.Artifact("general/persona")
	if !ok {
		t.Fatal("general/persona is not recorded in the lockfile")
	}
	if lock.Owns("CLAUDE.md") {
		t.Error("the lockfile claims ownership of CLAUDE.md; the project owns that file")
	}
	if len(rec.Lines) != 1 {
		t.Fatalf("recorded lines = %+v, want exactly one", rec.Lines)
	}
	if rec.Lines[0].Path != "CLAUDE.md" || rec.Lines[0].Line != importLine {
		t.Errorf("recorded line = %+v, want {CLAUDE.md %s}", rec.Lines[0], importLine)
	}
}

// The whole reason presence is tracked instead of a hash: the team edits
// CLAUDE.md constantly, and none of it is the kit's business.
func TestEditingTheRestOfTheFileIsNotReportedAsDrift(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	lock := &lockfile.File{Roots: roots}
	if err := buildPersona(t, kitDir, projectDir, lock).Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	write(t, projectDir, "CLAUDE.md", importLine+"\n\n# lo que el equipo escribió después\n")

	p := buildPersona(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, "CLAUDE.md"); k != Unchanged {
		t.Errorf("CLAUDE.md: kind = %q (%s), want %q — an unrelated edit must not read as drift", k, r, Unchanged)
	}
	if len(p.Blocked()) != 0 {
		t.Errorf("blocked = %+v, want none", p.Blocked())
	}
}

// Case 4: someone removes the line. The next plan notices and restores it,
// keeping everything the file gained in the meantime.
func TestARemovedLineIsPlannedAgainAndRestored(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	lock := &lockfile.File{Roots: roots}
	if err := buildPersona(t, kitDir, projectDir, lock).Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	write(t, projectDir, "CLAUDE.md", "# solo esto\n")

	p := buildPersona(t, kitDir, projectDir, lock)
	if k, _ := kindOf(p, "CLAUDE.md"); k != AppendLine {
		t.Fatalf("CLAUDE.md: kind = %q, want %q", k, AppendLine)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, projectDir, "CLAUDE.md"), "# solo esto\n"+importLine+"\n"; got != want {
		t.Errorf("CLAUDE.md = %q, want %q", got, want)
	}
}

// An indented or CRLF copy of the line is still the line. Appending a second
// one would be the tool arguing with the file.
func TestEnsureLineAcceptsTheLineAsTheProjectWroteIt(t *testing.T) {
	for name, content := range map[string]string{
		"indented": "# CRM DEAL\n  " + importLine + "\n",
		"crlf":     "# CRM DEAL\r\n" + importLine + "\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			kitDir, projectDir := personaFixture(t)
			write(t, projectDir, "CLAUDE.md", content)
			p := buildPersona(t, kitDir, projectDir, &lockfile.File{Roots: roots})
			if k, _ := kindOf(p, "CLAUDE.md"); k != Unchanged {
				t.Errorf("CLAUDE.md: kind = %q, want %q", k, Unchanged)
			}
		})
	}
}

// A substring is not a line: a path that merely starts with the import must
// not be mistaken for it.
func TestEnsureLineDoesNotMatchASubstring(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	write(t, projectDir, "CLAUDE.md", "@.claude/persona.md.bak\n")

	p := buildPersona(t, kitDir, projectDir, &lockfile.File{Roots: roots})
	if k, _ := kindOf(p, "CLAUDE.md"); k != AppendLine {
		t.Errorf("CLAUDE.md: kind = %q, want %q", k, AppendLine)
	}
}

// Building a plan must never write: --dry-run stops between Build and Apply,
// so anything Build touched would escape the flag entirely.
func TestBuildingAnEnsureLinePlanWritesNothing(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	buildPersona(t, kitDir, projectDir, &lockfile.File{Roots: roots})

	if _, err := os.Stat(filepath.Join(projectDir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("Build created CLAUDE.md; stat err = %v, want not-exist", err)
	}
}

// A traversal in the ensure_line file goes through the same guard as every
// other destination.
func TestEnsureLineRefusesToEscapeTheProject(t *testing.T) {
	kitDir, projectDir := personaFixture(t)
	a := personaArtifact()
	a.EnsureLine = &kit.EnsuredLine{File: "../outside/CLAUDE.md", Line: importLine}

	_, err := Build(Input{
		Artifacts: []kit.Artifact{a},
		Lock:      &lockfile.File{Roots: roots}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err == nil || !strings.Contains(err.Error(), "se sale del directorio del proyecto") {
		t.Fatalf("err = %v, want a traversal refusal", err)
	}
}
