package kit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultRepo is the kit repository the CLI reads from.
const DefaultRepo = "https://github.com/oliviosubelza/deal-dev-kit.git"

// TagPrefix marks the tags that version kit content. The CLI's own releases
// use bare `v*` tags, which this deliberately does not match.
const TagPrefix = "kit-v"

// Source describes where to read the kit from.
type Source struct {
	Repo    string // git URL
	Ref     string // tag, branch or SHA; empty means the newest kit-v* tag
	Offline bool   // use the cache without contacting the remote
}

// Checkout is a local copy of the kit at a resolved version.
type Checkout struct {
	Dir     string // directory containing kit.yaml
	Version string // the tag, or a short SHA when the ref is not a tag
}

// Fetch makes the kit available locally and returns the checkout. The clone is
// cached per repository, so the common case is a fetch, not a full clone.
func Fetch(src Source) (Checkout, error) {
	if src.Repo == "" {
		src.Repo = DefaultRepo
	}
	dir, err := cacheDir(src.Repo)
	if err != nil {
		return Checkout{}, err
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if src.Offline {
			return Checkout{}, fmt.Errorf("no hay copia en caché de %s y se pasó --offline", src.Repo)
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return Checkout{}, err
		}
		// A blobless clone pulls history cheaply and file contents on demand.
		// No --quiet: git's output is captured, not printed, and it is what
		// makes a failure explain itself in the error below.
		if out, err := git("", "clone", "--filter=blob:none", src.Repo, dir); err != nil {
			return Checkout{}, fmt.Errorf("al clonar %s: %w\n%s", src.Repo, err, out)
		}
	} else if !src.Offline {
		// The cache is a disposable mirror and the remote is authoritative:
		// --force adopts a tag that moved upstream instead of refusing to
		// clobber the local one, and --prune-tags drops a tag deleted
		// upstream so it can no longer be picked as the newest kit-v*.
		if out, err := git(dir, "fetch", "--tags", "--force", "--prune", "--prune-tags", "origin"); err != nil {
			return Checkout{}, fmt.Errorf("al hacer fetch de %s: %w\n%s", src.Repo, err, out)
		}
	}

	ref := src.Ref
	if ref == "" {
		ref, err = latestKitTag(dir)
		if err != nil {
			return Checkout{}, err
		}
	}
	if out, err := git(dir, "checkout", "--detach", "--quiet", ref); err != nil {
		return Checkout{}, fmt.Errorf("al hacer checkout de %q: %w\n%s", ref, err, out)
	}

	version := ref
	if !strings.HasPrefix(ref, TagPrefix) {
		sha, err := git(dir, "rev-parse", "--short", "HEAD")
		if err != nil {
			return Checkout{}, err
		}
		version = ref + "@" + strings.TrimSpace(sha)
	}
	return Checkout{Dir: dir, Version: version}, nil
}

// latestKitTag returns the highest kit-v* tag, falling back to the default
// branch when the repository has no kit release yet.
func latestKitTag(dir string) (string, error) {
	out, err := git(dir, "tag", "--list", TagPrefix+"*", "--sort=-v:refname")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if tag := strings.TrimSpace(line); tag != "" {
			return tag, nil
		}
	}

	head, err := git(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		if ref := strings.TrimSpace(head); ref != "" {
			return ref, nil
		}
	}
	return "origin/main", nil
}

// cacheDir is a stable per-repository directory under the user's cache.
func cacheDir(repo string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(repo))
	name := slug(repo) + "-" + hex.EncodeToString(sum[:])[:8]
	return filepath.Join(base, "deal-kit", "kits", name), nil
}

// slug turns a git URL into a readable directory-name fragment, so a developer
// looking in the cache can tell which repository a directory belongs to.
func slug(repo string) string {
	s := strings.TrimSuffix(repo, ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "kit"
	}
	return b.String()
}

// git runs a git command, always with an explicit -C and with inherited
// repository overrides stripped: a developer running deal-kit from inside
// another repository must not have that repository's environment leak into
// operations on the kit cache.
func git(dir string, args ...string) (string, error) {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", full...)
	cmd.Env = sanitizedEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// leakedGitVars are the environment variables that would redirect a git
// command away from the directory we passed with -C.
var leakedGitVars = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES", "GIT_GRAFT_FILE",
	"GIT_REPLACE_REF_BASE", "GIT_PREFIX",
}

func sanitizedEnv() []string {
	drop := make(map[string]bool, len(leakedGitVars))
	for _, k := range leakedGitVars {
		drop[k] = true
	}
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i > 0 && drop[kv[:i]] {
			continue
		}
		out = append(out, kv)
	}
	// Never stop for an interactive credential prompt inside a TUI.
	return append(out, "GIT_TERMINAL_PROMPT=0")
}
