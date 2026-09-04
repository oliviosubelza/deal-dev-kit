package tui

import (
	"fmt"
	"io"
	"os"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

func sortStrings(ss []string) { sort.Strings(ss) }

// IsTerminal reports whether w is an interactive terminal. A TUI started on a
// pipe or in CI would wait forever for a keypress, so callers fall back to the
// plain command path when this is false.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Run starts the interactive program and returns the finished model.
func Run(cfg Config) (Model, error) {
	p := tea.NewProgram(New(cfg))
	final, err := p.Run()
	if err != nil {
		return Model{}, err
	}
	m, ok := final.(Model)
	if !ok {
		return Model{}, fmt.Errorf("tipo de modelo inesperado %T", final)
	}
	return m, m.Err()
}
