package kit

import (
	"fmt"
	"strings"
)

// frontmatterBlock extracts the leading frontmatter block from data — the
// region between the first two `---` fence lines — and returns it as a set
// of key/value pairs. Only top-level `key: value` lines are parsed; nested
// YAML structures are not needed by the checks in this file.
//
// It returns an error if data has no opening `---` fence, or if the block
// is never terminated by a closing `---` fence.
func frontmatterBlock(data []byte) (map[string]string, error) {
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("no frontmatter block found (file must start with a %q fence)", "---")
	}

	fields := map[string]string{}
	closed := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			closed = true
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = trimQuotes(strings.TrimSpace(value))
	}
	if !closed {
		return nil, fmt.Errorf("frontmatter block is not terminated (missing closing %q fence)", "---")
	}
	return fields, nil
}

// trimQuotes strips a single matching pair of surrounding single or double
// quotes from a frontmatter value, e.g. `"web-ui"` -> `web-ui`. YAML permits
// quoting a scalar value; no existing skill quotes its name today, but a
// quoted value is still valid input and should not fail comparison against
// an unquoted expectation.
func trimQuotes(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// CheckFrontmatterName verifies that a skill's or agent's frontmatter
// declares a `name` matching the artifact's flattened install name. A
// mismatch means the artifact lands under one name but is looked up under
// another — the drift bug class this check exists to catch.
func CheckFrontmatterName(data []byte, a Artifact) error {
	want := a.InstallName()
	fields, err := frontmatterBlock(data)
	if err != nil {
		return fmt.Errorf("artifact %q: %w", a.ID, err)
	}
	got, ok := fields["name"]
	if !ok || got == "" {
		return fmt.Errorf("artifact %q: frontmatter must declare name: %q", a.ID, want)
	}
	if got != want {
		return fmt.Errorf("artifact %q: frontmatter must declare name: %q, got %q", a.ID, want, got)
	}
	return nil
}

// CheckDescriptionPresent verifies that a command's frontmatter declares a
// non-empty `description`. Commands have no `name` field to drift — their
// filename is derived from InstallName() — so this is the only frontmatter
// identity check that applies to them.
func CheckDescriptionPresent(data []byte) error {
	fields, err := frontmatterBlock(data)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if value, ok := fields["description"]; !ok || value == "" {
		return fmt.Errorf("frontmatter must declare a description")
	}
	return nil
}
