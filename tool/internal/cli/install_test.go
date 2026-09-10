package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/engram"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
)

// installManifest declares a web profile of exactly one artifact while three
// artifacts apply to web, so `install` and `init` cannot produce the same
// result by accident. backend/api applies to another project type and must
// stay out of both.
const installManifest = `
version: 1
project_types:
  web:
    match: "crm-deal-web"
    roots: { src: src, ui: src/shared/ui }
  backend:
    match: "crm-deal-*-service"
    roots: { src: src }
profiles:
  web: [web/ui]
  backend: [backend/api]
artifacts:
  - { id: web/ui, type: skill, applies_to: [web], src: skills/web/ui }
  - { id: web/testing, type: skill, applies_to: [web], src: skills/web/testing }
  - { id: general/conventions, type: skill, src: skills/general/conventions }
  - { id: backend/api, type: skill, applies_to: [backend], src: skills/backend/api }
`

// installProject builds a kit checkout and an empty project: no lockfile, no
// .claude directory, which is the state `install` has to work from.
func installProject(t *testing.T) (kitDir, projectDir string) {
	t.Helper()
	kitDir = t.TempDir()
	write(t, filepath.Join(kitDir, "kit.yaml"), installManifest)
	for id, name := range map[string]string{
		"web/ui":              "web-ui",
		"web/testing":         "web-testing",
		"general/conventions": "general-conventions",
		"backend/api":         "backend-api",
	} {
		write(t, filepath.Join(kitDir, "skills", filepath.FromSlash(id), "SKILL.md"),
			"---\nname: "+name+"\ndescription: test artifact\n---\n")
	}

	// The directory name is what MatchProjectType reads, so the project lives
	// in a named subdirectory rather than the temp root.
	projectDir = filepath.Join(t.TempDir(), "crm-deal-web")
	write(t, filepath.Join(projectDir, "package.json"), "{}\n")
	return kitDir, projectDir
}

// installedIDs are the artifact ids the project's lockfile records, sorted.
func installedIDs(t *testing.T, projectDir string) []string {
	t.Helper()
	lock, existed, err := lockfile.Load(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatalf("no %s in %s", lockfile.Name, projectDir)
	}
	ids := lockIDs(lock)
	sort.Strings(ids)
	return ids
}

// The whole point of the command: an uninitialised project ends up with every
// artifact that applies to its type, not the profile subset `init` installs.
func TestInstallTakesEveryApplicableArtifactAndInitOnlyTheProfile(t *testing.T) {
	kitDir, initProject := installProject(t)
	e, out := env(t, kitDir, initProject)
	e.AssumeYes = true
	if err := Init(e, ""); err != nil {
		t.Fatalf("Init() = %v\n%s", err, out)
	}
	gotInit := installedIDs(t, initProject)
	if want := []string{"web/ui"}; !equalStrings(gotInit, want) {
		t.Fatalf("Init installed %v, want %v", gotInit, want)
	}

	_, installProjectDir := installProject(t)
	e2, out2 := env(t, kitDir, installProjectDir)
	e2.AssumeYes = true
	if err := Install(e2, ""); err != nil {
		t.Fatalf("Install() = %v\n%s", err, out2)
	}
	got := installedIDs(t, installProjectDir)
	want := []string{"general/conventions", "web/testing", "web/ui"}
	if !equalStrings(got, want) {
		t.Errorf("Install installed %v, want %v", got, want)
	}
	if len(got) <= len(gotInit) {
		t.Errorf("Install (%d artifacts) did not install more than Init (%d)", len(got), len(gotInit))
	}
}

// An artifact declared for another project type is not "everything": the
// selection is bounded by Supports, exactly like the browser's list.
func TestInstallSkipsArtifactsOfAnotherProjectType(t *testing.T) {
	kitDir, projectDir := installProject(t)
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Install(e, ""); err != nil {
		t.Fatalf("Install() = %v\n%s", err, out)
	}
	for _, id := range installedIDs(t, projectDir) {
		if id == "backend/api" {
			t.Error("backend/api was installed into a web project")
		}
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".claude", "skills", "backend-api")); !os.IsNotExist(err) {
		t.Errorf("backend-api reached the project: %v", err)
	}
}

// The lockfile is written from scratch, so the project is initialised by this
// one command and `status` works afterwards.
func TestInstallInitialisesAProjectThatHasNoLockfile(t *testing.T) {
	kitDir, projectDir := installProject(t)
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Install(e, ""); err != nil {
		t.Fatalf("Install() = %v\n%s", err, out)
	}
	lock, existed, err := lockfile.Load(projectDir)
	if err != nil || !existed {
		t.Fatalf("lockfile.Load() = (%v, %v)", existed, err)
	}
	if lock.ProjectType != "web" {
		t.Errorf("lock.ProjectType = %q, want web", lock.ProjectType)
	}
	if lock.Roots["ui"] != "src/shared/ui" {
		t.Errorf("lock.Roots = %v, want the type's roots", lock.Roots)
	}
}

