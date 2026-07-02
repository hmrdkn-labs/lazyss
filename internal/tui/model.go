// Package tui implements the Bubble Tea cockpit model.
package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
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
	search        string
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
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// View renders the current cockpit state.
func (m Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m Model) render() string {
	var b strings.Builder
	b.WriteString("Lazy Secure Shell\n")
	if m.runtime != nil && (m.runtime.AWSProfile != "" || m.runtime.AWSRegion != "") {
		fmt.Fprintf(&b, "aws profile: %s", awsProfileLabel(m.runtime.AWSProfile))
		if m.runtime.AWSRegion != "" {
			fmt.Fprintf(&b, " region: %s", m.runtime.AWSRegion)
		}
		b.WriteByte('\n')
	}
	if m.statusLine != "" {
		b.WriteString(m.statusLine + "\n")
	}
	if len(m.statuses) > 0 {
		for _, status := range m.statuses {
			if status.Status == domain.ProviderDegraded {
				b.WriteString(m.providerWarning(status) + "\n")
			}
		}
	}
	if m.inputMode == "profile" {
		b.WriteString("AWS profiles\n")
		if len(m.profiles) == 0 {
			b.WriteString("No profiles\n")
		}
		for i, profile := range m.profiles {
			cursor := " "
			if i == m.profileCursor {
				cursor = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", cursor, profile)
		}
		return b.String()
	}
	if m.inputMode != "" {
		fmt.Fprintf(&b, "%s: %s\n", m.inputMode, m.search)
	}
	if len(m.visible) == 0 {
		b.WriteString("No machines\n")
		return b.String()
	}
	for i, machine := range m.visible {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		pin := " "
		if machine.Pinned {
			pin = "*"
		}
		method := machine.DefaultMethod()
		fmt.Fprintf(&b, "%s%s %-4s %-20s %-24s %-14s %-24s %-20s\n",
			cursor, pin, machine.Provider, machine.Name, machine.Address, method, machine.Health.Label, rel(machine.LastConnectedAt))
	}
	if m.details && len(m.visible) > 0 {
		machine := m.visible[m.cursor]
		b.WriteString("\nDetails\n")
		fmt.Fprintf(&b, "ID: %s\nProvider: %s\nNative: %s\nNote: %s\nConnections: %d\n",
			machine.ID, machine.Provider, machine.NativeID, machine.Note, machine.ConnectionCount)
		b.WriteString("Health history:\n")
		for _, obs := range lastHealth(machine.HealthHistory, 5) {
			fmt.Fprintf(&b, "  %s %s %s\n", obs.CheckedAt.Format("2006-01-02 15:04"), obs.Method, obs.Label)
		}
		b.WriteString("Session history:\n")
		for _, event := range lastSessions(machine.SessionHistory, 5) {
			outcome := "fail"
			if event.Success {
				outcome = "ok"
			}
			fmt.Fprintf(&b, "  %s %s %s\n", event.EndedAt.Format("2006-01-02 15:04"), event.Method, outcome)
		}
	}
	return b.String()
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
		m.inputMode = ""
		if m.search != "" {
			m.applySearch("")
		}
	case "enter":
		m.inputMode = ""
	case "backspace":
		if len(m.search) > 0 {
			m.applySearch(m.search[:len(m.search)-1])
		}
	default:
		key := msg.String()
		if len([]rune(key)) == 1 {
			m.applySearch(m.search + key)
		}
	}
	return m, nil
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
	return func() tea.Msg {
		_, err := m.runtime.Connect.Connect(context.Background(), machine, method, app.ConnectOptions{})
		if err != nil {
			return statusMsg(err.Error())
		}
		return statusMsg("session ended")
	}
}

type healthMsg domain.HealthObservation
type statusMsg string

func rel(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format("2006-01-02 15:04")
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
