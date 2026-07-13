package tui

import (
	"fmt"
	"strings"

	"github.com/hmrdkn-labs/lazyss/internal/brand"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func (m Model) detailPanel(width, height int) string {
	lines := []string{panelTitleStyle.Render("Details")}
	if len(m.visible) == 0 {
		lines = append(lines, m.emptyDetailLines(width)...)
		return strings.Join(lines, "\n")
	}
	machine := m.visible[m.cursor]
	lines = append(lines,
		displayFit(nonempty(machine.Name, string(machine.ID)), width),
		faintStyle.Render(displayFit(string(machine.ID), width)),
		"",
		m.detailLine("Provider", string(machine.Provider), width),
		m.detailLine("Native", machine.NativeID, width),
		m.detailLine("Address", machine.Address, width),
		m.detailLine("Method", string(machine.DefaultMethod()), width),
		m.detailLine("Health", nonempty(machine.Health.Label, "not checked"), width),
		m.detailLine("Last checked", rel(machine.LastCheckedAt), width),
		m.detailLine("Last connected", rel(machine.LastConnectedAt), width),
		m.detailLine("Connections", fmt.Sprintf("%d", machine.ConnectionCount), width),
		m.detailLine("Hidden", fmt.Sprintf("%t", machine.Hidden), width),
	)
	if machine.Scope.Profile != "" || machine.Scope.Region != "" || machine.Scope.Account != "" {
		lines = append(lines, "", panelTitleStyle.Render("Scope"))
		lines = append(lines,
			m.detailLine("Profile", machine.Scope.Profile, width),
			m.detailLine("Region", machine.Scope.Region, width),
			m.detailLine("Account", machine.Scope.Account, width),
		)
	}
	if len(machine.ProviderTags) > 0 {
		lines = append(lines, "", panelTitleStyle.Render("Tags"))
		for _, tag := range sortedProviderTags(machine.ProviderTags) {
			lines = append(lines, displayFit(tag, width))
		}
	}
	if machine.Note != "" {
		lines = append(lines, "", panelTitleStyle.Render("Note"), displayFit(machine.Note, width))
	}
	lines = append(lines, "", panelTitleStyle.Render("Recent health"))
	health := lastHealth(machine.HealthHistory, 3)
	if len(health) == 0 && machine.Health.Label != "" {
		health = []domain.HealthObservation{machine.Health}
	}
	if len(health) == 0 {
		lines = append(lines, dimStyle.Render("none"))
	}
	for _, obs := range health {
		lines = append(lines, displayFit(fmt.Sprintf("%s %s", rel(obs.CheckedAt), obs.Label), width))
	}
	if len(machine.SessionHistory) > 0 {
		lines = append(lines, "", panelTitleStyle.Render("Recent sessions"))
		for _, event := range lastSessions(machine.SessionHistory, 3) {
			outcome := "fail"
			if event.Success {
				outcome = "ok"
			}
			lines = append(lines, displayFit(fmt.Sprintf("%s %s %s", rel(event.EndedAt), event.Method, outcome), width))
		}
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) detailLine(key, value string, width int) string {
	if value == "" {
		value = "-"
	}
	keyWidth := 15
	if width < 34 {
		keyWidth = 12
	}
	return faintStyle.Render(displayPadRight(key, keyWidth)) + displayFit(value, width-keyWidth)
}

func (m Model) emptyDetailLines(width int) []string {
	if m.hasActiveFilter() {
		lines := []string{
			warnStyle.Render(displayFit("No matches", width)),
			m.detailLine("Filter", m.activeFilterLabel(), width),
			m.detailLine("Clear", "esc", width),
			m.detailLine("Edit", "f", width),
		}
		if line := availableTagFiltersLine(m.machines, width); line != "" {
			lines = append(lines, "", panelTitleStyle.Render("Available tags"), displayFit(strings.TrimPrefix(line, "Available tags: "), width))
		}
		return lines
	}
	lines := make([]string, 0, 16)
	for _, line := range brand.LogoLines() {
		lines = append(lines, faintStyle.Render(displayFit(line, width)))
	}
	lines = append(lines,
		"",
		panelTitleStyle.Render("Setup"),
		m.detailLine("AWS profile", "P", width),
		m.detailLine("AWS login", "L", width),
		m.detailLine("Source", "s", width),
		m.detailLine("Refresh", "r", width),
		m.detailLine("Filter", "f", width),
		m.detailLine("Doctor", "lazyss doctor", width),
	)
	return lines
}
