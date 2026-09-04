package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

// fixture builds a kit checkout and an empty project, and returns their paths.
func fixture(t *testing.T, kitFiles map[string]string) (kitDir, projectDir string) {
	t.Helper()
	kitDir = t.TempDir()
	projectDir = t.TempDir()
	for rel, content := range kitFiles {
		abs := filepath.Join(kitDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return kitDir, projectDir
}

var roots = map[string]string{"src": "src", "ui": "src/shared/ui"}

func componentArtifact() kit.Artifact {
	return kit.Artifact{
		ID: "ui-kit/base", Type: "component",
		Src: "ui-kit/lib", Dest: "{src}/shared/lib",
		NPM: map[string]string{"clsx": "^2.1.1"},
	}
}

func skillArtifact() kit.Artifact {
	return kit.Artifact{ID: "web/ui", Type: "skill", Src: "skills/web/ui"}
}

func commandArtifact() kit.Artifact {
	return kit.Artifact{ID: "web/generate-schema", Type: "command", Src: "commands/web/generate-schema.md"}
}

func agentArtifact() kit.Artifact {
	return kit.Artifact{ID: "backend/review-security", Type: "agent", Src: "agents/backend/review-security.md"}
}

func kindOf(p *Plan, path string) (Kind, string) {
	for _, a := range p.Actions {
		if a.Path == path {
			return a.Kind, a.Reason
		}
	}
	return "", "not in plan"
}

func TestBuildCreatesFilesInAnEmptyProject(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts":         "export const cn = 1\n",
		"ui-kit/lib/storage/index.ts": "export const s = 1\n",
	})
	lock := &lockfile.File{Roots: roots}

	p, err := Build(Input{
		Artifacts: []kit.Artifact{componentArtifact()},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"src/shared/lib/utils.ts", "src/shared/lib/storage/index.ts"} {
		if k, r := kindOf(p, want); k != Create {
			t.Errorf("%s: kind = %q (%s), want create", want, k, r)
		}
	}
	if p.Deps["clsx"] != "^2.1.1" {
		t.Errorf("deps = %v, want clsx", p.Deps)
	}
}

func TestSkillDestinationIsDerivedFromFlattenedID(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"skills/web/ui/SKILL.md": "---\nname: web-ui\n---\n",
	})

	p, err := Build(Input{
		Artifacts: []kit.Artifact{skillArtifact()},
		Lock:      &lockfile.File{}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, ".claude/skills/web-ui/SKILL.md"); k != Create {
		t.Errorf("kind = %q (%s), want create at the flattened path", k, r)
	}
}

func TestCommandDestinationIsDerivedFromLeafName(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"commands/web/generate-schema.md": "---\ndescription: writes a Zod schema\n---\n",
	})

	p, err := Build(Input{
		Artifacts: []kit.Artifact{commandArtifact()},
		Lock:      &lockfile.File{}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, ".claude/commands/generate-schema.md"); k != Create {
		t.Errorf("kind = %q (%s), want create at the leaf-name path", k, r)
	}
}

func TestAgentDestinationIsDerivedFromFlattenedID(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"agents/backend/review-security.md": "---\nname: backend-review-security\n---\n",
	})

	p, err := Build(Input{
		Artifacts: []kit.Artifact{agentArtifact()},
		Lock:      &lockfile.File{}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, ".claude/agents/backend-review-security.md"); k != Create {
		t.Errorf("kind = %q (%s), want create at the flattened path", k, r)
	}
}

func TestApplyWritesFilesAndRecordsThem(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts": "export const cn = 1\n",
	})
	lock := &lockfile.File{Roots: roots}

	p, err := Build(Input{
		Artifacts: []kit.Artifact{componentArtifact()},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	written := filepath.Join(projectDir, "src", "shared", "lib", "utils.ts")
	got, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("file was not written: %v", err)
	}
	if string(got) != "export const cn = 1\n" {
		t.Errorf("content = %q", got)
	}
	if !lock.Owns("src/shared/lib/utils.ts") {
		t.Error("lockfile does not own the written file")
	}
	if h, _ := lock.RecordedHash("src/shared/lib/utils.ts"); h != lockfile.Hash(got) {
		t.Error("recorded hash does not match the written content")
	}
}

func TestUnchangedWhenProjectAlreadyMatchesTheKit(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts": "export const cn = 1\n",
	})
	lock := &lockfile.File{Roots: roots}

	first, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := first.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	second, err := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(second, "src/shared/lib/utils.ts"); k != Unchanged {
		t.Errorf("kind = %q (%s), want unchanged on a second run", k, r)
	}
	if len(second.Changes()) != 0 {
		t.Errorf("a re-run must produce no changes, got %d", len(second.Changes()))
	}
}

