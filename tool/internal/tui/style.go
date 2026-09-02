package tui

import "github.com/charmbracelet/lipgloss"

// The palette is deep purple on near-black. Colours are fixed rather than
// adaptive because the design paints its own surface.
//
// State never depends on colour alone: every marker that carries meaning also
// differs in glyph, so the interface reads in a monochrome terminal and for a
// colour-blind reader.
const (
	colBg      = lipgloss.Color("#0D0A14")
	colRow     = lipgloss.Color("#1B1428")
	colPurple  = lipgloss.Color("#C4A7FF")
	colViolet  = lipgloss.Color("#8B5CF6")
	colMagenta = lipgloss.Color("#F0A7D8")
	colText    = lipgloss.Color("#E8E2F5")
	colMuted   = lipgloss.Color("#7A6E96")
	colFaint   = lipgloss.Color("#4A4063")
	colGood    = lipgloss.Color("#8FE3B4")
	colWarn    = lipgloss.Color("#F5C97B")
	colBad     = lipgloss.Color("#F58A93")
)

var (
	bg = lipgloss.NewStyle().Background(colBg)

	panel = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colViolet).
		BorderBackground(colBg).
		Background(colBg).
		Foreground(colText).
		Padding(1, 3)

	titleText = bg.Foreground(colPurple).Bold(true)
	subtle    = bg.Foreground(colMuted)
	faintText = bg.Foreground(colFaint)
	bodyText  = bg.Foreground(colText)
	section   = bg.Foreground(colMagenta).Bold(true)

	rowCursor = lipgloss.NewStyle().Background(colRow).Foreground(colText)
	pick      = lipgloss.NewStyle().Background(colRow).Foreground(colPurple).Bold(true)

	goodText = bg.Foreground(colGood)
	warnText = bg.Foreground(colWarn)
	badText  = bg.Foreground(colBad)

	keyCap  = bg.Foreground(colPurple)
	keyText = bg.Foreground(colFaint)
)

// rowStyle returns the base style for a list line.
func rowStyle(cursor bool) lipgloss.Style {
	if cursor {
		return rowCursor
	}
	return bg
}
