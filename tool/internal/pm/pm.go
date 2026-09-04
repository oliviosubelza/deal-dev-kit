// Package pm detects the JavaScript package manager a project uses and runs
// dependency installs with it, so a consumer never has to care which one the
// kit was authored with.
package pm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Manager is a supported JavaScript package manager.
type Manager string

const (
	PNPM Manager = "pnpm"
	NPM  Manager = "npm"
	Yarn Manager = "yarn"
	Bun  Manager = "bun"
)

// lockfiles maps a lockfile to the manager that writes it. A project's
// lockfile is the most reliable signal: it is what CI actually installs from.
var lockfiles = []struct {
	file    string
	manager Manager
}{
	{"pnpm-lock.yaml", PNPM},
	{"bun.lockb", Bun},
	{"bun.lock", Bun},
	{"yarn.lock", Yarn},
	{"package-lock.json", NPM},
}

// Detect identifies the package manager for a project directory. It prefers
// the lockfile, then the `packageManager` field in package.json. A project
// with neither returns false rather than a guess.
func Detect(projectDir string) (Manager, bool) {
	for _, lf := range lockfiles {
		if _, err := os.Stat(filepath.Join(projectDir, lf.file)); err == nil {
			return lf.manager, true
		}
	}

	data, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		return "", false
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false
	}
	// The field is "<name>@<version>", e.g. "pnpm@9.1.0".
	name := pkg.PackageManager
	if i := strings.Index(name, "@"); i > 0 {
		name = name[:i]
	}
	switch Manager(name) {
	case PNPM:
		return PNPM, true
	case NPM:
		return NPM, true
	case Yarn:
		return Yarn, true
	case Bun:
		return Bun, true
	}
	return "", false
}

// InstallArgs builds the command line that adds the given dependencies.
// Dependencies are sorted so the command is reproducible and reviewable.
func InstallArgs(m Manager, deps map[string]string) []string {
	specs := make([]string, 0, len(deps))
	for name, rng := range deps {
		specs = append(specs, name+"@"+rng)
	}
	sort.Strings(specs)

	switch m {
	case PNPM:
		return append([]string{"pnpm", "add"}, specs...)
	case Yarn:
		return append([]string{"yarn", "add"}, specs...)
	case Bun:
		return append([]string{"bun", "add"}, specs...)
	default:
		return append([]string{"npm", "install", "--save"}, specs...)
	}
}

// Install runs the dependency install in projectDir. Output is streamed to the
// caller's writer so the developer sees the package manager's own progress.
func Install(projectDir string, m Manager, deps map[string]string, out io.Writer) error {
	if len(deps) == 0 {
		return nil
	}
	args := InstallArgs(m, deps)
	if _, err := exec.LookPath(args[0]); err != nil {
		return fmt.Errorf("%s no está instalado: %w", args[0], err)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = projectDir
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
