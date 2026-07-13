package tui

import (
	"strings"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

// Column widths are derived from the panel's inner width so a single code path
// serves every terminal size; NAME claims the surplus the fixed columns leave.
const (
	colProvider = 3
	colMethod   = 13 // fits "aws-ssm-shell"
	colHealth   = 11 // glyph + space + "unchecked"
	colAddress  = 20
	colLastConn = 16 // "2006-01-02 15:04"
	colNameMin  = 8
	// Optional columns appear only while NAME keeps a readable width, so long
	// hostnames stay legible instead of being crowded out by metadata.
	nameComfort = 36
)

// listColumns holds the resolved width of each list column. A zero width marks
// an optional column that the current panel width cannot afford.
type listColumns struct {
	width         int
	name          int
	address       int
	lastConnected int
}

func allocColumns(width int) listColumns {
	c := listColumns{width: width}
	const baseGaps = 5 // separators among the six always-present cells
	fixed := 1 + 1 + colProvider + colMethod + colHealth
	c.name = width - fixed - baseGaps
	if c.name-(colLastConn+1) >= nameComfort {
		c.lastConnected = colLastConn
		c.name -= colLastConn + 1
	}
	if c.name-(colAddress+1) >= nameComfort {
		c.address = colAddress
		c.name -= colAddress + 1
	}
	if c.name < colNameMin {
		c.name = colNameMin
	}
	return c
}

func (c listColumns) header() string {
	cells := []string{" ", " ", displayPadRight("Prv", colProvider), displayPadRight("Name", c.name)}
	if c.address > 0 {
		cells = append(cells, displayPadRight("Address", c.address))
	}
	cells = append(cells, displayPadRight("Method", colMethod), displayPadRight("Health", colHealth))
	if c.lastConnected > 0 {
		cells = append(cells, displayPadRight("Last conn", c.lastConnected))
	}
	return displayPadRight(strings.Join(cells, " "), c.width)
}

func (m Model) row(index int, machine domain.Machine, c listColumns) string {
	marker := " "
	if index == m.cursor {
		marker = "›"
	}
	pin := " "
	if machine.Pinned {
		pin = "*"
	}
	glyph, label := healthGlyph(machine.Health.Status)
	cells := []string{
		marker,
		pin,
		displayPadRight(string(machine.Provider), colProvider),
		displayPadRight(nonempty(machine.Name, string(machine.ID)), c.name),
	}
	if c.address > 0 {
		cells = append(cells, displayPadRight(nonempty(machine.Address, machine.NativeID), c.address))
	}
	cells = append(cells,
		displayPadRight(string(machine.DefaultMethod()), colMethod),
		displayPadRight(glyph+" "+label, colHealth),
	)
	if c.lastConnected > 0 {
		cells = append(cells, displayPadRight(rel(machine.LastConnectedAt), c.lastConnected))
	}
	line := displayPadRight(strings.Join(cells, " "), c.width)
	if index == m.cursor {
		return selectedStyle.Width(c.width).Render(line)
	}
	switch machine.Health.Status {
	case domain.HealthUp:
		return goodStyle.Render(line)
	case domain.HealthDown:
		return badStyle.Render(line)
	default:
		return line
	}
}

// healthGlyph renders reachability as a shape (not colour alone) so state stays
// legible in monochrome terminals; the full label lives in the detail panel.
func healthGlyph(s domain.HealthStatus) (glyph, label string) {
	switch s {
	case domain.HealthUp:
		return "●", "up"
	case domain.HealthDown:
		return "×", "down"
	default:
		return "○", "unchecked"
	}
}

// fitLines forces s to exactly n lines so paired panels render identical border
// box heights regardless of how much content each holds.
func fitLines(s string, n int) string {
	if n < 0 {
		n = 0
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
