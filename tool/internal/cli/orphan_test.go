package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

// The manifest a project is synced against. It declares web/ui and nothing
// else, so a lockfile entry for anything else is orphaned.
const testManifest = `
version: 1
project_types:
  web:
    match: "crm-deal-web"
    roots: { src: src, ui: src/shared/ui }
profiles:
  web: [web/ui]
artifacts:
  - { id: web/ui, type: skill, applies_to: [web], src: skills/web/ui }
`

const skillContent = "---\nname: web-ui\ndescription: web conventions\n---\n"

const orphanPath = ".claude/skills/general-pr-workflow/SKILL.md"

// project builds a kit checkout and a project synced against it. The project
// has web/ui installed and unmodified; orphanContent, when non-empty, is
// written to the orphan's file, and recordedOrphan is the hash the lockfile
// claims for it, so a test can make the file match or diverge.
func project(t *testing.T, orphanContent, recordedOrphan string) (kitDir, projectDir string) {
	t.Helper()
	kitDir, projectDir = t.TempDir(), t.TempDir()

	write(t, filepath.Join(kitDir, "kit.yaml"), testManifest)
	write(t, filepath.Join(kitDir, "skills", "web", "ui", "SKILL.md"), skillContent)

	write(t, filepath.Join(projectDir, "package.json"), "{}\n")
	write(t, filepath.Join(projectDir, ".claude", "skills", "web-ui", "SKILL.md"), skillContent)

	lock := &lockfile.File{
		KitVersion:  "kit-v0.1.0",
		ProjectType: "web",
		Roots:       map[string]string{"src": "src", "ui": "src/shared/ui"},
		Artifacts: []lockfile.Installed{{
			ID: "web/ui",
			Files: []lockfile.OwnedFile{{
				Path: ".claude/skills/web-ui/SKILL.md",
				Hash: lockfile.Hash([]byte(skillContent)),
			}},
		}},
	}
	if orphanContent != "" {
		write(t, filepath.Join(projectDir, filepath.FromSlash(orphanPath)), orphanContent)
		lock.Artifacts = append(lock.Artifacts, lockfile.Installed{
			ID:    "general/pr-workflow",
			Files: []lockfile.OwnedFile{{Path: orphanPath, Hash: recordedOrphan}},
		})
	}
	if err := lock.Save(projectDir); err != nil {
		t.Fatal(err)
	}
	return kitDir, projectDir
}

func write(t *testing.T, abs, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func env(t *testing.T, kitDir, projectDir string) (Env, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	return Env{
		Stdout: out, Stderr: out, Stdin: strings.NewReader(""),
		Cwd: projectDir, KitDir: kitDir, Version: "test", NoDeps: true,
	}, out
}

func TestStatusReportsAnOrphanedArtifactInsteadOfFailing(t *testing.T) {
	kitDir, projectDir := project(t, "old skill\n", lockfile.Hash([]byte("old skill\n")))
	e, out := env(t, kitDir, projectDir)

	if err := Status(e); err != nil {
		t.Fatalf("Status() = %v, want no error for an id the manifest no longer declares", err)
	}
	if !strings.Contains(out.String(), "general/pr-workflow") {
		t.Errorf("the orphan is missing from the report:\n%s", out)
	}
	if !strings.Contains(out.String(), "HUÉRFANO") {
		t.Errorf("the orphan is not reported as ORPHANED:\n%s", out)
	}
	if !strings.Contains(out.String(), "web/ui") {
		t.Errorf("the installed artifact is missing from the report:\n%s", out)
	}
}

func TestStatusOnAProjectWithNoOrphansIsUnchanged(t *testing.T) {
	kitDir, projectDir := project(t, "", "")
	e, out := env(t, kitDir, projectDir)

	if err := Status(e); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "HUÉRFANO") {
		t.Errorf("nothing is orphaned here:\n%s", out)
	}
	if !strings.Contains(out.String(), "web/ui                   ok") {
		t.Errorf("web/ui should report ok:\n%s", out)
	}
}

func TestUpdatePlansTheDeletionOfAnOrphanedArtifact(t *testing.T) {
	kitDir, projectDir := project(t, "old skill\n", lockfile.Hash([]byte("old skill\n")))
	e, out := env(t, kitDir, projectDir)
	e.DryRun = true

	if err := Update(e); err != nil {
		t.Fatalf("Update() = %v, want no error", err)
	}
	if !strings.Contains(out.String(), "borrar       "+orphanPath) {
		t.Errorf("the orphan's file is not planned for deletion:\n%s", out)
	}
}

func TestUpdateDeletesAnOrphanedArtifactAndItsLockEntry(t *testing.T) {
	kitDir, projectDir := project(t, "old skill\n", lockfile.Hash([]byte("old skill\n")))
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true

	if err := Update(e); err != nil {
		t.Fatalf("Update() = %v, want no error", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(orphanPath))); !os.IsNotExist(err) {
		t.Errorf("the orphan's file survived the update (err = %v)", err)
	}
	lock, _, err := lockfile.Load(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Artifact("general/pr-workflow"); ok {
		t.Error("the orphan is still recorded in the lockfile")
	}
	if _, ok := lock.Artifact("web/ui"); !ok {
		t.Error("web/ui must stay recorded")
	}
}

func TestUpdateBlocksAnOrphanedArtifactWithALocalEdit(t *testing.T) {
	kitDir, projectDir := project(t, "edited by hand\n", lockfile.Hash([]byte("as deal-kit wrote it\n")))
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true

	err := Update(e)
	if err == nil {
		t.Fatal("Update() = nil, want the blocked error")
	}
	if !strings.Contains(err.Error(), "requieren atención") {
		t.Errorf("error = %v, want the same blocked wording every artifact uses", err)
	}
	if !strings.Contains(out.String(), "ya no forma parte de este artefacto, pero fue editado localmente") {
		t.Errorf("the reason is missing from the report:\n%s", out)
	}
	abs := filepath.Join(projectDir, filepath.FromSlash(orphanPath))
	if data, err := os.ReadFile(abs); err != nil || string(data) != "edited by hand\n" {
		t.Errorf("the locally edited file must be untouched (data = %q, err = %v)", data, err)
	}
	lock, _, err := lockfile.Load(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Artifact("general/pr-workflow"); !ok {
		t.Error("a blocked orphan must stay recorded, or its file becomes unowned")
	}
}

func TestAddStillRejectsAnIDTheManifestDoesNotDeclare(t *testing.T) {
	kitDir, projectDir := project(t, "old skill\n", lockfile.Hash([]byte("old skill\n")))
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true

	// A typo the user types is not history: it must fail, orphans present or
	// not, or `add` silently installs nothing and reports success.
	err := Add(e, []string{"ui-kit/buton"})
	if err == nil {
		t.Fatal("Add() = nil, want an error for an unknown id")
	}
	if !strings.Contains(err.Error(), `artefacto desconocido "ui-kit/buton"`) {
		t.Errorf("error = %v, want unknown artifact", err)
	}
}

func TestStatusOmitsTheDetailColumnForAnOrphanWithNoFilesLeft(t *testing.T) {
	kitDir, projectDir := project(t, "old skill\n", lockfile.Hash([]byte("old skill\n")))
	if err := os.Remove(filepath.Join(projectDir, filepath.FromSlash(orphanPath))); err != nil {
		t.Fatal(err)
	}
	e, out := env(t, kitDir, projectDir)

	if err := Status(e); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, "HUÉRFANO") && line != strings.TrimRight(line, " ") {
			t.Errorf("the orphan line has a dangling detail column: %q", line)
		}
	}
}
