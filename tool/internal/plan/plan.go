// Package plan computes the difference between the kit manifest and a
// project's lockfile, and applies it. Every mutation goes through a Plan, so
// --dry-run and the confirmation prompt show exactly what will happen, and the
// TUI and the non-interactive flags share one source of truth.
package plan

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/paths"
)

// Kind is what a single action does.
type Kind string

const (
	Create    Kind = "create"
	Overwrite Kind = "overwrite"
	Unchanged Kind = "unchanged"
	Delete    Kind = "delete"
	Blocked   Kind = "blocked" // a local edit or a foreign file; needs a human
)

// Reasons for removing a file the lockfile records but nothing produces any
// more. An orphaned artifact reuses them: from the project's side the file is
// gone from the kit either way, and one wording keeps the report uniform.
const (
	reasonRemoved       = "no longer part of this artifact"
	reasonRemovedEdited = "no longer part of this artifact, but edited locally"
)

// Action is one filesystem mutation, or the decision not to make one.
type Action struct {
	Kind       Kind
	ArtifactID string
	Path       string // project-relative, slash-separated
	Reason     string

	content []byte // source content for Create and Overwrite
	hash    string
}

// Plan is the full set of actions for one sync.
type Plan struct {
	Actions []Action
	Deps    map[string]string // npm dependency -> semver range

	owned map[string][]lockfile.OwnedFile // artifact ID -> files after apply
}

// Input is everything Build needs to compute a plan.
type Input struct {
	Artifacts  []kit.Artifact // resolved, dependencies-first
	Lock       *lockfile.File
	KitDir     string
	ProjectDir string
	Roots      map[string]string

	// Rewrites maps an import prefix the kit authors against to the prefix the
	// project uses. See internal/plan/rewrite.go.
	Rewrites map[string]string

	// Orphans are ids the lockfile records that the manifest no longer
	// declares, already separated out by kit.Manifest.PartitionInstalled.
	// They have no Artifact to install from, so everything they own is
	// removed under the same rule that removes a file an artifact stopped
	// producing.
	Orphans []string
}

// Build computes the plan without touching the project.
func Build(in Input) (*Plan, error) {
	p := &Plan{
		Deps:  kit.NPMDeps(in.Artifacts),
		owned: map[string][]lockfile.OwnedFile{},
	}

	for _, a := range in.Artifacts {
		pairs, err := filePairs(in.KitDir, in.Roots, a)
		if err != nil {
			return nil, fmt.Errorf("artifact %q: %w", a.ID, err)
		}

		produced := map[string]bool{}
		for _, fp := range pairs {
			produced[fp.dest] = true
			act, err := classify(in, a, fp)
			if err != nil {
				return nil, err
			}
			p.Actions = append(p.Actions, act)
			if act.Kind != Blocked {
				p.owned[a.ID] = append(p.owned[a.ID], lockfile.OwnedFile{Path: fp.dest, Hash: act.hash})
			}
		}

		// A file the artifact used to own but no longer produces is removed —
		// but only when the project has not diverged from what we wrote.
		if err := p.planRemovals(in, a.ID, produced); err != nil {
			return nil, err
		}
	}

	// An orphaned artifact produces nothing, so every file it still owns goes
	// under exactly the same rule.
	for _, id := range in.Orphans {
		// Registering the id with no files is what tells Apply to drop the
		// lock entry; planRemovals adds files back if any are blocked.
		if _, ok := in.Lock.Artifact(id); !ok {
			continue
		}
		if _, recorded := p.owned[id]; !recorded {
			p.owned[id] = nil
		}
		if err := p.planRemovals(in, id, nil); err != nil {
			return nil, err
		}
	}

	sort.SliceStable(p.Actions, func(i, j int) bool {
		if p.Actions[i].ArtifactID != p.Actions[j].ArtifactID {
			return p.Actions[i].ArtifactID < p.Actions[j].ArtifactID
		}
		return p.Actions[i].Path < p.Actions[j].Path
	})
	return p, nil
}

// planRemovals plans the removal of every file the lockfile records for an
// artifact that the artifact no longer produces. produced is the set of
// destinations it still writes; for an orphaned artifact it is empty, so the
// whole record is removed. A file that diverged from the recorded hash is
// blocked and stays owned: deal-kit never deletes work it cannot restore.
func (p *Plan) planRemovals(in Input, id string, produced map[string]bool) error {
	prev, ok := in.Lock.Artifact(id)
	if !ok {
		return nil
	}
	for _, old := range prev.Files {
		if produced[old.Path] {
			continue
		}
		abs := filepath.Join(in.ProjectDir, filepath.FromSlash(old.Path))
		current, exists, err := lockfile.HashFile(abs)
		if err != nil {
			return err
		}
		switch {
		case !exists:
			// Already gone; nothing to do, and nothing to record.
		case current != old.Hash:
			p.Actions = append(p.Actions, Action{
				Kind: Blocked, ArtifactID: id, Path: old.Path,
				Reason: reasonRemovedEdited,
			})
			p.owned[id] = append(p.owned[id], old)
		default:
			p.Actions = append(p.Actions, Action{
				Kind: Delete, ArtifactID: id, Path: old.Path,
				Reason: reasonRemoved,
			})
		}
	}
	return nil
}

