package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingLockfileIsNotAnError(t *testing.T) {
	f, existed, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existed {
		t.Error("existed = true for a project with no lockfile")
	}
	if f.Roots == nil {
		t.Error("Roots must be usable without a nil check")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &File{
		KitVersion:  "kit-v1.4.0",
		ProjectType: "web",
		Roots:       map[string]string{"src": "src", "ui": "src/shared/ui"},
		Artifacts: []Installed{
			{ID: "ui-kit/base", Files: []OwnedFile{{Path: "src/shared/lib/utils.ts", Hash: "abc"}}},
		},
	}
	if err := want.Save(dir); err != nil {
		t.Fatal(err)
	}

	got, existed, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatal("existed = false after Save")
	}
	if got.KitVersion != want.KitVersion || got.ProjectType != want.ProjectType {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got.Roots["ui"] != "src/shared/ui" {
		t.Errorf("roots = %v", got.Roots)
	}
	if len(got.Artifacts) != 1 || got.Artifacts[0].Files[0].Hash != "abc" {
		t.Errorf("artifacts = %+v", got.Artifacts)
	}
}

func TestSaveIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	f := &File{
		Artifacts: []Installed{
			{ID: "ui-kit/button", Files: []OwnedFile{{Path: "z.tsx", Hash: "2"}, {Path: "a.tsx", Hash: "1"}}},
			{ID: "ui-kit/base", Files: []OwnedFile{{Path: "b.ts", Hash: "3"}}},
		},
	}
	if err := f.Save(dir); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, Name))

	// Saving the same content in a different input order must not change bytes,
	// or every run produces a spurious diff.
	g := &File{
		Artifacts: []Installed{
			{ID: "ui-kit/base", Files: []OwnedFile{{Path: "b.ts", Hash: "3"}}},
			{ID: "ui-kit/button", Files: []OwnedFile{{Path: "a.tsx", Hash: "1"}, {Path: "z.tsx", Hash: "2"}}},
		},
	}
	if err := g.Save(dir); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, Name))

	if string(first) != string(second) {
		t.Errorf("Save is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestSetReplacesRatherThanDuplicates(t *testing.T) {
	f := &File{}
	f.Set(Installed{ID: "ui-kit/base", Files: []OwnedFile{{Path: "a.ts", Hash: "1"}}})
	f.Set(Installed{ID: "ui-kit/base", Files: []OwnedFile{{Path: "a.ts", Hash: "2"}}})

	if len(f.Artifacts) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(f.Artifacts))
	}
	if got, _ := f.RecordedHash("a.ts"); got != "2" {
		t.Errorf("hash = %q, want the replacement %q", got, "2")
	}
}

func TestRemove(t *testing.T) {
	f := &File{}
	f.Set(Installed{ID: "a", Files: []OwnedFile{{Path: "a.ts"}}})
	f.Set(Installed{ID: "b", Files: []OwnedFile{{Path: "b.ts"}}})
	f.Remove("a")

	if _, ok := f.Artifact("a"); ok {
		t.Error("artifact a is still present")
	}
	if _, ok := f.Artifact("b"); !ok {
		t.Error("Remove dropped the wrong artifact")
	}
	if f.Owns("a.ts") {
		t.Error("Owns still reports a removed file")
	}
}

func TestOwnsGuardsUnmanagedPaths(t *testing.T) {
	f := &File{Artifacts: []Installed{
		{ID: "ui-kit/base", Files: []OwnedFile{{Path: "src/shared/lib/utils.ts", Hash: "x"}}},
	}}

	tests := []struct {
		path string
		want bool
	}{
		{"src/shared/lib/utils.ts", true},
		{"src/shared/lib/other.ts", false},
		{"src/main.tsx", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := f.Owns(tt.path); got != tt.want {
			t.Errorf("Owns(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestHashFileMissingFile(t *testing.T) {
	_, found, err := HashFile(filepath.Join(t.TempDir(), "nope.ts"))
	if err != nil {
		t.Fatalf("a missing file must not be an error: %v", err)
	}
	if found {
		t.Error("found = true for a missing file")
	}
}

func TestHashFileMatchesHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.ts")
	content := []byte("export const a = 1\n")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, found, err := HashFile(p)
	if err != nil || !found {
		t.Fatalf("HashFile: %v found=%v", err, found)
	}
	if want := Hash(content); got != want {
		t.Errorf("HashFile = %q, want %q", got, want)
	}
}
