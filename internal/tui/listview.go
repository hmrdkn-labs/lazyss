package tui

import (
	"fmt"
	"strings"
)

func (m Model) machineList(width, height int) string {
	cols := allocColumns(width)
	header := panelTitleStyle.Render(fmt.Sprintf("Machines (%d)", len(m.visible)))
	lines := []string{header, faintStyle.Render(cols.header())}
	if len(m.visible) == 0 {
		lines = append(lines, m.emptyListLines(width)...)
		return strings.Join(lines, "\n")
	}
	start, end := m.window(height - 3)
	for i := start; i < end; i++ {
		lines = append(lines, m.row(i, m.visible[i], cols))
	}
	return strings.Join(lines, "\n")
}

func (m Model) compactList(width, height int) string {
	cols := allocColumns(width)
	lines := []string{panelTitleStyle.Render(fmt.Sprintf("Machines (%d)", len(m.visible)))}
	if len(m.visible) == 0 {
		lines = append(lines, m.emptyListLines(width)...)
		return strings.Join(lines, "\n")
	}
	lines = append(lines, faintStyle.Render(cols.header()))
	start, end := m.window(height - 2)
	for i := start; i < end; i++ {
		lines = append(lines, m.row(i, m.visible[i], cols))
	}
	return strings.Join(lines, "\n")
}

// windowRange returns the [start,end) slice of a total-length list that fits
// rows lines while keeping cursor on screen.
func windowRange(cursor, total, rows int) (int, int) {
	if rows < 1 {
		rows = 1
	}
	start := 0
	if cursor >= rows {
		start = cursor - rows + 1
	}
	end := start + rows
	if end > total {
		end = total
	}
	return start, end
}

// window keeps the cockpit list cursor visible within rows lines.
func (m Model) window(rows int) (int, int) {
	return windowRange(m.cursor, len(m.visible), rows)
}

func (m Model) emptyListLines(width int) []string {
	if m.hasActiveFilter() {
		lines := []string{
			warnStyle.Render(displayFit("No matches", width)),
			displayFit("Active: "+m.activeFilterLabel(), width),
			displayFit("Press esc to clear, f to edit, r to refresh", width),
		}
		if line := availableTagFiltersLine(m.machines, width); line != "" {
			lines = append(lines, displayFit(line, width))
		}
		return lines
	}
	lines := []string{
		warnStyle.Render(displayFit("Setup", width)),
		displayFit("No machines loaded yet.", width),
		displayFit("P profile | L login | s source | r refresh", width),
	}
	if m.runtime != nil && strings.TrimSpace(m.runtime.AWSProfile) == "" && m.shouldShowAWSOnboarding() {
		lines = append(lines, displayFit("Choose an AWS profile to load SSM machines.", width))
	}
	return lines
}
