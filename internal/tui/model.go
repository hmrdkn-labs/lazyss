// Package tui implements the Bubble Tea cockpit model.
package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
	"github.com/hamardikan/lazyss/internal/ports"
)

const maxProviderWarningRunes = 78

// Runtime wires the TUI to app-layer services and clipboard support.
type Runtime struct {
	Inventory     *app.InventoryService
	Connect       *app.ConnectService
	Health        *app.HealthService
	Query         app.InventoryQuery
	Copy          func(string) error
	Preferences   ports.PreferenceStore
	AWSProfiles   ports.AWSProfileProvider
	AWSLogin      ports.AWSLoginRunner
	AWSProfile    string
	AWSRegion     string
	SetAWSProfile func(ctx context.Context, profile string) (*app.InventoryService, error)
}

// Model is the Bubble Tea state for the machine cockpit.
type Model struct {
	runtime       *Runtime
	machines      []domain.Machine
	visible       []domain.Machine
	statuses      []domain.ProviderStatus
	cursor        int
	width         int
	height        int
	search        string
	filterText    string
	filter        cockpitFilter
	inputMode     string
	details       bool
	profiles      []string
	profileCursor int
	refreshSeq    int
	statusLine    string
}

type machinesMsg struct {
	seq      int
	machines []domain.Machine
	statuses []domain.ProviderStatus
	err      error
}

type profilesMsg struct {
	profiles []string
	err      error
}

type profileSelectedMsg struct {
	profile   string
	inventory *app.InventoryService
	err       error
}

type awsLoginMsg struct {
	err error
}

type connectFinishedMsg struct {
	err error
}

// NewModel creates a TUI model with an initial filtered view.
func NewModel(runtime *Runtime) Model {
	m := Model{runtime: runtime}
	m.recompute()
	return m
}

// Init starts initial inventory loading when services are available.
func (m Model) Init() tea.Cmd {
	if m.runtime == nil || m.runtime.Inventory == nil {
		return nil
	}
	m.refreshSeq++
	return m.fetchCmd(m.refreshSeq)
}

