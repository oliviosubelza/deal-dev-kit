package tui

import "sort"

// check is the tri-state of a checkbox: a group is partially selected when
// some but not all of its items are.
type check int

const (
	unchecked check = iota
	partial
	checked
)

// group is a heading with its artifacts underneath.
type group struct {
	name      string
	collapsed bool
	items     []int // indices into Model.items
}

// row is one visible line: a group heading, or an artifact.
type row struct {
	group int // index into Model.groups, or -1 on a flat screen
	item  int // index into Model.items, or -1 for a heading
}

func (r row) isHeading() bool { return r.item < 0 }

// buildGroups arranges the component items into ordered groups. Skills are
// excluded: they live on their own screen as a short flat list.
func buildGroups(all []item, components []item) []group {
	index := map[string]int{}
	for i, it := range all {
		index[it.id] = i
	}

	byName := map[string][]int{}
	for _, c := range components {
		byName[c.group] = append(byName[c.group], index[c.id])
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		pi, pj := groupRank(names[i]), groupRank(names[j])
		if pi != pj {
			return pi < pj
		}
		return names[i] < names[j]
	})

	out := make([]group, 0, len(names))
	for _, name := range names {
		idx := byName[name]
		sort.Slice(idx, func(a, b int) bool { return all[idx[a]].id < all[idx[b]].id })
		out = append(out, group{name: name, items: idx})
	}
	return out
}

// groupRank pins Foundation first: everything else depends on it.
func groupRank(name string) int {
	if name == "Foundation" {
		return 0
	}
	return 1
}

// groupState reports a group's tri-state, ignoring items hidden by the filter.
func (m Model) groupState(g group) check {
	total, selected := m.groupCounts(g)
	switch {
	case total == 0 || selected == 0:
		return unchecked
	case selected == total:
		return checked
	default:
		return partial
	}
}

// groupCounts reports how many of a group's visible items are selected.
func (m Model) groupCounts(g group) (total, selected int) {
	for _, i := range g.items {
		if !m.matches(i) {
			continue
		}
		total++
		if m.items[i].selected() {
			selected++
		}
	}
	return total, selected
}

// rows flattens the current screen into visible lines. The skills screen is a
// short flat list; components are a foldable tree.
func (m Model) rows() []row {
	if m.screen == screenSkills {
		var out []row
		for _, i := range m.skills {
			if m.matches(i) {
				out = append(out, row{group: -1, item: i})
			}
		}
		return out
	}

	var out []row
	for gi, g := range m.groups {
		var visible []int
		for _, i := range g.items {
			if m.matches(i) {
				visible = append(visible, i)
			}
		}
		if len(visible) == 0 {
			continue
		}
		out = append(out, row{group: gi, item: -1})
		if g.collapsed && m.filter == "" {
			continue
		}
		for _, i := range visible {
			out = append(out, row{group: gi, item: i})
		}
	}
	return out
}
