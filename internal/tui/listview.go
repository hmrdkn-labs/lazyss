package tui

import (
	"fmt"
	"strings"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func (m Model) machineList(width, height int) string {
	header := panelTitleStyle.Render(fmt.Sprintf("Machines (%d)", len(m.visible)))
	columns := faintStyle.Render(m.listHeader(width))
	lines := []string{header, columns}
	if len(m.visible) == 0 {
		lines = append(lines, m.emptyListLines(width)...)
		return strings.Join(lines, "\n")
	}
	rows := height - 3
	if rows < 1 {
		rows = 1
	}
	start := 0
	if m.cursor >= rows {
		start = m.cursor - rows + 1
	}
	end := start + rows
	if end > len(m.visible) {
		end = len(m.visible)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.machineRow(i, m.visible[i], width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) listHeader(width int) string {
	if width >= 98 {
		return fmt.Sprintf("  %-8s %-22s %-26s %-15s %-24s %-16s", "Provider", "Name", "Address", "Method", "Health", "Last connected")
	}
	return fmt.Sprintf("  %-6s %-20s %-14s %-20s", "Provider", "Name", "Method", "Health")
}

func (m Model) machineRow(index int, machine domain.Machine, width int) string {
	pin := " "
	if machine.Pinned {
		pin = "*"
	}
	name := nonempty(machine.Name, string(machine.ID))
	health := nonempty(machine.Health.Label, "not checked")
	line := ""
	if width >= 98 {
		line = fmt.Sprintf("%s %-8s %-22s %-26s %-15s %-24s %-16s",
			pin,
			machine.Provider,
			displayFit(name, 22),
			displayFit(nonempty(machine.Address, machine.NativeID), 26),
			displayFit(string(machine.DefaultMethod()), 15),
			displayFit(health, 24),
			displayFit(rel(machine.LastConnectedAt), 16),
		)
	} else {
		line = fmt.Sprintf("%s %-6s %-20s %-14s %-20s",
			pin,
			machine.Provider,
			displayFit(name, 20),
			displayFit(string(machine.DefaultMethod()), 14),
			displayFit(health, 20),
		)
	}
	line = displayPadRight(line, width)
	if index == m.cursor {
		return selectedStyle.Width(width).Render(line)
	}
	if machine.Health.Status == domain.HealthUp {
		return goodStyle.Render(line)
	}
	if machine.Health.Status == domain.HealthDown {
		return badStyle.Render(line)
	}
	return line
}

func (m Model) compactList(width, height int) string {
	lines := []string{panelTitleStyle.Render(fmt.Sprintf("Machines (%d)", len(m.visible)))}
	if len(m.visible) == 0 {
		lines = append(lines, m.emptyListLines(width)...)
		return strings.Join(lines, "\n")
	}
	rows := height - 1
	for i, machine := range m.visible {
		if i >= rows {
			break
		}
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		line := prefix + string(machine.Provider) + " " + nonempty(machine.Name, string(machine.ID)) + " " + string(machine.DefaultMethod()) + " " + nonempty(machine.Health.Label, "not checked")
		lines = append(lines, displayFit(line, width))
	}
	return strings.Join(lines, "\n")
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
