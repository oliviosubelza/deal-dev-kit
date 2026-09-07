// Package execenv builds the environment every external command deal-kit runs
// gets. It exists because the same list lived in two packages — internal/kit's
// git calls and internal/engram's `claude plugin marketplace add` — and a
// security list kept in two copies drifts the first time only one of them is
// fixed.
package execenv

import (
	"os"
	"strings"
)

// LeakedGitVars are the environment variables that redirect a git command away
// from the repository it was pointed at. deal-kit normally runs from inside a
// project checkout, so these are frequently set, and inheriting them would
// send the kit cache fetch — or the marketplace clone `claude` performs — into
// that project's repository instead.
var LeakedGitVars = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_NAMESPACE", "GIT_CEILING_DIRECTORIES", "GIT_GRAFT_FILE",
	"GIT_REPLACE_REF_BASE", "GIT_PREFIX",
}

// Sanitized is the process environment with LeakedGitVars removed and the
// interactive credential prompt disabled: a git that stops to ask for a
// password has nothing to read it, and would hang the terminal.
func Sanitized() []string {
	drop := make(map[string]bool, len(LeakedGitVars))
	for _, k := range LeakedGitVars {
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
	return append(out, "GIT_TERMINAL_PROMPT=0")
}
