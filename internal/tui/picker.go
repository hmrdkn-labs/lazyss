package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func (m Model) renderProfilePicker() string {
	var b strings.Builder
	b.WriteString(m.titleBar() + "\n")
	if line := m.awsLine(); line != "" {
		b.WriteString(line + "\n")
	}
	var content strings.Builder
	content.WriteString(panelTitleStyle.Render("AWS profiles") + "\n")
	if len(m.profiles) == 0 {
		content.WriteString(dimStyle.Render("No profiles") + "\n")
	}
	for i, profile := range m.profiles {
		cursor := " "
		if i == m.profileCursor {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %s", cursor, profile)
		if i == m.profileCursor {
			line = selectedStyle.Render(displayPadRight(line, 32))
		}
		content.WriteString(line + "\n")
	}
	b.WriteString(panelActiveStyle.Width(42).Render(content.String()))
	b.WriteString("\nKeys: j/k move | Enter select | esc cancel | q quit\n")
	return b.String()
}

func (m Model) handleProfileKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeCockpit
	case "j", "down":
		if m.profileCursor < len(m.profiles)-1 {
			m.profileCursor++
		}
	case "k", "up":
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case "enter":
		if len(m.profiles) == 0 {
			m.mode = modeCockpit
			return m, nil
		}
		profile := m.profiles[m.profileCursor]
		m.mode = modeCockpit
		return m, m.selectProfileCmd(profile)
	}
	return m, nil
}

func (m *Model) handleProfilesMsg(msg profilesMsg) {
	if msg.err != nil {
		m.statusLine = "aws profiles: " + msg.err.Error()
		return
	}
	m.profiles = append([]string(nil), msg.profiles...)
	m.profileCursor = 0
	m.mode = modeProfilePicker
	if len(m.profiles) == 0 {
		m.statusLine = "aws profiles: none found"
	}
}

func (m Model) handleProfileSelectedMsg(msg profileSelectedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusLine = "aws profile: " + msg.err.Error()
		return m, nil
	}
	if m.runtime != nil {
		m.runtime.AWSProfile = msg.profile
		if msg.inventory != nil {
			m.runtime.Inventory = msg.inventory
		}
		if m.runtime.Preferences != nil {
			_ = m.runtime.Preferences.SavePreferences(context.Background(), domain.OperatorPreferences{
				AWSProfile: msg.profile,
				AWSRegion:  m.runtime.AWSRegion,
			})
		}
	}
	m.statusLine = "aws profile: " + msg.profile
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

func (m Model) listProfilesCmd() tea.Cmd {
	if m.runtime == nil || m.runtime.AWSProfiles == nil {
		return func() tea.Msg {
			return profilesMsg{err: fmt.Errorf("aws profile discovery is not configured")}
		}
	}
	return func() tea.Msg {
		profiles, err := m.runtime.AWSProfiles.ListProfiles(context.Background())
		return profilesMsg{profiles: profiles, err: err}
	}
}

func (m Model) selectProfileCmd(profile string) tea.Cmd {
	if m.runtime == nil || m.runtime.SetAWSProfile == nil {
		return func() tea.Msg {
			return profileSelectedMsg{profile: profile, err: fmt.Errorf("aws profile switching is not configured")}
		}
	}
	return func() tea.Msg {
		inventory, err := m.runtime.SetAWSProfile(context.Background(), profile)
		return profileSelectedMsg{profile: profile, inventory: inventory, err: err}
	}
}