func TestOverwriteWhenTheKitMovedForward(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts": "export const cn = 1\n",
	})
	lock := &lockfile.File{Roots: roots}
	first, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := first.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	// The kit publishes a new version of the same file.
	if err := os.WriteFile(filepath.Join(kitDir, "ui-kit", "lib", "utils.ts"),
		[]byte("export const cn = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, "src/shared/lib/utils.ts"); k != Overwrite {
		t.Fatalf("kind = %q (%s), want overwrite", k, r)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(projectDir, "src", "shared", "lib", "utils.ts"))
	if string(got) != "export const cn = 2\n" {
		t.Errorf("content = %q, want the updated version", got)
	}
}

func TestBlockedWhenAManagedFileWasEditedLocally(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts": "export const cn = 1\n",
	})
	lock := &lockfile.File{Roots: roots}
	first, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := first.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	// Someone edits the installed file, and the kit also moves forward.
	local := filepath.Join(projectDir, "src", "shared", "lib", "utils.ts")
	if err := os.WriteFile(local, []byte("export const cn = 999 // mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kitDir, "ui-kit", "lib", "utils.ts"),
		[]byte("export const cn = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	k, reason := kindOf(p, "src/shared/lib/utils.ts")
	if k != Blocked {
		t.Fatalf("kind = %q, want blocked", k)
	}
	if reason != "edited locally since deal-kit wrote it" {
		t.Errorf("reason = %q", reason)
	}

	// Apply must refuse, and the local edit must survive untouched.
	if err := p.Apply(projectDir, lock); err == nil {
		t.Fatal("Apply must refuse while a file is blocked")
	}
	got, _ := os.ReadFile(local)
	if string(got) != "export const cn = 999 // mine\n" {
		t.Errorf("the local edit was destroyed: %q", got)
	}
}

func TestBlockedWhenAnUnmanagedFileIsInTheWay(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts": "export const cn = 1\n",
	})
	// The project already has its own file at the destination.
	dest := filepath.Join(projectDir, "src", "shared", "lib", "utils.ts")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("// pre-existing project file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: &lockfile.File{Roots: roots}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	k, reason := kindOf(p, "src/shared/lib/utils.ts")
	if k != Blocked {
		t.Fatalf("kind = %q, want blocked for a file deal-kit never wrote", k)
	}
	if reason != "file exists but is not managed by deal-kit" {
		t.Errorf("reason = %q", reason)
	}
}

func TestDeletesFilesTheArtifactNoLongerShips(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts":  "export const cn = 1\n",
		"ui-kit/lib/legacy.ts": "export const old = 1\n",
	})
	lock := &lockfile.File{Roots: roots}
	first, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := first.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	// The kit drops legacy.ts.
	if err := os.Remove(filepath.Join(kitDir, "ui-kit", "lib", "legacy.ts")); err != nil {
		t.Fatal(err)
	}

	p, err := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, "src/shared/lib/legacy.ts"); k != Delete {
		t.Fatalf("kind = %q (%s), want delete", k, r)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "shared", "lib", "legacy.ts")); !os.IsNotExist(err) {
		t.Error("legacy.ts was not removed")
	}
	if lock.Owns("src/shared/lib/legacy.ts") {
		t.Error("lockfile still owns the deleted file")
	}
}

func TestDoesNotDeleteAFileTheUserEdited(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/utils.ts":  "export const cn = 1\n",
		"ui-kit/lib/legacy.ts": "export const old = 1\n",
	})
	lock := &lockfile.File{Roots: roots}
	first, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := first.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	local := filepath.Join(projectDir, "src", "shared", "lib", "legacy.ts")
	if err := os.WriteFile(local, []byte("export const old = 42 // still needed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(kitDir, "ui-kit", "lib", "legacy.ts")); err != nil {
		t.Fatal(err)
	}

	p, err := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := kindOf(p, "src/shared/lib/legacy.ts"); k != Blocked {
		t.Fatalf("kind = %q, want blocked rather than a silent delete", k)
	}
	if _, err := os.Stat(local); err != nil {
		t.Error("the edited file must still exist")
	}
}

