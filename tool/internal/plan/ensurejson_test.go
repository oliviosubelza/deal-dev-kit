package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

const settingsFile = ".claude/settings.json"

// attributionArtifact mirrors the real general/attribution: no src at all,
// only two keys guaranteed inside the project's own .claude/settings.json.
func attributionArtifact() kit.Artifact {
	return kit.Artifact{
		ID: "general/attribution", Type: "config",
		EnsureJSON: &kit.EnsuredJSON{
			File: settingsFile,
			Values: map[string]any{
				"attribution": map[string]any{"commit": "", "pr": ""},
			},
		},
	}
}

func buildAttribution(t *testing.T, kitDir, projectDir string, lock *lockfile.File) *Plan {
	t.Helper()
	p, err := Build(Input{
		Artifacts: []kit.Artifact{attributionArtifact()},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Case 1: the file does not exist. It is created holding only the declared
// keys — nothing else is invented into a project's settings.
func TestEnsureJSONCreatesTheFileWhenItIsAbsent(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	p := buildAttribution(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, settingsFile); k != MergeJSON {
		t.Fatalf("%s: kind = %q (%s), want %q", settingsFile, k, r, MergeJSON)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	want := "{\n  \"attribution\": {\n    \"commit\": \"\",\n    \"pr\": \"\"\n  }\n}\n"
	if got := read(t, projectDir, settingsFile); got != want {
		t.Errorf("settings.json =\n%s\nwant\n%s", got, want)
	}
}

// Case 2: the key is absent. It is set, and every sibling key the project put
// in the file survives byte for byte — this is the whole reason ensure_json
// exists instead of a plain `dest`.
func TestEnsureJSONLeavesTheProjectsOwnKeysAlone(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	before := `{
  "permissions": {
    "allow": [
      "Bash(go test:*)"
    ]
  },
  "model": "opus",
  "hooks": {}
}
`
	write(t, projectDir, settingsFile, before)

	p := buildAttribution(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, settingsFile); k != MergeJSON {
		t.Fatalf("%s: kind = %q (%s), want %q", settingsFile, k, r, MergeJSON)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	want := `{
  "permissions": {
    "allow": [
      "Bash(go test:*)"
    ]
  },
  "model": "opus",
  "hooks": {},
  "attribution": {
    "commit": "",
    "pr": ""
  }
}
`
	if got := read(t, projectDir, settingsFile); got != want {
		t.Errorf("settings.json =\n%s\nwant\n%s", got, want)
	}
}

// Case 3: the keys are already there with the declared values. Nothing to do,
// and nothing written.
func TestEnsureJSONIsUnchangedWhenTheKeysAlreadyHoldTheValue(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	before := "{\n  \"model\": \"opus\",\n  \"attribution\": {\n    \"commit\": \"\",\n    \"pr\": \"\"\n  }\n}\n"
	write(t, projectDir, settingsFile, before)

	p := buildAttribution(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, settingsFile); k != Unchanged {
		t.Fatalf("%s: kind = %q (%s), want %q", settingsFile, k, r, Unchanged)
	}
	if len(p.Changes()) != 0 {
		t.Errorf("Changes() = %d, want 0", len(p.Changes()))
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	if got := read(t, projectDir, settingsFile); got != before {
		t.Errorf("settings.json was rewritten:\n%s", got)
	}
}

// Case 4: the key is there with another value. The project decided that on
// purpose, so a human decides — this is the one place ensure_json blocks and
// ensure_line never does.
func TestEnsureJSONBlocksOnAKeyThatHoldsAnotherValue(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	before := "{\n  \"attribution\": {\n    \"commit\": \"ours\",\n    \"pr\": \"\"\n  }\n}\n"
	write(t, projectDir, settingsFile, before)

	p := buildAttribution(t, kitDir, projectDir, lock)
	k, reason := kindOf(p, settingsFile)
	if k != Blocked {
		t.Fatalf("%s: kind = %q, want %q", settingsFile, k, Blocked)
	}
	if !strings.Contains(reason, "attribution.commit") {
		t.Errorf("reason = %q, want it to name attribution.commit", reason)
	}
	if err := p.Apply(projectDir, lock); err == nil {
		t.Fatal("Apply succeeded over a blocked key")
	}
	if got := read(t, projectDir, settingsFile); got != before {
		t.Errorf("a blocked plan still wrote the file:\n%s", got)
	}
}

// A file the project owns that is not JSON at all cannot be merged into, and
// rewriting it whole would lose it.
func TestEnsureJSONBlocksOnAFileItCannotParse(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}
	write(t, projectDir, settingsFile, "{ not json at all\n")

	p := buildAttribution(t, kitDir, projectDir, lock)
	if k, _ := kindOf(p, settingsFile); k != Blocked {
		t.Fatalf("%s: kind = %q, want %q", settingsFile, k, Blocked)
	}
}

// An intermediate key that is not an object cannot be descended into without
// replacing it, which is the same destruction case 4 refuses.
func TestEnsureJSONBlocksWhenAnIntermediateKeyIsNotAnObject(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}
	write(t, projectDir, settingsFile, "{\n  \"attribution\": \"off\"\n}\n")

	p := buildAttribution(t, kitDir, projectDir, lock)
	k, reason := kindOf(p, settingsFile)
	if k != Blocked {
		t.Fatalf("%s: kind = %q, want %q", settingsFile, k, Blocked)
	}
	if !strings.Contains(reason, "attribution") {
		t.Errorf("reason = %q, want it to name the key", reason)
	}
}

// Only the missing key is added; a sibling under the same object that already
// agrees is not rewritten, and one the project added under it survives.
func TestEnsureJSONMergesLeafByLeafNotWholeObject(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}
	write(t, projectDir, settingsFile,
		"{\n  \"attribution\": {\n    \"commit\": \"\",\n    \"theirs\": true\n  }\n}\n")

	p := buildAttribution(t, kitDir, projectDir, lock)
	if k, r := kindOf(p, settingsFile); k != MergeJSON {
		t.Fatalf("%s: kind = %q (%s), want %q", settingsFile, k, r, MergeJSON)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"attribution\": {\n    \"commit\": \"\",\n    \"theirs\": true,\n    \"pr\": \"\"\n  }\n}\n"
	if got := read(t, projectDir, settingsFile); got != want {
		t.Errorf("settings.json =\n%s\nwant\n%s", got, want)
	}
}

// The record is presence of a key and the value guaranteed, never a hash of a
// file the project owns: hashing settings.json would report drift on every run
// as soon as the team edited its own permissions.
func TestEnsureJSONIsRecordedAsKeysNotAHash(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	p := buildAttribution(t, kitDir, projectDir, lock)
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}

	rec, ok := lock.Artifact("general/attribution")
	if !ok {
		t.Fatal("the artifact left no record in the lockfile")
	}
	if len(rec.Files) != 0 {
		t.Errorf("Files = %v, want none: deal-kit owns no file here", rec.Files)
	}
	if lock.Owns(settingsFile) {
		t.Error("settings.json is recorded as owned; the project owns it")
	}
	want := []lockfile.EnsuredJSON{
		{Path: settingsFile, Key: "attribution.commit", Value: `""`},
		{Path: settingsFile, Key: "attribution.pr", Value: `""`},
	}
	if len(rec.JSON) != len(want) {
		t.Fatalf("JSON = %v, want %v", rec.JSON, want)
	}
	for i, w := range want {
		if rec.JSON[i] != w {
			t.Errorf("JSON[%d] = %v, want %v", i, rec.JSON[i], w)
		}
	}
}

// The team editing the rest of the file is not drift: only the keys are ours.
func TestEditingTheRestOfSettingsIsNotReportedAsDrift(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	if err := buildAttribution(t, kitDir, projectDir, lock).Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	write(t, projectDir, settingsFile,
		"{\n  \"attribution\": {\n    \"commit\": \"\",\n    \"pr\": \"\"\n  },\n  \"model\": \"opus\"\n}\n")

	p := buildAttribution(t, kitDir, projectDir, lock)
	if b := p.Blocked(); len(b) != 0 {
		t.Fatalf("blocked = %v, want none", b)
	}
	if k, _ := kindOf(p, settingsFile); k != Unchanged {
		t.Errorf("kind = %q, want %q", k, Unchanged)
	}
}

// A key deleted by hand comes back on the next run, and what the team wrote
// after it stays where it was.
func TestEnsureJSONRestoresAKeyThatWasRemoved(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	if err := buildAttribution(t, kitDir, projectDir, lock).Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	write(t, projectDir, settingsFile, "{\n  \"model\": \"opus\"\n}\n")

	p := buildAttribution(t, kitDir, projectDir, lock)
	if k, _ := kindOf(p, settingsFile); k != MergeJSON {
		t.Fatalf("kind = %q, want %q", k, MergeJSON)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	got := read(t, projectDir, settingsFile)
	if !strings.Contains(got, `"model": "opus"`) || !strings.Contains(got, `"pr": ""`) {
		t.Errorf("settings.json =\n%s", got)
	}
}

// Apply must converge, so it re-checks rather than trusting the planned Kind.
// A value that appeared between planning and writing is a decision made after
// the plan was computed, and losing that race silently is the one failure mode
// this feature could plausibly produce.
func TestEnsureJSONRefusesAValueThatAppearedAfterPlanning(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}

	p := buildAttribution(t, kitDir, projectDir, lock)
	write(t, projectDir, settingsFile, "{\n  \"attribution\": {\n    \"commit\": \"theirs\"\n  }\n}\n")

	err := p.Apply(projectDir, lock)
	if err == nil || !strings.Contains(err.Error(), "attribution.commit") {
		t.Fatalf("Apply error = %v, want it to refuse attribution.commit", err)
	}
}

// The file is resolved through the same traversal guard as any other
// destination: an artifact cannot write outside the project.
func TestEnsureJSONRejectsAFileOutsideTheProject(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	a := attributionArtifact()
	a.EnsureJSON.File = "../outside/settings.json"

	_, err := Build(Input{
		Artifacts: []kit.Artifact{a},
		Lock:      &lockfile.File{Roots: roots}, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err == nil {
		t.Fatal("Build accepted a file outside the project")
	}
	if _, statErr := os.Stat(filepath.Join(projectDir, "..", "outside")); statErr == nil {
		t.Error("something was written outside the project")
	}
}

// A src-less artifact copies nothing. Without the guard in filePairs the empty
// src resolves to the kit checkout itself and the whole kit gets installed.
func TestAnArtifactWithoutSrcCopiesNothing(t *testing.T) {
	kitDir, projectDir := fixture(t, map[string]string{"skills/web/ui/SKILL.md": "x\n"})
	lock := &lockfile.File{Roots: roots}

	p := buildAttribution(t, kitDir, projectDir, lock)
	for _, a := range p.Actions {
		if a.Kind == Create || a.Kind == Overwrite {
			t.Errorf("a src-less artifact planned to write %q", a.Path)
		}
	}
}

// A number is compared as the literal the file carries, so a merge never
// rewrites 1 as 1.0 behind the project's back.
func TestEnsureJSONKeepsANumberLiteralAsWritten(t *testing.T) {
	kitDir, projectDir := fixture(t, nil)
	lock := &lockfile.File{Roots: roots}
	write(t, projectDir, settingsFile, "{\n  \"timeout\": 30\n}\n")

	a := kit.Artifact{
		ID: "general/attribution", Type: "config",
		EnsureJSON: &kit.EnsuredJSON{
			File:   settingsFile,
			Values: map[string]any{"timeout": 30, "retries": 2},
		},
	}
	p, err := Build(Input{
		Artifacts: []kit.Artifact{a},
		Lock:      lock, KitDir: kitDir, ProjectDir: projectDir, Roots: roots,
	})
	if err != nil {
		t.Fatal(err)
	}
	if k, r := kindOf(p, settingsFile); k != MergeJSON {
		t.Fatalf("kind = %q (%s), want %q", k, r, MergeJSON)
	}
	if err := p.Apply(projectDir, lock); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"timeout\": 30,\n  \"retries\": 2\n}\n"
	if got := read(t, projectDir, settingsFile); got != want {
		t.Errorf("settings.json =\n%s\nwant\n%s", got, want)
	}
}
