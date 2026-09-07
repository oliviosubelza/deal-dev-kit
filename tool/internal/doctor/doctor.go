// Package doctor reports which external tools are available, so a failure
// surfaces as "pnpm is not installed" rather than as a generator crashing
// halfway through creating a project.
package doctor

import (
	"os/exec"
	"regexp"
	"strings"
)

// Tool is an external program deal-kit or a project generator depends on.
type Tool struct {
	Name     string
	Purpose  string
	Required bool
	// VersionArgs gets the version. Empty means the tool is only probed for
	// presence, which is all some programs reliably support.
	VersionArgs []string
}

// Result is what was found for one tool.
type Result struct {
	Tool
	Found   bool
	Path    string
	Version string
}

// OK reports whether this result blocks work.
func (r Result) OK() bool { return r.Found || !r.Required }

// Report is the outcome of a check.
type Report struct {
	Results []Result
}

// Missing lists the required tools that were not found.
func (rep Report) Missing() []Result {
	var out []Result
	for _, r := range rep.Results {
		if r.Required && !r.Found {
			out = append(out, r)
		}
	}
	return out
}

// OK reports whether every required tool is present.
func (rep Report) OK() bool { return len(rep.Missing()) == 0 }

// Found reports whether a named tool is available.
func (rep Report) Found(name string) bool {
	for _, r := range rep.Results {
		if r.Name == name {
			return r.Found
		}
	}
	return false
}

// Core is what deal-kit itself needs.
func Core() []Tool {
	return []Tool{
		{Name: "git", Purpose: "descargar el kit", Required: true, VersionArgs: []string{"--version"}},
	}
}

// ForWeb is what creating and running a React + Vite project needs.
func ForWeb() []Tool {
	return []Tool{
		{Name: "git", Purpose: "descargar el kit", Required: true, VersionArgs: []string{"--version"}},
		{Name: "node", Purpose: "ejecutar las herramientas", Required: true, VersionArgs: []string{"--version"}},
		// One package manager is required, but which one is the project's
		// choice, so each is optional on its own and the caller checks that at
		// least one turned up.
		{Name: "pnpm", Purpose: "instalar dependencias", VersionArgs: []string{"--version"}},
		{Name: "npm", Purpose: "instalar dependencias", VersionArgs: []string{"--version"}},
		// Claude Code and Engram are editor tooling, not project tooling: the
		// same developer uses them on backend, web and mobile alike. cli.Doctor
		// calls ForWeb for every project type, and reporting them everywhere is
		// the intended result, not a leak. Both stay optional — Required keeps
		// its zero value — so a machine without them still passes.
		{Name: "claude", Purpose: "instalar el plugin Engram", VersionArgs: []string{"--version"}},
		{Name: "engram", Purpose: "memoria persistente del agente", VersionArgs: []string{"--version"}},
	}
}

// Check probes each tool on the current PATH.
func Check(tools []Tool) Report {
	rep := Report{Results: make([]Result, 0, len(tools))}
	for _, t := range tools {
		r := Result{Tool: t}
		if path, err := exec.LookPath(t.Name); err == nil {
			r.Found, r.Path = true, path
			r.Version = version(t.Name, t.VersionArgs)
		}
		rep.Results = append(rep.Results, r)
	}
	return rep
}

var versionPattern = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

// version runs the tool and extracts the first version-looking number.
// Output formats differ wildly ("git version 2.43.0", "v20.11.0", "9.1.0"), so
// a pattern is more reliable than parsing each one.
func version(name string, args []string) string {
	if len(args) == 0 {
		return ""
	}
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	if m := versionPattern.FindString(line); m != "" {
		return m
	}
	return ""
}

// PackageManager returns the preferred package manager that is installed.
// pnpm comes first because it is what the kit's own lockfile uses.
func (rep Report) PackageManager() (string, bool) {
	for _, name := range []string{"pnpm", "npm"} {
		if rep.Found(name) {
			return name, true
		}
	}
	return "", false
}
