package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

// Cleanup plan sections in display and navigation order. Only delete
// candidates are selectable; the others exist so the operator sees the full
// picture the CLI dry-run reports before deciding what to remove.
const (
	cleanupSecHide = iota
	cleanupSecPortForward
	cleanupSecDelete
	cleanupSecProtected
)

var cleanupSectionTitles = [...]string{
	cleanupSecHide:        "Hide recommendations",
	cleanupSecPortForward: "Port-forward recommendations",
	cleanupSecDelete:      "Delete candidates",
	cleanupSecProtected:   "Protected (kept, not selectable)",
}

// cleanupState holds the transient dry-run plan the cleanup view navigates.
// rows are item indices ordered by section, so the flat cursor lines up with
// the rendered rows. selected keys are host aliases in the allow-list.
type cleanupState struct {
	items    []ports.CleanupItem
	rows     []int
	cursor   int
	selected map[string]bool
	confirm  bool
}

func newCleanupState(plan ports.CleanupPlan) cleanupState {
	st := cleanupState{items: plan.Items, selected: map[string]bool{}}
	for kind := range cleanupSectionTitles {
		for i, it := range plan.Items {
			if cleanupKind(it) == kind {
				st.rows = append(st.rows, i)
			}
		}
	}
	return st
}

// cleanupKind maps a plan item to its section, or -1 for plain kept machines
// that carry no recommendation and would only clutter the cleanup view.
func cleanupKind(it ports.CleanupItem) int {
	switch {
	case it.Action == ports.CleanupDeleteCandidate:
		return cleanupSecDelete
	case it.Action == ports.CleanupHide && isPortForwardReason(it.Reason):
		return cleanupSecPortForward
	case it.Action == ports.CleanupHide:
		return cleanupSecHide
	case it.Protected:
		return cleanupSecProtected
	default:
		return -1
	}
}

func isPortForwardReason(reason string) bool {
	return strings.Contains(strings.ToLower(reason), "port forward")
}

// openCleanup loads the dry-run plan. A nil service disables the key with a
// status hint rather than opening an empty view.
func (m Model) openCleanup() (tea.Model, tea.Cmd) {
	if m.runtime == nil || m.runtime.Cleanup == nil {
		m.statusLine = "cleanup unavailable"
		return m, nil
	}
	plan, err := m.runtime.Cleanup.Plan(ports.CleanupOptions{})
	if err != nil {
		m.statusLine = "cleanup failed: " + err.Error()
		return m, nil
	}
	m.cleanup = newCleanupState(plan)
	m.mode = modeCleanup
	return m, nil
}

func (m Model) handleCleanupKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.cleanup.confirm {
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.cleanup.confirm = false
		case "y":
			return m.applyCleanup()
		}
		return m, nil
	}
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeCockpit
		m.cleanup = cleanupState{}
	case "j", "down":
		if m.cleanup.cursor < len(m.cleanup.rows)-1 {
			m.cleanup.cursor++
		}
	case "k", "up":
		if m.cleanup.cursor > 0 {
			m.cleanup.cursor--
		}
	case " ", "space":
		m.toggleCleanupSelection()
	case "w":
		if len(m.selectedCleanupHosts()) > 0 {
			m.cleanup.confirm = true
		}
	}
	return m, nil
}

func (m *Model) toggleCleanupSelection() {
	if m.cleanup.cursor < 0 || m.cleanup.cursor >= len(m.cleanup.rows) {
		return
	}
	it := m.cleanup.items[m.cleanup.rows[m.cleanup.cursor]]
	if cleanupKind(it) != cleanupSecDelete || it.Protected {
		return
	}
	if m.cleanup.selected[it.Host] {
		delete(m.cleanup.selected, it.Host)
	} else {
		m.cleanup.selected[it.Host] = true
	}
}

// selectedCleanupHosts returns the chosen allow-list, guarding against stale
// selections that are no longer selectable delete candidates.
func (m Model) selectedCleanupHosts() []string {
	var hosts []string
	for _, it := range m.cleanup.items {
		if cleanupKind(it) == cleanupSecDelete && !it.Protected && m.cleanup.selected[it.Host] {
			hosts = append(hosts, it.Host)
		}
	}
	sort.Strings(hosts)
	return hosts
}

func (m Model) applyCleanup() (tea.Model, tea.Cmd) {
	hosts := m.selectedCleanupHosts()
	res, err := m.runtime.Cleanup.Apply(ports.CleanupApplyOptions{Hosts: hosts})
	m.mode = modeCockpit
	m.cleanup = cleanupState{}
	if err != nil {
		m.statusLine = "cleanup failed: " + err.Error()
		return m, nil
	}
	m.statusLine = fmt.Sprintf("cleanup wrote backup %s; removed %s", res.BackupPath, strings.Join(res.RemovedHosts, ", "))
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

func (m Model) renderCleanup() string {
	if m.cleanup.confirm {
		return m.renderCleanupConfirm()
	}
	width, _ := m.layoutSize()
	var b strings.Builder
	b.WriteString(m.titleBar() + "\n")
	b.WriteString(panelTitleStyle.Render("SSH cleanup - dry run") + "\n")
	if len(m.cleanup.rows) == 0 {
		b.WriteString(dimStyle.Render("No cleanup recommendations.") + "\n")
	}
	row := 0
	for kind := range cleanupSectionTitles {
		first := true
		for _, idx := range m.cleanup.rows {
			it := m.cleanup.items[idx]
			if cleanupKind(it) != kind {
				continue
			}
			if first {
				b.WriteString("\n" + metaStyle.Render(cleanupSectionTitles[kind]) + "\n")
				first = false
			}
			b.WriteString(m.cleanupRow(it, kind, row == m.cleanup.cursor, width) + "\n")
			row++
		}
	}
	b.WriteString("\n" + dimStyle.Render("j/k move | space select delete candidate | w write | esc back") + "\n")
	return b.String()
}

func (m Model) cleanupRow(it ports.CleanupItem, kind int, active bool, width int) string {
	selectable := kind == cleanupSecDelete && !it.Protected
	box := "   "
	if selectable {
		mark := " "
		if m.cleanup.selected[it.Host] {
			mark = "x"
		}
		box = "[" + mark + "]"
	}
	text := fmt.Sprintf("%s %-22s %-18s %s", box, displayFit(it.Host, 22), displayFit(it.Reason, 18), cleanupItemTarget(it))
	text = displayPadRight(text, clampInt(width-1, 1, width))
	switch {
	case active:
		return selectedStyle.Render(text)
	case it.Protected:
		return faintStyle.Render(text)
	default:
		return text
	}
}

func (m Model) renderCleanupConfirm() string {
	hosts := m.selectedCleanupHosts()
	var c strings.Builder
	c.WriteString(panelTitleStyle.Render("Confirm cleanup write") + "\n\n")
	c.WriteString("These hosts will be removed from your SSH config:\n")
	for _, h := range hosts {
		c.WriteString("  - " + h + "\n")
	}
	c.WriteString("\n" + dimStyle.Render("A backup (config.lazyss-backup-<timestamp>) is written before any change.") + "\n")
	c.WriteString("\ny apply | esc cancel")

	var b strings.Builder
	b.WriteString(m.titleBar() + "\n")
	b.WriteString(panelActiveStyle.Width(60).Render(c.String()))
	b.WriteString("\n")
	return b.String()
}

func cleanupItemTarget(it ports.CleanupItem) string {
	if it.HostName == "" {
		return "-"
	}
	return fmt.Sprintf("%s@%s:%d", nonempty(it.User, "-"), it.HostName, it.Port)
}
