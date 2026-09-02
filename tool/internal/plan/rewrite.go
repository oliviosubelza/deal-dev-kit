package plan

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// rewriter rewrites import specifiers as a file is installed.
//
// The kit authors components against its own layout (`@/components/ui/button`),
// but a project puts them somewhere else (`@/shared/ui/button`). Without this,
// every installed component would import a path that does not exist.
type rewriter struct {
	prefixes []string // longest first, so a specific rule wins over a general one
	rules    map[string]string
}

func newRewriter(rules map[string]string) rewriter {
	prefixes := make([]string, 0, len(rules))
	for from := range rules {
		prefixes = append(prefixes, from)
	}
	// Longest first: "@/components/ui/" must win over a hypothetical
	// "@/components/". Ties break alphabetically so the result is stable.
	sort.Slice(prefixes, func(i, j int) bool {
		if len(prefixes[i]) != len(prefixes[j]) {
			return len(prefixes[i]) > len(prefixes[j])
		}
		return prefixes[i] < prefixes[j]
	})
	return rewriter{prefixes: prefixes, rules: rules}
}

// apply rewrites the content of one file. Binary content is passed through
// untouched, and so is anything when there are no rules.
func (r rewriter) apply(content []byte) []byte {
	if len(r.prefixes) == 0 || !utf8.Valid(content) {
		return content
	}

	s := string(content)
	var b strings.Builder
	b.Grow(len(s))

	// Only rewrite inside quoted strings: a prefix appearing in prose or a
	// comment is not an import and must be left alone.
	for i := 0; i < len(s); {
		q := strings.IndexAny(s[i:], "\"'`")
		if q < 0 {
			b.WriteString(s[i:])
			break
		}
		q += i
		quote := s[q]
		end := strings.IndexByte(s[q+1:], quote)
		if end < 0 {
			b.WriteString(s[i:])
			break
		}
		end += q + 1

		b.WriteString(s[i : q+1])
		b.WriteString(r.rewriteSpecifier(s[q+1 : end]))
		b.WriteByte(quote)
		i = end + 1
	}
	return []byte(b.String())
}

// rewriteSpecifier replaces the first matching prefix in a quoted string.
func (r rewriter) rewriteSpecifier(spec string) string {
	for _, from := range r.prefixes {
		if strings.HasPrefix(spec, from) {
			return r.rules[from] + spec[len(from):]
		}
	}
	return spec
}
