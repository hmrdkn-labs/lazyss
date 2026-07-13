package tui

import tea "charm.land/bubbletea/v2"

// mode is the cockpit's top-level input state. Esc always returns to modeCockpit.
type mode int

const (
	modeCockpit mode = iota
	modeInput
	modeProfilePicker
	modeHelp
)

func (m Model) render() string {
	switch m.mode {
	case modeProfilePicker:
		return m.renderProfilePicker()
	case modeHelp:
		return m.renderHelp()
	default:
		return m.renderCockpit()
	}
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeProfilePicker:
		return m.handleProfileKey(msg)
	case modeInput:
		return m.handleInputKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	default:
		return m.handleCockpitKey(msg)
	}
}
