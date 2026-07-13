package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

// historyLines renders every stored session and health entry for the machine,
// newest first, as plain content lines (no title or footer).
func historyLines(machine domain.Machine, width int) []string {
	lines := []string{panelTitleStyle.Render("Sessions")}
	if len(machine.SessionHistory) == 0 {
		lines = append(lines, dimStyle.Render("none"))
	}
	for i := len(machine.SessionHistory) - 1; i >= 0; i-- {
		event := machine.SessionHistory[i]
		outcome := "fail"
		if event.Success {
			outcome = "ok"
		}
		lines = append(lines, displayFit(fmt.Sprintf("%s  %s  %s", rel(event.EndedAt), event.Method, outcome), width))
	}
	lines = append(lines, "", panelTitleStyle.Render("Health"))
	if len(machine.HealthHistory) == 0 {
		lines = append(lines, dimStyle.Render("none"))
	}
	for i := len(machine.HealthHistory) - 1; i >= 0; i-- {
		obs := machine.HealthHistory[i]
		lines = append(lines, displayFit(fmt.Sprintf("%s  %s  %s", rel(obs.CheckedAt), obs.Method, nonempty(obs.Label, string(obs.Status))), width))
	}
	return lines
}

func (m Model) historyViewport() int {
	_, height := m.layoutSize()
	if vp := height - 3; vp > 1 {
		return vp
	}
	return 1
}

func (m Model) renderHistory() string {
	// An async inventory/health update can empty the visible set while the
	// history view is open; fall back to the cockpit rather than indexing.
	if len(m.visible) == 0 {
		return m.renderCockpit()
	}
	machine := m.visible[m.cursor]
	width, _ := m.layoutSize()
	vp := m.historyViewport()

	lines := historyLines(machine, width-4)
	offset := clampInt(m.historyOffset, 0, historyMaxOffset(len(lines), vp))
	end := offset + vp
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("History: "+nonempty(machine.Name, string(machine.ID))) + "\n")
	b.WriteString(strings.Join(lines[offset:end], "\n"))
	b.WriteString("\n" + metaStyle.Render("j/k pgup/pgdn scroll | esc close"))
	return b.String()
}

func historyMaxOffset(total, viewport int) int {
	if total <= viewport {
		return 0
	}
	return total - viewport
}

func (m Model) handleHistoryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		m.mode = modeCockpit
		m.historyOffset = 0
		return m, nil
	}
	machine := m.visible[m.cursor]
	max := historyMaxOffset(len(historyLines(machine, 0)), m.historyViewport())
	switch msg.String() {
	case "esc":
		m.mode = modeCockpit
		m.historyOffset = 0
	case "j", "down":
		m.historyOffset = clampInt(m.historyOffset+1, 0, max)
	case "k", "up":
		m.historyOffset = clampInt(m.historyOffset-1, 0, max)
	case "pgdown", "pgdn":
		m.historyOffset = clampInt(m.historyOffset+m.historyViewport(), 0, max)
	case "pgup":
		m.historyOffset = clampInt(m.historyOffset-m.historyViewport(), 0, max)
	}
	return m, nil
}
