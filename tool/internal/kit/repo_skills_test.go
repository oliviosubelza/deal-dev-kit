package kit

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// nonKitSymbols lists PascalCase names that are allowed to appear in the
// scanned regions of a SKILL.md without being a ui-kit export — React or
// TypeScript built-ins, or third-party component names the catalog only
// mentions. It is empty today: the two scanned regions name kit exports and
// nothing else. Keep it sorted, give every entry the reason it is here, and
// treat growth as a signal — more than a handful of entries means the
// extraction rule below is too broad and should be narrowed, not padded.
var nonKitSymbols = map[string]string{}

var (
	// `export { A, B as C, type D }` — the shape 60 of the ui-kit files use.
	tsExportBlock = regexp.MustCompile(`(?m)^export\s+(?:type\s+)?\{([^}]*)\}`)
	// `export function X`, `export const X`, `export interface X`, …
	tsExportDecl = regexp.MustCompile(`(?m)^export\s+(?:default\s+)?(?:async\s+)?(?:function|const|let|var|class|type|interface|enum)\s+([A-Za-z_$][\w$]*)`)

	mdCodeSpan = regexp.MustCompile("`([^`]+)`")
	// Deliberately strict: a whole code span that is one PascalCase word and
	// nothing else. This is what excludes everything a backtick also wraps in
	// these skills — file paths, CSS classes, npm packages, props, commands,
	// HTML elements, Tailwind tokens and code fragments all fail it.
	pascalCase = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

	// The catalog table's header row. Structural, not a filename check: a
	// skill that does not carry this table contributes no symbols, which is
	// how mobile skills stay out (React Native Paper is not shipped here).
	catalogTableHeader = regexp.MustCompile(`^\|\s*Need\s*\|\s*Use\s*\|\s*$`)
	mdHeading          = regexp.MustCompile(`^#{1,6}\s+(.*?)\s*$`)
)

// compositionRulesHeading is the second scanned region. Like the catalog
// table it is matched by structure, so only a skill that actually documents
// the catalog's composition rules is checked.
const compositionRulesHeading = "Critical composition rules"

// TestSkillsOnlyNameRealKitExports reads the SKILL.md files that actually
// ship in this repository and asserts that every component name they hand a
// developer resolves to a real export of ui-kit.
//
// This drift is not hypothetical: the original catalog skill named three
// exports that do not exist (PortalContainer, Chart, Resizable), and a later
// revision claimed the catalog runs on Radix when 26 of its files import
// @base-ui/react. kit.yaml has had repo_manifest_test.go guarding it since
// the beginning; the prose had nothing.
//
// Scope is deliberately two structured regions — the `| Need | Use |` catalog
// table and the "Critical composition rules" section — where a backticked
// PascalCase word can only be a component. Everything else in these skills is
// prose, and a symbol check over prose cries wolf.
func TestSkillsOnlyNameRealKitExports(t *testing.T) {
	root := filepath.Join("..", "..", "..")

	exports, sources := kitExports(t, root)
	// A parser that silently stops matching would make this test pass by
	// finding nothing to check. Both ends are asserted non-trivial.
	if len(exports) < 100 {
		t.Fatalf("only %d exports parsed from ui-kit: the export parser is broken, not the skills", len(exports))
	}

	skills := skillFiles(t, root)
	if len(skills) == 0 {
		t.Fatal("no SKILL.md found under skills/")
	}

	checked := 0
	for _, rel := range skills {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		for _, ref := range documentedSymbols(string(data)) {
			checked++
			if _, ok := exports[ref.symbol]; ok {
				continue
			}
			if why, ok := nonKitSymbols[ref.symbol]; ok {
				t.Logf("%s:%d: %q allowed as a non-kit symbol (%s)", rel, ref.line, ref.symbol, why)
				continue
			}
			t.Errorf("%s:%d: %s names %q, which no ui-kit source exports.\n"+
				"\tExpected an `export` of %q in one of %s.\n"+
				"\tEither the symbol is misspelled or renamed in the source, or the component does not exist.\n"+
				"\tIf it is a legitimate non-kit name, add it to nonKitSymbols in %s with the reason.",
				rel, ref.line, ref.region, ref.symbol,
				ref.symbol, strings.Join(sources, ", "), "tool/internal/kit/repo_skills_test.go")
		}
	}

	if checked == 0 {
		t.Fatal("no documented component names extracted from any SKILL.md: the extractor is broken, so this test proves nothing")
	}
	t.Logf("checked %d documented component names across %d SKILL.md files against %d ui-kit exports", checked, len(skills), len(exports))
}

// symbolRef is one PascalCase name found in a scanned region.
type symbolRef struct {
	symbol string
	line   int
	region string
}

// documentedSymbols extracts the component names a SKILL.md hands a
// developer, from the two structured regions only.
func documentedSymbols(md string) []symbolRef {
	var refs []symbolRef
	seen := map[string]bool{}

	inCompositionRules := false
	inCatalogTable := false

	for i, line := range strings.Split(md, "\n") {
		if m := mdHeading.FindStringSubmatch(line); m != nil {
			inCompositionRules = m[1] == compositionRulesHeading
			inCatalogTable = false
		}
		if catalogTableHeader.MatchString(line) {
			inCatalogTable = true
			continue // the header row itself names no component
		}
		if inCatalogTable && !strings.HasPrefix(line, "|") {
			inCatalogTable = false
		}

		region := ""
		switch {
		case inCatalogTable:
			region = "the component catalog table"
		case inCompositionRules:
			region = "the " + compositionRulesHeading + " section"
		default:
			continue
		}

		for _, m := range mdCodeSpan.FindAllStringSubmatch(line, -1) {
			sym := m[1]
			if !pascalCase.MatchString(sym) || seen[sym] {
				continue
			}
			seen[sym] = true
			refs = append(refs, symbolRef{symbol: sym, line: i + 1, region: region})
		}
	}
	return refs
}

// kitExports parses every export out of the ui-kit sources, so the expected
// set is derived from the code on every run instead of a hardcoded list that
// would go stale exactly the way the skills did.
func kitExports(t *testing.T, root string) (map[string]bool, []string) {
	t.Helper()

	exports := map[string]bool{}
	dirs := map[string]bool{}

	uiKit := filepath.Join(root, "ui-kit")
	err := filepath.WalkDir(uiKit, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(uiKit, path)
		if relErr == nil {
			dirs[filepath.ToSlash(filepath.Dir(filepath.Join("ui-kit", rel)))] = true
		}
		src := string(data)
		for _, m := range tsExportBlock.FindAllStringSubmatch(src, -1) {
			for _, part := range strings.Split(m[1], ",") {
				part = strings.TrimSpace(part)
				part = strings.TrimPrefix(part, "type ")
				if idx := strings.Index(part, " as "); idx >= 0 {
					part = strings.TrimSpace(part[idx+len(" as "):])
				}
				if part != "" {
					exports[strings.TrimSpace(part)] = true
				}
			}
		}
		for _, m := range tsExportDecl.FindAllStringSubmatch(src, -1) {
			exports[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading ui-kit sources: %v", err)
	}

	sources := make([]string, 0, len(dirs))
	for d := range dirs {
		sources = append(sources, d+"/*.ts(x)")
	}
	sort.Strings(sources)
	return exports, sources
}

// skillFiles lists every SKILL.md under skills/, repository-relative.
func skillFiles(t *testing.T, root string) []string {
	t.Helper()

	var out []string
	err := filepath.WalkDir(filepath.Join(root, "skills"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking skills/: %v", err)
	}
	sort.Strings(out)
	return out
}
