package kit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	tests := []struct{ repo, want string }{
		{"https://github.com/oliviosubelza/deal-dev-kit.git", "deal-dev-kit"},
		{"git@github.com:oliviosubelza/deal-dev-kit.git", "deal-dev-kit"},
		{"https://gitlab.com/group/sub/My.Kit", "my-kit"},
		{"", "kit"},
	}
	for _, tt := range tests {
		if got := slug(tt.repo); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.repo, got, tt.want)
		}
	}
}

func TestCacheDirIsStableAndDistinct(t *testing.T) {
	a, err := cacheDir("https://github.com/org/kit.git")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := cacheDir("https://github.com/org/kit.git")
	if a != again {
		t.Errorf("cacheDir is not stable: %q vs %q", a, again)
	}

	// Two repositories that slug the same must not share a directory.
	b, _ := cacheDir("https://gitlab.com/other/kit.git")
	if a == b {
		t.Errorf("different repositories share a cache directory: %q", a)
	}
}

func TestSanitizedEnvDropsRepositoryOverrides(t *testing.T) {
	t.Setenv("GIT_DIR", "/somewhere/else/.git")
	t.Setenv("GIT_WORK_TREE", "/somewhere/else")
	t.Setenv("PATH", os.Getenv("PATH")) // must survive

	env := sanitizedEnv()
	joined := strings.Join(env, "\n")
	for _, banned := range []string{"GIT_DIR=", "GIT_WORK_TREE="} {
		if strings.Contains(joined, banned) {
			t.Errorf("%s leaked into the git environment", banned)
		}
	}
	if !strings.Contains(joined, "PATH=") {
		t.Error("PATH was dropped")
	}
	if !strings.Contains(joined, "GIT_TERMINAL_PROMPT=0") {
		t.Error("GIT_TERMINAL_PROMPT=0 was not set; git could block on a credential prompt")
	}
}

// --- integration: these drive the real git binary ---

func skipWithoutGit(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping: runs the real git binary")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// originRepo builds a throwaway git repository that looks like a kit.
func originRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(sanitizedEnv(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--quiet", "--initial-branch=main")
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("kit.yaml", "version: 1\nproject_types:\n  web:\n    match: crm-deal-web\n")
	run("add", "-A")
	run("commit", "--quiet", "-m", "v1")
	run("tag", "kit-v1.0.0")

	write("kit.yaml", "version: 1\nproject_types:\n  web:\n    match: crm-deal-web\n  backend:\n    match: 'crm-deal-*-service'\n")
	run("add", "-A")
	run("commit", "--quiet", "-m", "v2")
	run("tag", "kit-v1.2.0")

	// A CLI release tag must never be mistaken for a kit version.
	run("tag", "v9.9.9")

	write("kit.yaml", "version: 1\nproject_types:\n  web:\n    match: crm-deal-web\n  mobile:\n    match: crm-deal-mobile\n")
	run("add", "-A")
	run("commit", "--quiet", "-m", "unreleased work on main")
	return dir
}

func TestFetchChecksOutTheNewestKitTag(t *testing.T) {
	skipWithoutGit(t)
	origin := originRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := Fetch(Source{Repo: origin})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "kit-v1.2.0" {
		t.Errorf("version = %q, want the newest kit tag kit-v1.2.0", got.Version)
	}

	m, err := LoadManifest(got.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.ProjectTypes["backend"]; !ok {
		t.Error("checked out the wrong commit: kit-v1.2.0 adds the backend project type")
	}
	if _, ok := m.ProjectTypes["mobile"]; ok {
		t.Error("checked out unreleased work from main instead of the tag")
	}
}

func TestFetchIgnoresCLIReleaseTags(t *testing.T) {
	skipWithoutGit(t)
	origin := originRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := Fetch(Source{Repo: origin})
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(got.Version, "v9") {
		t.Errorf("version = %q: a CLI release tag was treated as a kit version", got.Version)
	}
}

func TestFetchWithAnExplicitRef(t *testing.T) {
	skipWithoutGit(t)
	origin := originRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := Fetch(Source{Repo: origin, Ref: "kit-v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "kit-v1.0.0" {
		t.Errorf("version = %q, want kit-v1.0.0", got.Version)
	}
	m, _ := LoadManifest(got.Dir)
	if _, ok := m.ProjectTypes["backend"]; ok {
		t.Error("kit-v1.0.0 predates the backend project type")
	}
}

func TestFetchOnABranchRecordsTheSHA(t *testing.T) {
	skipWithoutGit(t)
	origin := originRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := Fetch(Source{Repo: origin, Ref: "origin/main"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.Version, "origin/main@") {
		t.Errorf("version = %q, want a branch pinned to a SHA", got.Version)
	}
}

func TestFetchReusesTheCacheAndPicksUpNewTags(t *testing.T) {
	skipWithoutGit(t)
	origin := originRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	first, err := Fetch(Source{Repo: origin})
	if err != nil {
		t.Fatal(err)
	}

	// The kit publishes a new version.
	cmd := exec.Command("git", "-C", origin, "tag", "kit-v2.0.0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	second, err := Fetch(Source{Repo: origin})
	if err != nil {
		t.Fatal(err)
	}
	if second.Dir != first.Dir {
		t.Errorf("cache was not reused: %q then %q", first.Dir, second.Dir)
	}
	if second.Version != "kit-v2.0.0" {
		t.Errorf("version = %q, want the newly published kit-v2.0.0", second.Version)
	}
}

func TestFetchOfflineWithoutACacheFails(t *testing.T) {
	skipWithoutGit(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	_, err := Fetch(Source{Repo: originRepo(t), Offline: true})
	if err == nil {
		t.Fatal("expected an error with no cache and --offline")
	}
	if !strings.Contains(err.Error(), "offline") {
		t.Errorf("error = %q, want it to mention offline", err)
	}
}

func TestFetchOfflineUsesAnExistingCache(t *testing.T) {
	skipWithoutGit(t)
	origin := originRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if _, err := Fetch(Source{Repo: origin}); err != nil {
		t.Fatal(err)
	}
	got, err := Fetch(Source{Repo: origin, Offline: true})
	if err != nil {
		t.Fatalf("offline fetch with a warm cache must work: %v", err)
	}
	if got.Version != "kit-v1.2.0" {
		t.Errorf("version = %q", got.Version)
	}
}

func TestFetchUnknownRefFails(t *testing.T) {
	skipWithoutGit(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	_, err := Fetch(Source{Repo: originRepo(t), Ref: "kit-v99.0.0"})
	if err == nil {
		t.Fatal("expected an error for a ref that does not exist")
	}
}
