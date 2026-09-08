package plan

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/lockfile"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/paths"
)

// ensureLineAction decides what an artifact's `ensure_line` needs, without
// touching the project. Three cases, and no fourth:
//
//	file absent          -> AppendLine, and Apply creates it holding the line
//	file has the line    -> Unchanged
//	file lacks the line  -> AppendLine, appended after everything already there
//
// It can never return Blocked. Every other destination blocks when the project
// owns the file, because writing it would destroy work the CLI cannot restore.
// Here the project owning the file is the normal case: the CLI adds one line
// and rewrites nothing, so there is no work to lose and nothing to refuse.
func ensureLineAction(in Input, a kit.Artifact) (Action, lockfile.EnsuredLine, bool, error) {
	if a.EnsureLine == nil {
		return Action{}, lockfile.EnsuredLine{}, false, nil
	}
	dest, err := paths.Resolve(a.EnsureLine.File, in.Roots)
	if err != nil {
		return Action{}, lockfile.EnsuredLine{}, false, err
	}
	line := a.EnsureLine.Line

	data, err := os.ReadFile(filepath.Join(in.ProjectDir, filepath.FromSlash(dest)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Action{}, lockfile.EnsuredLine{}, false, err
	}

	act := Action{ArtifactID: a.ID, Path: dest, Line: line}
	switch {
	case err == nil && hasLine(data, line):
		act.Kind = Unchanged
	case err == nil:
		act.Kind = AppendLine
		act.Reason = reasonLineMissing
	default:
		act.Kind = AppendLine
		act.Reason = reasonLineFileMissing
	}
	return act, lockfile.EnsuredLine{Path: dest, Line: line}, true, nil
}

const (
	reasonLineMissing     = "falta la línea de import"
	reasonLineFileMissing = "el archivo no existe todavía"
)

// hasLine reports whether the content already carries the line as a line of
// its own. Comparison is per line and space-trimmed — a project that indented
// the import, or that ends its lines with CRLF, already has it, and appending
// a second copy would be the tool arguing with the file. A substring match
// would be wrong in the other direction: "@.claude/persona.md.bak" contains
// the line but is not it.
func hasLine(content []byte, line string) bool {
	want := strings.TrimSpace(line)
	for got := range strings.SplitSeq(string(content), "\n") {
		if strings.TrimSpace(got) == want {
			return true
		}
	}
	return false
}

// appendLine guarantees the line is present in the file at abs, leaving every
// byte already there exactly where it was. A missing file is created holding
// the line; a file that lacks a trailing newline gets one first, so the append
// never joins itself onto the project's last line.
//
// The presence check runs again here rather than trusting the planned Kind:
// Apply must converge even if the file changed between planning and writing,
// and a duplicated import line is the one failure mode this feature could
// plausibly produce.
func appendLine(abs, line string) error {
	data, err := os.ReadFile(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		return os.WriteFile(abs, []byte(line+"\n"), 0o644)
	case err != nil:
		return err
	}
	if hasLine(data, line) {
		return nil
	}

	// Keep whatever mode the project gave the file: this is its file, and the
	// CLI is a guest in it.
	perm := fs.FileMode(0o644)
	if info, err := os.Stat(abs); err == nil {
		perm = info.Mode().Perm()
	}

	out := data
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, line...)
	out = append(out, '\n')
	return os.WriteFile(abs, out, perm)
}
