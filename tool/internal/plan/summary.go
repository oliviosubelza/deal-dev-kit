package plan

import (
	"path"
	"sort"
	"strings"
)

// DirSummary is what happened inside one destination directory.
type DirSummary struct {
	Dir         string // project-relative, or the file itself when it sits at the root
	Created     int
	Overwritten int
	Deleted     int
}

// Total is how many files the entry accounts for.
func (d DirSummary) Total() int { return d.Created + d.Overwritten + d.Deleted }

// maxSummaryRows keeps the closing summary to something a reader takes in at a
// glance. Beyond it, directories are folded into their parents.
const maxSummaryRows = 8

// Summarize folds a plan's changes into a short per-directory report.
//
// Listing 74 paths tells the reader nothing they can hold; what they need is
// where the work landed. Depth shrinks until the report fits, so a wide change
// collapses to its roots instead of scrolling.
func Summarize(actions []Action) []DirSummary {
	for depth := 3; depth >= 1; depth-- {
		rows := foldSingletons(summarizeAtDepth(actions, depth))
		if len(rows) <= maxSummaryRows || depth == 1 {
			return rows
		}
	}
	return nil
}

// foldSingletons merges sibling directories that each hold a single file into
// their shared parent. Four skills listed as four one-file directories say
// less than one line reading ".claude/skills  4 new", and cost four rows.
func foldSingletons(rows []DirSummary) []DirSummary {
	parents := map[string]int{}
	for _, r := range rows {
		if r.Total() == 1 {
			parents[path.Dir(r.Dir)]++
		}
	}

	merged := map[string]*DirSummary{}
	var order []string
	for _, r := range rows {
		key := r.Dir
		// Only fold where it actually buys a row, and never up to the root:
		// a bare "." names nothing the reader can act on.
		if parent := path.Dir(r.Dir); r.Total() == 1 && parents[parent] > 1 && parent != "." {
			key = parent
		}
		d, ok := merged[key]
		if !ok {
			d = &DirSummary{Dir: key}
			merged[key] = d
			order = append(order, key)
		}
		d.Created += r.Created
		d.Overwritten += r.Overwritten
		d.Deleted += r.Deleted
	}

	sort.Strings(order)
	out := make([]DirSummary, 0, len(order))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out
}

func summarizeAtDepth(actions []Action, depth int) []DirSummary {
	byDir := map[string]*DirSummary{}
	var order []string

	for _, a := range actions {
		switch a.Kind {
		case Create, Overwrite, Delete:
		default:
			continue
		}

		key := groupKey(a.Path, depth)
		d, ok := byDir[key]
		if !ok {
			d = &DirSummary{Dir: key}
			byDir[key] = d
			order = append(order, key)
		}
		switch a.Kind {
		case Create:
			d.Created++
		case Overwrite:
			d.Overwritten++
		case Delete:
			d.Deleted++
		}
	}

	sort.Strings(order)
	out := make([]DirSummary, 0, len(order))
	for _, k := range order {
		out = append(out, *byDir[k])
	}
	return out
}

// groupKey is the directory a path is reported under, trimmed to depth
// segments. A file directly at the project root is reported as itself, since
// naming its directory would just say ".".
func groupKey(p string, depth int) string {
	dir := path.Dir(p)
	if dir == "." || dir == "" {
		return p
	}
	segments := strings.Split(dir, "/")
	if len(segments) > depth {
		segments = segments[:depth]
	}
	return strings.Join(segments, "/")
}
