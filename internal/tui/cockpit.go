package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hmrdkn-labs/lazyss/internal/brand"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func (m Model) renderCockpit() string {
	width, height := m.layoutSize()
	title := m.titleBar()
	meta := m.awsLine()
	warnings := m.providerWarnings()
	status := m.statusText()
	bodyHeight := height - 7 - len(warnings)
	if status != "" {
		bodyHeight--
	}
	if bodyHeight < 8 {
		bodyHeight = 8
	}

	var body string
	switch {
	case width >= 92 && m.details:
		detailWidth := clampInt(width*38/100, 34, 54)
		listWidth := width - detailWidth
		body = lipglossJoinHorizontal(
			panelActiveStyle.Width(listWidth-2).Height(bodyHeight).Render(m.machineList(listWidth-4, bodyHeight)),
			panelStyle.Width(detailWidth-2).Height(bodyHeight).Render(m.detailPanel(detailWidth-4, bodyHeight)),
		)
	case width >= 92:
		body = panelActiveStyle.Width(width - 2).Height(bodyHeight).Render(m.machineList(width-4, bodyHeight))
	default:
		body = m.compactList(width, bodyHeight)
	}

	var b strings.Builder
	b.WriteString(title + "\n")
	if meta != "" {
		b.WriteString(meta + "\n")
	}
	if m.mode == modeInput {
		fmt.Fprintf(&b, "%s: %s\n", m.inputKind, m.inputValue())
		if m.inputKind == "filter" {
			b.WriteString(m.availableFiltersLine() + "\n")
			if line := availableTagFiltersLine(m.machines, width); line != "" {
				b.WriteString(line + "\n")
			}
		}
	}
	if status != "" {
		b.WriteString(status + "\n")
	}
	for _, warning := range warnings {
		b.WriteString(warning + "\n")
	}
	b.WriteString(body)
	b.WriteString("\n" + m.footer())
	return b.String()
}

func (m Model) titleBar() string {
	width, _ := m.layoutSize()
	source := m.sourceLabel()
	count := fmt.Sprintf("%d machines", len(m.visible))
	left := titleStyle.Render(brand.Name)
	meta := metaStyle.Render(" " + source + " | " + count)
	line := left + meta
	if width > 0 && lipglossWidth(line) < width {
		return line
	}
	return line
}

func (m Model) awsLine() string {
	if !m.shouldShowAWSOnboarding() || m.runtime == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "AWS: %s", awsProfileSummary(m.runtime.AWSProfile))
	if m.runtime.AWSRegion != "" {
		fmt.Fprintf(&b, " region %s", m.runtime.AWSRegion)
	}
	if strings.TrimSpace(m.runtime.AWSProfile) == "" {
		b.WriteString(" - press P to choose, L to login")
	} else {
		b.WriteString(" - P change, L login")
	}
	return b.String()
}

func (m Model) providerWarnings() []string {
	var out []string
	for _, status := range m.statuses {
		if status.Status == domain.ProviderDegraded {
			out = append(out, m.providerWarning(status))
		}
	}
	return out
}

func (m Model) statusText() string {
	if m.statusLine == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(m.statusLine), "failed") || strings.Contains(strings.ToLower(m.statusLine), "error") {
		return badStyle.Render(m.statusLine)
	}
	return warnStyle.Render(m.statusLine)
}

func (m Model) shouldShowAWSOnboarding() bool {
	if m.runtime == nil {
		return false
	}
	source := m.runtime.Query.Source
	return source == "" || source == "all" || source == "aws" || m.runtime.AWSProfile != "" || m.runtime.AWSRegion != ""
}

func (m Model) layoutSize() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 32
	}
	return width, height
}

func (m Model) sourceLabel() string {
	if m.runtime == nil || m.runtime.Query.Source == "" {
		return "source all"
	}
	if m.runtime.Query.Source == "aws" {
		return "source ssm"
	}
	return "source " + m.runtime.Query.Source
}

func (m Model) cycleSource() (tea.Model, tea.Cmd) {
	if m.runtime == nil {
		return m, nil
	}
	switch m.runtime.Query.Source {
	case "", "all":
		m.runtime.Query.Source = "ssh"
	case "ssh":
		m.runtime.Query.Source = "aws"
	default:
		m.runtime.Query.Source = "all"
	}
	m.statusLine = m.sourceLabel()
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

func lipglossJoinHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}

func (m Model) providerWarning(status domain.ProviderStatus) string {
	message := compactProviderMessage(status.Message)
	if message == "" {
		message = "provider unavailable"
	}
	if status.Name == "aws" && isAWSAuthMessage(message) {
		label := ""
		if m.runtime != nil && m.runtime.AWSProfile != "" {
			label = " " + awsProfileLabel(m.runtime.AWSProfile)
		}
		return truncateRunes(fmt.Sprintf("source aws%s degraded: auth failed; P profile / L login", label), maxProviderWarningRunes)
	}
	line := fmt.Sprintf("source %s degraded: %s", status.Name, message)
	if len([]rune(line)) > maxProviderWarningRunes {
		if idx := strings.LastIndex(message, " ("); idx > 0 {
			line = fmt.Sprintf("source %s degraded: %s", status.Name, strings.TrimSpace(message[:idx]))
		}
	}
	return truncateRunes(line, maxProviderWarningRunes)
}

func awsProfileLabel(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return "default"
	}
	return profile
}

func awsProfileSummary(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return "no profile selected"
	}
	return profile + " profile"
}

func isAWSAuthMessage(message string) bool {
	message = strings.ToLower(message)
	if strings.Contains(message, "auth failed") {
		return true
	}
	for _, code := range []string{
		"expiredtoken",
		"expiredtokenexception",
		"invalidclienttokenid",
		"signaturedoesnotmatch",
		"unrecognizedclientexception",
	} {
		if strings.Contains(message, strings.ToLower(code)) {
			return true
		}
	}
	return false
}

func compactProviderMessage(message string) string {
	message = strings.TrimSpace(message)
	if idx := strings.LastIndex(message, "api error "); idx >= 0 {
		message = strings.TrimSpace(strings.TrimPrefix(message[idx:], "api error "))
	}
	if idx := strings.Index(message, "RequestID:"); idx >= 0 {
		message = strings.TrimSpace(message[:idx])
		message = strings.TrimRight(message, ",")
	}
	return message
}

func truncateRunes(s string, limit int) string {
	runes := []rune(strings.TrimSpace(s))
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