func TestApplyPrunesDirectoriesLeftEmpty(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"ui-kit/lib/storage/adapter.ts": "export const a = 1\n",
	})
	lock := &lockfile.File{Roots: roots}
	first, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := first.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(kitDir, "ui-kit", "lib", "storage", "adapter.ts")); err != nil {
		t.Fatal(err)
	}

	p, _ := Build(Input{Artifacts: []kit.Artifact{componentArtifact()},
		Lock: lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "shared", "lib", "storage")); !os.IsNotExist(err) {
		t.Error("empty storage/ directory was not pruned")
	}
	// Pruning must stop at the project root.
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatal("pruning removed the project directory")
	}
}

func TestBuildRejectsADestinationThatEscapesTheProject(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{"ui-kit/lib/utils.ts": "x\n"})
	a := componentArtifact()
	a.Dest = "{src}/../../outside"

	_, err := Build(Input{Artifacts: []kit.Artifact{a},
		Lock: &lockfile.File{}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots})
	if err == nil {
		t.Fatal("expected Build to reject a destination outside the project")
	}
}

// writeProjectFile writes a file inside the project and returns its hash, so a
// test can record in the lockfile exactly what deal-kit would have written.
func writeProjectFile(t *testing.T, projectDir, rel, content string) string {
	t.Helper()
	abs := filepath.Join(projectDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return lockfile.Hash([]byte(content))
}

func TestBuildDeletesTheFilesOfAnOrphanedArtifact(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"skills/web/ui/SKILL.md": "---\nname: web-ui\n---\n",
	})
	hash := writeProjectFile(t, projectDir, ".claude/skills/general-pr-workflow/SKILL.md", "old skill\n")
	lock := &lockfile.File{Roots: roots, Artifacts: []lockfile.Installed{{
		ID:    "general/pr-workflow",
		Files: []lockfile.OwnedFile{{Path: ".claude/skills/general-pr-workflow/SKILL.md", Hash: hash}},
	}}}

	p, err := Build(Input{
		Artifacts: []kit.Artifact{skillArtifact()},
		Orphans:   []string{"general/pr-workflow"},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, ".claude/skills/general-pr-workflow/SKILL.md"); k != Delete {
		t.Fatalf("kind = %q (%s), want delete", k, r)
	}

	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "general-pr-workflow", "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("the orphaned file is still on disk (err = %v)", err)
	}
	if _, ok := lock.Artifact("general/pr-workflow"); ok {
		t.Error("the orphaned artifact is still recorded in the lockfile")
	}
}

func TestBuildBlocksAnOrphanedArtifactWithALocalEdit(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"skills/web/ui/SKILL.md": "---\nname: web-ui\n---\n",
	})
	writeProjectFile(t, projectDir, ".claude/skills/general-pr-workflow/SKILL.md", "edited by hand\n")
	lock := &lockfile.File{Roots: roots, Artifacts: []lockfile.Installed{{
		ID: "general/pr-workflow",
		Files: []lockfile.OwnedFile{{
			Path: ".claude/skills/general-pr-workflow/SKILL.md",
			Hash: lockfile.Hash([]byte("as deal-kit wrote it\n")),
		}},
	}}}

	p, err := Build(Input{
		Artifacts: []kit.Artifact{skillArtifact()},
		Orphans:   []string{"general/pr-workflow"},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	k, reason := kindOf(p, ".claude/skills/general-pr-workflow/SKILL.md")
	if k != Blocked {
		t.Fatalf("kind = %q (%s), want blocked", k, reason)
	}
	if reason != "no longer part of this artifact, but edited locally" {
		t.Errorf("reason = %q, want the wording every other removal uses", reason)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "general-pr-workflow", "SKILL.md")); err != nil {
		t.Errorf("the locally edited file must stay on disk: %v", err)
	}
}

func TestBuildIgnoresAnOrphanWhoseFilesAreAlreadyGone(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{
		"skills/web/ui/SKILL.md": "---\nname: web-ui\n---\n",
	})
	lock := &lockfile.File{Roots: roots, Artifacts: []lockfile.Installed{{
		ID: "general/pr-workflow",
		Files: []lockfile.OwnedFile{{
			Path: ".claude/skills/general-pr-workflow/SKILL.md",
			Hash: lockfile.Hash([]byte("gone\n")),
		}},
	}}}

	p, err := Build(Input{
		Artifacts: []kit.Artifact{skillArtifact()},
		Orphans:   []string{"general/pr-workflow"},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := kindOf(p, ".claude/skills/general-pr-workflow/SKILL.md"); k != "" {
		t.Errorf("kind = %q, want no action for a file that is already gone", k)
	}
	// The stale entry must still leave the lockfile, or status reports the
	// orphan forever with nothing left to do about it.
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Artifact("general/pr-workflow"); ok {
		t.Error("the orphaned artifact is still recorded in the lockfile")
	}
}