type pair struct {
	src  string // absolute path inside the kit checkout
	dest string // project-relative, slash-separated
}

// filePairs enumerates every file an artifact installs and where it lands.
func filePairs(kitDir string, roots map[string]string, a kit.Artifact) ([]pair, error) {
	srcAbs := filepath.Join(kitDir, filepath.FromSlash(a.Src))
	info, err := os.Stat(srcAbs)
	if err != nil {
		return nil, err
	}

	destBase := a.Dest
	switch a.Type {
	case "skill":
		destBase = a.SkillDir()
	case "command":
		destBase = a.CommandFile()
	case "agent":
		destBase = a.AgentFile()
	}
	resolved, err := paths.Resolve(destBase, roots)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []pair{{src: srcAbs, dest: resolved}}, nil
	}

	var out []pair
	err = filepath.WalkDir(srcAbs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcAbs, p)
		if err != nil {
			return err
		}
		out = append(out, pair{src: p, dest: path.Join(resolved, filepath.ToSlash(rel))})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].dest < out[j].dest })
	return out, nil
}

// classify decides what happens to one destination file.
func classify(in Input, a kit.Artifact, fp pair) (Action, error) {
	content, err := os.ReadFile(fp.src)
	if err != nil {
		return Action{}, err
	}
	content = newRewriter(in.Rewrites).apply(content)
	srcHash := lockfile.Hash(content)
	base := Action{ArtifactID: a.ID, Path: fp.dest, content: content, hash: srcHash}

	abs := filepath.Join(in.ProjectDir, filepath.FromSlash(fp.dest))
	current, exists, err := lockfile.HashFile(abs)
	if err != nil {
		return Action{}, err
	}

	if !exists {
		base.Kind = Create
		return base, nil
	}

	recorded, owned := in.Lock.RecordedHash(fp.dest)
	if !owned {
		// The file is there but deal-kit never wrote it. Refuse: it belongs to
		// the project, and overwriting it would destroy work we cannot restore.
		base.Kind = Blocked
		base.Reason = "file exists but is not managed by deal-kit"
		return base, nil
	}
	if current != recorded {
		base.Kind = Blocked
		base.Reason = "edited locally since deal-kit wrote it"
		return base, nil
	}
	if current == srcHash {
		base.Kind = Unchanged
		return base, nil
	}
	base.Kind = Overwrite
	return base, nil
}

// Blocked returns the actions that need a human decision.
func (p *Plan) Blocked() []Action {
	var out []Action
	for _, a := range p.Actions {
		if a.Kind == Blocked {
			out = append(out, a)
		}
	}
	return out
}

// Changes returns the actions that would modify the project.
func (p *Plan) Changes() []Action {
	var out []Action
	for _, a := range p.Actions {
		switch a.Kind {
		case Create, Overwrite, Delete:
			out = append(out, a)
		}
	}
	return out
}

// Apply writes the plan to disk. It refuses to run while anything is blocked,
// so a local edit is never lost to a partially applied sync.
func (p *Plan) Apply(projectDir string, lock *lockfile.File) error {
	if b := p.Blocked(); len(b) > 0 {
		return fmt.Errorf("%d file(s) need attention before applying; run `deal-kit status` to see them", len(b))
	}

	for _, a := range p.Actions {
		abs := filepath.Join(projectDir, filepath.FromSlash(a.Path))
		switch a.Kind {
		case Create, Overwrite:
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(abs, a.content, 0o644); err != nil {
				return err
			}
		case Delete:
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return err
			}
			pruneEmptyDirs(projectDir, filepath.Dir(abs))
		}
	}

	for id, files := range p.owned {
		if len(files) == 0 {
			lock.Remove(id)
			continue
		}
		lock.Set(lockfile.Installed{ID: id, Files: files})
	}
	return nil
}

// pruneEmptyDirs removes directories left empty by a delete, stopping at the
// project root so it can never walk out of the project.
func pruneEmptyDirs(projectDir, dir string) {
	root := filepath.Clean(projectDir)
	for {
		cur := filepath.Clean(dir)
		if cur == root || !strings.HasPrefix(cur, root+string(filepath.Separator)) {
			return
		}
		entries, err := os.ReadDir(cur)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(cur); err != nil {
			return
		}
		dir = filepath.Dir(cur)
	}
}