// --type is the escape hatch when the directory name does not match, and it
// has to reach Install the same way it reaches Init.
func TestInstallHonoursTheTypeOverride(t *testing.T) {
	kitDir, _ := installProject(t)
	projectDir := t.TempDir() // a name MatchProjectType cannot resolve
	write(t, filepath.Join(projectDir, "package.json"), "{}\n")

	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Install(e, ""); err == nil {
		t.Fatalf("Install() without --type resolved a type it cannot detect:\n%s", out)
	}
	e2, out2 := env(t, kitDir, projectDir)
	e2.AssumeYes = true
	if err := Install(e2, "backend"); err != nil {
		t.Fatalf("Install(--type backend) = %v\n%s", err, out2)
	}
	// general/conventions declares no applies_to, so it is valid everywhere.
	if got, want := installedIDs(t, projectDir), []string{"backend/api", "general/conventions"}; !equalStrings(got, want) {
		t.Errorf("installed %v, want %v", got, want)
	}
}

// Convergence: the second run plans nothing and writes nothing.
func TestASecondInstallIsANoOp(t *testing.T) {
	kitDir, projectDir := installProject(t)
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Install(e, ""); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, projectDir)

	e2, out := env(t, kitDir, projectDir)
	e2.AssumeYes = true
	if err := Install(e2, ""); err != nil {
		t.Fatalf("second Install() = %v\n%s", err, out)
	}
	if !strings.Contains(out.String(), "ya está actualizado") {
		t.Errorf("the second run planned work:\n%s", out)
	}
	if after := snapshot(t, projectDir); after != before {
		t.Errorf("the second run changed the project:\n%s\nwas\n%s", after, before)
	}
}

// An already-initialised project keeps what it has and gains what is missing:
// `install` after `init` is additive, never a reinstall.
func TestInstallOnAnInitialisedProjectAddsOnlyWhatIsMissing(t *testing.T) {
	kitDir, projectDir := installProject(t)
	e, _ := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Init(e, ""); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(projectDir, ".claude", "skills", "web-ui", "SKILL.md")
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}

	e2, out := env(t, kitDir, projectDir)
	e2.AssumeYes = true
	if err := Install(e2, ""); err != nil {
		t.Fatalf("Install() = %v\n%s", err, out)
	}
	if got, want := installedIDs(t, projectDir), []string{"general/conventions", "web/testing", "web/ui"}; !equalStrings(got, want) {
		t.Errorf("installed %v, want %v", got, want)
	}
	after, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(info.ModTime()) {
		t.Error("the artifact init had already installed was rewritten")
	}
}

// Engram is a user-global install the browser offers on its own screen. The
// "Instalar todo" menu entry deliberately leaves it out, and the direct
// command must not become the back door that installs it.
func TestInstallNeverInstallsEngram(t *testing.T) {
	applied := stubEngram(t, engram.Status{State: engram.StateClaudeMissing}, engram.Outcome{})

	kitDir, projectDir := installProject(t)
	e, out := env(t, kitDir, projectDir)
	e.AssumeYes = true
	if err := Install(e, ""); err != nil {
		t.Fatalf("Install() = %v\n%s", err, out)
	}
	if len(*applied) > 0 {
		t.Errorf("Install ran an Engram plan: %v", *applied)
	}
	for _, path := range strings.Split(snapshot(t, projectDir), "\n") {
		if strings.Contains(path, "plugins") {
			t.Errorf("Install wrote %q", path)
		}
	}
}

// The header must not call this a profile: it is precisely what the command
// does not install.
func TestInstallHeaderDescribesTheScopeRatherThanTheProfile(t *testing.T) {
	kitDir, projectDir := installProject(t)
	e, out := env(t, kitDir, projectDir)
	e.DryRun = true
	if err := Install(e, ""); err != nil {
		t.Fatalf("Install() = %v\n%s", err, out)
	}
	if !strings.Contains(out.String(), "alcance    todo lo que aplica a web") {
		t.Errorf("the header does not describe the scope:\n%s", out)
	}
	if strings.Contains(out.String(), "perfil") {
		t.Errorf("the header calls the selection a profile:\n%s", out)
	}
	if !strings.Contains(out.String(), "--dry-run: no se escribió nada") {
		t.Errorf("--dry-run did not stop before writing:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(projectDir, lockfile.Name)); !os.IsNotExist(err) {
		t.Errorf("--dry-run wrote the lockfile: %v", err)
	}
}

// Only "y" applies, for install as for every other command that writes.
func TestInstallWithoutAConfirmationWritesNothing(t *testing.T) {
	kitDir, projectDir := installProject(t)
	e, _ := env(t, kitDir, projectDir) // env's Stdin is empty: an EOF, not a "y"
	if err := Install(e, ""); err != ErrAborted {
		t.Fatalf("Install() = %v, want ErrAborted", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, lockfile.Name)); !os.IsNotExist(err) {
		t.Errorf("a declined plan wrote the lockfile: %v", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// snapshot lists every file under dir with its content hash, so a test can
// assert a run changed nothing at all.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		lines = append(lines, filepath.ToSlash(rel)+" "+lockfile.Hash(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}
