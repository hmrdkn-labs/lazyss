package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

// profilePickerRows caps how many profiles the fixed picker box shows at once;
// longer lists scroll around the cursor.
const profilePickerRows = 10

func (m Model) renderProfilePicker() string {
	var b strings.Builder
	b.WriteString(m.titleBar() + "\n")
	if line := m.awsLine(); line != "" {
		b.WriteString(line + "\n")
	}
	var content strings.Builder
	title, empty := pickerLabels(m.pickerKind)
	content.WriteString(panelTitleStyle.Render(title) + "\n")
	if len(m.profiles) == 0 {
		content.WriteString(dimStyle.Render(empty) + "\n")
	}
	start, end := windowRange(m.profileCursor, len(m.profiles), profilePickerRows)
	for i := start; i < end; i++ {
		cursor := " "
		if i == m.profileCursor {
			cursor = ">"
		}
		line := fmt.Sprintf("%s %s", cursor, m.profiles[i])
		if i == m.profileCursor {
			line = selectedStyle.Render(displayPadRight(line, 46))
		}
		content.WriteString(line + "\n")
	}
	if len(m.profiles) > profilePickerRows {
		content.WriteString(dimStyle.Render(fmt.Sprintf("%d-%d of %d", start+1, end, len(m.profiles))) + "\n")
	}
	b.WriteString(panelActiveStyle.Width(56).Render(content.String()))
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
		value := m.pickerValues[m.profileCursor]
		m.mode = modeCockpit
		return m, m.selectProfileCmd(m.pickerKind, value)
	}
	return m, nil
}

func pickerLabels(kind domain.ProviderKind) (title, empty string) {
	switch kind {
	case domain.ProviderGCP:
		return "GCP projects", "No projects"
	case domain.ProviderAzure:
		return "Azure subscriptions", "No subscriptions"
	default:
		return "AWS profiles", "No profiles"
	}
}

func scopeNoun(kind domain.ProviderKind) string {
	switch kind {
	case domain.ProviderGCP:
		return "gcp project"
	case domain.ProviderAzure:
		return "azure subscription"
	default:
		return "aws profile"
	}
}

func (m *Model) handleProfilesMsg(msg profilesMsg) {
	kind := msg.provider
	if kind == "" {
		kind = domain.ProviderAWS
	}
	noun := scopeNoun(kind)
	if msg.err != nil {
		m.statusLine = noun + "s: " + msg.err.Error()
		return
	}
	m.pickerKind = kind
	m.profiles = append([]string(nil), msg.profiles...)
	m.pickerValues = append([]string(nil), msg.values...)
	if len(m.pickerValues) != len(m.profiles) {
		m.pickerValues = append([]string(nil), msg.profiles...)
	}
	m.profileCursor = 0
	m.mode = modeProfilePicker
	if len(m.profiles) == 0 {
		m.statusLine = noun + "s: none found"
	}
}

func (m Model) handleProfileSelectedMsg(msg profileSelectedMsg) (tea.Model, tea.Cmd) {
	noun := scopeNoun(msg.provider)
	if msg.err != nil {
		m.statusLine = noun + ": " + msg.err.Error()
		return m, nil
	}
	if m.runtime != nil {
		switch msg.provider {
		case domain.ProviderGCP:
			m.runtime.GCPProject = msg.profile
		case domain.ProviderAzure:
			m.runtime.AzureSubscription = msg.profile
		default:
			m.runtime.AWSProfile = msg.profile
		}
		if msg.inventory != nil {
			m.runtime.Inventory = msg.inventory
		}
		if m.runtime.Preferences != nil {
			_ = m.runtime.Preferences.SavePreferences(context.Background(), domain.OperatorPreferences{
				AWSProfile:        m.runtime.AWSProfile,
				AWSRegion:         m.runtime.AWSRegion,
				GCPProject:        m.runtime.GCPProject,
				AzureSubscription: m.runtime.AzureSubscription,
			})
		}
	}
	m.statusLine = noun + ": " + msg.profile
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

// listScopesCmd lists the AWS profiles, GCP projects or Azure subscriptions of
// the provider P currently targets.
func (m Model) listScopesCmd() tea.Cmd {
	kind := m.scopeProvider()
	var lister ports.ScopeLister
	if m.runtime != nil {
		switch kind {
		case domain.ProviderGCP:
			lister = m.runtime.GCPScopes
		case domain.ProviderAzure:
			lister = m.runtime.AzureScopes
		}
	}
	if kind == domain.ProviderAWS {
		return m.listProfilesCmd()
	}
	if lister == nil {
		return func() tea.Msg {
			return profilesMsg{provider: kind, err: fmt.Errorf("discovery is not configured")}
		}
	}
	return func() tea.Msg {
		scopes, err := lister.ListScopes(context.Background())
		msg := profilesMsg{provider: kind, err: err}
		for _, s := range scopes {
			label := s.ID
			if s.Name != "" && s.Name != s.ID {
				label = s.Name + " (" + s.ID + ")"
			}
			msg.profiles = append(msg.profiles, label)
			msg.values = append(msg.values, s.ID)
		}
		return msg
	}
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

func (m Model) selectProfileCmd(kind domain.ProviderKind, value string) tea.Cmd {
	var set func(context.Context, string) (*app.InventoryService, error)
	if m.runtime != nil {
		switch kind {
		case domain.ProviderGCP:
			set = m.runtime.SetGCPProject
		case domain.ProviderAzure:
			set = m.runtime.SetAzureSubscription
		default:
			set = m.runtime.SetAWSProfile
		}
	}
	if set == nil {
		return func() tea.Msg {
			return profileSelectedMsg{provider: kind, profile: value, err: fmt.Errorf("switching is not configured")}
		}
	}
	return func() tea.Msg {
		inventory, err := set(context.Background(), value)
		return profileSelectedMsg{provider: kind, profile: value, inventory: inventory, err: err}
	}
}
