package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// keyEntry is one row of the cockpit key table, the single source for cockpit
// key dispatch, the footer, and the help overlay. Key v remains unregistered
// so a later step can claim it without conflict.
type keyEntry struct {
	keys   []string // keys tea reports that trigger this action
	label  string   // key as shown to the operator (e.g. "j/k", "Enter")
	desc   string   // footer wording
	help   string   // help-overlay wording
	action func(m Model, key string) (tea.Model, tea.Cmd)
}

var cockpitKeys = []keyEntry{
	{keys: []string{"j", "k", "down", "up"}, label: "j/k", desc: "move", help: "move selection", action: func(m Model, key string) (tea.Model, tea.Cmd) {
		switch key {
		case "j", "down":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		}
		return m, nil
	}},
	{keys: []string{"enter"}, label: "Enter", desc: "connect", help: "connect to selected machine", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m, m.connectSelectedCmd()
	}},
	{keys: []string{"s"}, label: "s", desc: "source", help: "cycle source all/ssh/ssm", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m.cycleSource()
	}},
	{keys: []string{"m"}, label: "m", desc: "method", help: "cycle access method", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.cycleMethod()
		return m, nil
	}},
	{keys: []string{"/"}, label: "/", desc: "search", help: "fuzzy search machines", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.mode = modeInput
		m.inputKind = "search"
		return m, nil
	}},
	{keys: []string{"f"}, label: "f", desc: "filter", help: "structured filter", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.mode = modeInput
		m.inputKind = "filter"
		return m, nil
	}},
	{keys: []string{"p"}, label: "p", desc: "pin", help: "toggle pin", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.togglePin()
		return m, nil
	}},
	{keys: []string{"x"}, label: "x", desc: "hide", help: "hide selected machine", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.toggleHidden()
		return m, nil
	}},
	{keys: []string{"u"}, label: "u", desc: "show hidden", help: "toggle showing hidden", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m.toggleShowHidden()
	}},
	{keys: []string{"h"}, label: "h", desc: "detail", help: "toggle detail panel", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.details = !m.details
		return m, nil
	}},
	{keys: []string{"e"}, label: "e", desc: "edit", help: "edit note, tags, preferred method", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m.openEditor()
	}},
	{keys: []string{"c"}, label: "c", desc: "copy", help: "copy connect command", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m, m.copySelectedCmd()
	}},
	{keys: []string{"g"}, label: "g", desc: "check", help: "health check selected", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m.checkSelected()
	}},
	{keys: []string{"G"}, label: "G", desc: "check all", help: "health check visible", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m.checkVisible()
	}},
	{keys: []string{"v"}, label: "v", desc: "history", help: "view full session/health history", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		if len(m.visible) == 0 {
			return m, nil
		}
		m.mode = modeHistory
		m.historyOffset = 0
		return m, nil
	}},
	{keys: []string{"P"}, label: "P", desc: "profile", help: "choose AWS profile", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m, m.listProfilesCmd()
	}},
	{keys: []string{"L"}, label: "L", desc: "login", help: "AWS SSO login", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m, m.awsLoginCmd()
	}},
	{keys: []string{"r"}, label: "r", desc: "refresh", help: "reload inventory", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.refreshSeq++
		return m, m.fetchCmd(m.refreshSeq)
	}},
	{keys: []string{"C"}, label: "C", desc: "cleanup", help: "ssh config cleanup", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m.openCleanup()
	}},
	{keys: []string{"?"}, label: "?", desc: "help", help: "toggle this help", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		m.mode = modeHelp
		return m, nil
	}},
	{keys: []string{"q", "ctrl+c"}, label: "q", desc: "quit", help: "quit lazyss", action: func(m Model, _ string) (tea.Model, tea.Cmd) {
		return m, tea.Quit
	}},
}

// handleCockpitKey routes a keypress through the registry. Esc is universal: it
// clears any active search/filter and stays in the cockpit.
func (m Model) handleCockpitKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" {
		m.clearSearchAndFilter()
		return m, nil
	}
	for _, e := range cockpitKeys {
		for _, k := range e.keys {
			if k == key {
				return e.action(m, key)
			}
		}
	}
	return m, nil
}

func (m Model) footer() string {
	width, _ := m.layoutSize()
	tokens := make([]string, 0, len(cockpitKeys))
	for _, e := range cockpitKeys {
		tokens = append(tokens, e.label+" "+e.desc)
	}
	return packTokens(tokens, " | ", width)
}

func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(m.titleBar() + "\n")
	var content strings.Builder
	content.WriteString(panelTitleStyle.Render("Keys") + "\n")
	for _, e := range cockpitKeys {
		content.WriteString(faintStyle.Render(displayPadRight(e.label, 8)) + e.help + "\n")
	}
	b.WriteString(panelActiveStyle.Width(48).Render(content.String()))
	b.WriteString("\nesc close | q quit")
	return b.String()
}

func (m Model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key := msg.String(); key == "q" || key == "ctrl+c" {
		return m, tea.Quit
	}
	m.mode = modeCockpit
	return m, nil
}

// packTokens greedily fills lines up to width with whole tokens joined by sep,
// wrapping to a new line rather than clipping a token mid-word.
func packTokens(tokens []string, sep string, width int) string {
	if width <= 0 {
		return strings.Join(tokens, sep)
	}
	var b strings.Builder
	line := ""
	for _, tok := range tokens {
		switch {
		case line == "":
			line = tok
		case lipglossWidth(line+sep+tok) <= width:
			line += sep + tok
		default:
			b.WriteString(line + "\n")
			line = tok
		}
	}
	b.WriteString(line)
	return b.String()
}
