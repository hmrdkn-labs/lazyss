package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		kind := m.inputKind
		m.mode = modeCockpit
		if kind == "search" && m.search != "" {
			m.applySearch("")
		}
	case "enter":
		if m.inputKind == "filter" {
			return m.submitFilter()
		}
		m.mode = modeCockpit
	case "backspace":
		if m.inputKind == "filter" && len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		} else if len(m.search) > 0 {
			m.applySearch(m.search[:len(m.search)-1])
		}
	default:
		key := msg.String()
		if len([]rune(key)) == 1 {
			if m.inputKind == "filter" {
				m.filterText += key
			} else {
				m.applySearch(m.search + key)
			}
		}
	}
	return m, nil
}

func (m Model) submitFilter() (tea.Model, tea.Cmd) {
	filter, err := parseFilterExpression(m.filterText)
	if err != nil {
		m.statusLine = "filter: " + err.Error()
		return m, nil
	}
	m.mode = modeCockpit
	m.filter = filter
	if m.runtime != nil {
		m.runtime.Query.Tags = nil
		m.runtime.Query.NamePrefix = ""
		if filter.Hidden != "" {
			m.runtime.Query.ShowHidden = filter.Hidden == "true"
		}
	}
	m.statusLine = "filter: " + nonempty(filter.Raw, "cleared")
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

func (m Model) inputValue() string {
	if m.inputKind == "filter" {
		return m.filterText
	}
	return m.search
}

func (m Model) availableFiltersLine() string {
	return "Available filters: tag:Key=Value | name:prefix | method:ssh|ssm | health:up|down|unknown | hidden:true|false | text"
}

func (m *Model) applySearch(query string) {
	m.search = query
	m.recompute()
}

func (m *Model) clearSearchAndFilter() {
	if m.search == "" && m.filterText == "" && m.filter.empty() {
		return
	}
	m.search = ""
	m.filterText = ""
	m.filter = cockpitFilter{}
	if m.runtime != nil {
		m.runtime.Query.Tags = nil
		m.runtime.Query.NamePrefix = ""
	}
	m.statusLine = "filter cleared"
	m.recompute()
}

func (m Model) hasActiveFilter() bool {
	return m.search != "" || !m.filter.empty()
}

func (m Model) activeFilterLabel() string {
	var parts []string
	if m.filter.Raw != "" {
		parts = append(parts, m.filter.Raw)
	}
	if m.search != "" {
		parts = append(parts, "search:"+m.search)
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}
