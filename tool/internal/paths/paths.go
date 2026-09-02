// Package paths resolves the destination templates used in kit.yaml against a
// concrete project. A template names a well-known root ({src}, {ui},
// {features}, {modules}) that the project type defines; `deal-kit init`
// records the resolved values in deal-kit.lock, so a project with a
// non-standard layout can override them without touching kit.yaml.
package paths

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var placeholder = regexp.MustCompile(`\{([a-z_]+)\}`)

// Resolve expands the {root} placeholders in a destination template.
// The result is a slash-separated path relative to the project root.
func Resolve(template string, roots map[string]string) (string, error) {
	var missing []string
	out := placeholder.ReplaceAllStringFunc(template, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := roots[name]
		if !ok {
			missing = append(missing, name)
			return m
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("unknown root %s in %q", strings.Join(quoteAll(missing), ", "), template)
	}
	clean := path.Clean(out)
	if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("destination %q escapes the project directory", template)
	}
	return clean, nil
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}