// Update applies one Bubble Tea message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case machinesMsg:
		m.handleMachinesMsg(msg)
	case healthMsg:
		m.applyHealth(domain.HealthObservation(msg))
	case statusMsg:
		m.statusLine = string(msg)
	case profilesMsg:
		m.handleProfilesMsg(msg)
	case profileSelectedMsg:
		return m.handleProfileSelectedMsg(msg)
	case awsLoginMsg:
		return m.handleAWSLoginMsg(msg)
	case connectFinishedMsg:
		return m.handleConnectFinishedMsg(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// View renders the current cockpit state.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.inputMode == "profile" {
		return m.renderProfilePicker()
	}
	return m.renderCockpit()
}

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
	if width >= 92 {
		detailWidth := clampInt(width*38/100, 34, 54)
		listWidth := width - detailWidth
		body = lipglossJoinHorizontal(
			panelActiveStyle.Width(listWidth-2).Height(bodyHeight).Render(m.machineList(listWidth-4, bodyHeight)),
			panelStyle.Width(detailWidth-2).Height(bodyHeight).Render(m.detailPanel(detailWidth-4, bodyHeight)),
		)
	} else {
		body = m.compactList(width, bodyHeight)
	}

	var b strings.Builder
	b.WriteString(title + "\n")
	if meta != "" {
		b.WriteString(meta + "\n")
	}
	if m.inputMode != "" {
		fmt.Fprintf(&b, "%s: %s\n", m.inputMode, m.inputValue())
		if m.inputMode == "filter" {
			b.WriteString(m.availableFiltersLine() + "\n")
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

func (m Model) titleBar() string {
	width, _ := m.layoutSize()
	source := m.sourceLabel()
	count := fmt.Sprintf("%d machines", len(m.visible))
	left := titleStyle.Render("Lazy Secure Shell")
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

func (m Model) footer() string {
	return "Keys: j/k move | s source all/ssh/ssm | f filter | / search | Enter connect | g check | G check visible | P profile | L login | r refresh | h details | q quit\n"
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

func lipglossJoinHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}

func (m Model) machineList(width, height int) string {
	header := panelTitleStyle.Render(fmt.Sprintf("Machines (%d)", len(m.visible)))
	columns := faintStyle.Render(m.listHeader(width))
	lines := []string{header, columns}
	if len(m.visible) == 0 {
		lines = append(lines, dimStyle.Render("No machines"))
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

func (m Model) detailPanel(width, height int) string {
	lines := []string{panelTitleStyle.Render("Details")}
	if len(m.visible) == 0 {
		lines = append(lines, dimStyle.Render("No selection"))
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

func (m Model) compactList(width, height int) string {
	lines := []string{panelTitleStyle.Render(fmt.Sprintf("Machines (%d)", len(m.visible)))}
	if len(m.visible) == 0 {
		lines = append(lines, dimStyle.Render("No machines"))
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

func (m Model) inputValue() string {
	if m.inputMode == "filter" {
		return m.filterText
	}
	return m.search
}

func (m Model) availableFiltersLine() string {
	return "Available filters: tag:Key=Value | name:prefix | method:ssh|ssm | health:up|down|unknown | text"
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

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.inputMode == "profile" {
		return m.handleProfileKey(msg)
	}
	if m.inputMode != "" {
		return m.handleInputKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.refreshSeq++
		return m, m.fetchCmd(m.refreshSeq)
	case "s":
		return m.cycleSource()
	case "j", "down":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "m":
		m.cycleMethod()
	case "p":
		m.togglePin()
	case "/":
		m.inputMode = "search"
	case "f":
		m.inputMode = "filter"
	case "h":
		m.details = !m.details
	case "c":
		return m, m.copySelectedCmd()
	case "g":
		return m, m.checkSelectedCmd()
	case "G":
		return m, m.checkVisibleCmd()
	case "P":
		return m, m.listProfilesCmd()
	case "L":
		return m, m.awsLoginCmd()
	case "enter":
		return m, m.connectSelectedCmd()
	}
	return m, nil
}

func (m Model) handleProfileKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.inputMode = ""
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
			m.inputMode = ""
			return m, nil
		}
		profile := m.profiles[m.profileCursor]
		m.inputMode = ""
		return m, m.selectProfileCmd(profile)
	}
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		mode := m.inputMode
		m.inputMode = ""
		if mode == "search" && m.search != "" {
			m.applySearch("")
		}
	case "enter":
		if m.inputMode == "filter" {
			return m.submitFilter()
		}
		m.inputMode = ""
	case "backspace":
		if m.inputMode == "filter" && len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		} else if len(m.search) > 0 {
			m.applySearch(m.search[:len(m.search)-1])
		}
	default:
		key := msg.String()
		if len([]rune(key)) == 1 {
			if m.inputMode == "filter" {
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
	m.inputMode = ""
	m.filter = filter
	if m.runtime != nil {
		m.runtime.Query.Tags = filter.queryTags()
		m.runtime.Query.NamePrefix = filter.NamePrefix
	}
	m.statusLine = "filter: " + nonempty(filter.Raw, "cleared")
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
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

func (m Model) fetchCmd(seq int) tea.Cmd {
	if m.runtime == nil || m.runtime.Inventory == nil {
		return func() tea.Msg {
			return machinesMsg{seq: seq, err: fmt.Errorf("inventory is not configured")}
		}
	}
	return func() tea.Msg {
		result, err := m.runtime.Inventory.List(context.Background(), m.runtime.Query)
		return machinesMsg{seq: seq, machines: result.Machines, statuses: result.Statuses, err: err}
	}
}

func (m *Model) handleMachinesMsg(msg machinesMsg) {
	if msg.seq != 0 && msg.seq < m.refreshSeq {
		return
	}
	m.machines = append([]domain.Machine(nil), msg.machines...)
	m.statuses = append([]domain.ProviderStatus(nil), msg.statuses...)
	if msg.err != nil {
		m.statusLine = msg.err.Error()
	}
	m.recompute()
}

func (m *Model) handleProfilesMsg(msg profilesMsg) {
	if msg.err != nil {
		m.statusLine = "aws profiles: " + msg.err.Error()
		return
	}
	m.profiles = append([]string(nil), msg.profiles...)
	m.profileCursor = 0
	m.inputMode = "profile"
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

func (m Model) handleAWSLoginMsg(msg awsLoginMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusLine = "aws login: " + msg.err.Error()
		return m, nil
	}
	m.statusLine = "aws login complete"
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

func (m Model) handleConnectFinishedMsg(msg connectFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusLine = msg.err.Error()
	} else {
		m.statusLine = "session ended"
	}
	if m.runtime == nil || m.runtime.Inventory == nil {
		return m, nil
	}
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

func (m *Model) applySearch(query string) {
	m.search = query
	m.recompute()
}

func (m *Model) recompute() {
	if m.search == "" {
		m.visible = append([]domain.Machine(nil), m.machines...)
	} else {
		names := make([]string, len(m.machines))
		for i, machine := range m.machines {
			names[i] = machine.Name + " " + machine.Address + " " + machine.NativeID
		}
		matches := fuzzy.Find(m.search, names)
		m.visible = m.visible[:0]
		for _, match := range matches {
			m.visible = append(m.visible, m.machines[match.Index])
		}
	}
	if !m.filter.empty() {
		filtered := m.visible[:0]
		for _, machine := range m.visible {
			if m.filter.matches(machine) {
				filtered = append(filtered, machine)
			}
		}
		m.visible = filtered
	}
	if len(m.visible) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
}

func (m *Model) cycleMethod() {
	if len(m.visible) == 0 {
		return
	}
	selected := m.visible[m.cursor]
	if len(selected.Methods) == 0 {
		return
	}
	idx := 0
	for i, method := range selected.Methods {
		if method == selected.DefaultMethod() {
			idx = i
			break
		}
	}
	selected.SelectedMethod = selected.Methods[(idx+1)%len(selected.Methods)]
	m.replaceMachine(selected)
	m.saveOverlay(selected)
	m.recompute()
}

func (m *Model) applyHealth(obs domain.HealthObservation) {
	for i := range m.machines {
		if m.machines[i].ID == obs.MachineID {
			m.machines[i].Health = obs
			m.machines[i].LastCheckedAt = obs.CheckedAt
		}
	}
	m.recompute()
}

func (m *Model) togglePin() {
	if len(m.visible) == 0 {
		return
	}
	selected := m.visible[m.cursor]
	selected.Pinned = !selected.Pinned
	m.replaceMachine(selected)
	m.saveOverlay(selected)
	domain.SortMachines(m.machines)
	m.recompute()
}

func (m Model) saveOverlay(machine domain.Machine) {
	if m.runtime == nil || m.runtime.Inventory == nil || m.runtime.Inventory.Store == nil {
		return
	}
	overlay := domain.MachineOverlay{
		MachineID:       machine.ID,
		Pinned:          machine.Pinned,
		Tags:            machine.Tags,
		Note:            machine.Note,
		PreferredMethod: machine.SelectedMethod,
		LastCheckedAt:   machine.LastCheckedAt,
		LastConnectedAt: machine.LastConnectedAt,
		ConnectionCount: machine.ConnectionCount,
		LastHealth:      machine.Health,
		HealthHistory:   machine.HealthHistory,
		SessionHistory:  machine.SessionHistory,
	}
	_ = m.runtime.Inventory.Store.SaveOverlay(context.Background(), overlay)
}

func (m *Model) replaceMachine(machine domain.Machine) {
	for i := range m.machines {
		if m.machines[i].ID == machine.ID {
			m.machines[i] = machine
			return
		}
	}
}

func (m Model) checkSelectedCmd() tea.Cmd {
	if m.runtime == nil || m.runtime.Health == nil || len(m.visible) == 0 {
		return nil
	}
	machine := m.visible[m.cursor]
	method := machine.DefaultMethod()
	return func() tea.Msg {
		obs := m.runtime.Health.CheckSelected(context.Background(), machine, method)
		return healthMsg(obs)
	}
}

func (m Model) checkVisibleCmd() tea.Cmd {
	if m.runtime == nil || m.runtime.Health == nil || len(m.visible) == 0 {
		return nil
	}
	machines := append([]domain.Machine(nil), m.visible...)
	return func() tea.Msg {
		for _, machine := range machines {
			_ = m.runtime.Health.CheckSelected(context.Background(), machine, machine.DefaultMethod())
		}
		result, _ := m.runtime.Inventory.List(context.Background(), m.runtime.Query)
		return machinesMsg{seq: m.refreshSeq, machines: result.Machines, statuses: result.Statuses}
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

func (m Model) awsLoginCmd() tea.Cmd {
	if m.runtime == nil || m.runtime.AWSLogin == nil {
		return func() tea.Msg {
			return awsLoginMsg{err: fmt.Errorf("aws login is not configured")}
		}
	}
	profile := ""
	if m.runtime != nil {
		profile = strings.TrimSpace(m.runtime.AWSProfile)
	}
	if profile == "" {
		return func() tea.Msg {
			return awsLoginMsg{err: fmt.Errorf("choose an AWS profile with P first")}
		}
	}
	return tea.Exec(loginExecCommand{runner: m.runtime.AWSLogin, profile: profile}, func(err error) tea.Msg {
		return awsLoginMsg{err: err}
	})
}

type loginExecCommand struct {
	runner  ports.AWSLoginRunner
	profile string
}

func (c loginExecCommand) Run() error {
	return c.runner.Login(context.Background(), c.profile)
}

func (loginExecCommand) SetStdin(io.Reader) {}

func (loginExecCommand) SetStdout(io.Writer) {}

func (loginExecCommand) SetStderr(io.Writer) {}

func (m Model) copySelectedCmd() tea.Cmd {
	if m.runtime == nil || m.runtime.Connect == nil || len(m.visible) == 0 {
		return nil
	}
	machine := m.visible[m.cursor]
	return func() tea.Msg {
		cmd, err := m.runtime.Connect.BuildCommand(contextMachine(machine), machine.DefaultMethod(), app.ConnectOptions{})
		if err != nil {
			return statusMsg(err.Error())
		}
		text := strings.Join(cmd.Argv(), " ")
		if m.runtime.Copy != nil {
			if err := m.runtime.Copy(text); err != nil {
				return statusMsg("copy failed: " + err.Error())
			}
		}
		return statusMsg("copied: " + text)
	}
}

func (m Model) connectSelectedCmd() tea.Cmd {
	if m.runtime == nil || m.runtime.Connect == nil || len(m.visible) == 0 {
		return nil
	}
	machine := m.visible[m.cursor]
	method := machine.DefaultMethod()
	return tea.Exec(connectExecCommand{connect: m.runtime.Connect, machine: machine, method: method}, func(err error) tea.Msg {
		return connectFinishedMsg{err: err}
	})
}

type connectExecCommand struct {
	connect *app.ConnectService
	machine domain.Machine
	method  domain.AccessMethod
}

func (c connectExecCommand) Run() error {
	_, err := c.connect.Connect(context.Background(), c.machine, c.method, app.ConnectOptions{})
	return err
}

func (connectExecCommand) SetStdin(io.Reader) {}

func (connectExecCommand) SetStdout(io.Writer) {}

func (connectExecCommand) SetStderr(io.Writer) {}

type healthMsg domain.HealthObservation
type statusMsg string

func rel(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format("2006-01-02 15:04")
}

func nonempty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func lastHealth(items []domain.HealthObservation, n int) []domain.HealthObservation {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func lastSessions(items []domain.SessionEvent, n int) []domain.SessionEvent {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func contextMachine(machine domain.Machine) domain.Machine {
	return machine
}
