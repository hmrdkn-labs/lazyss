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

	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
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
	mode          mode
	inputKind     string
	details       bool
	profiles      []string
	profileCursor int
	refreshSeq    int
	statusLine    string
	historyOffset int
	editor        editorState
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

type healthMsg domain.HealthObservation
type statusMsg string

// NewModel creates a TUI model with the detail panel shown and an initial filtered view.
func NewModel(runtime *Runtime) Model {
	m := Model{runtime: runtime, details: true}
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
	if !m.showHidden() {
		filtered := m.visible[:0]
		for _, machine := range m.visible {
			if !machine.Hidden {
				filtered = append(filtered, machine)
			}
		}
		m.visible = filtered
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

func (m *Model) toggleHidden() {
	if len(m.visible) == 0 {
		return
	}
	selected := m.visible[m.cursor]
	selected.Hidden = !selected.Hidden
	m.replaceMachine(selected)
	m.saveOverlay(selected)
	if selected.Hidden {
		m.statusLine = "hidden " + nonempty(selected.Name, string(selected.ID))
	} else {
		m.statusLine = "unhidden " + nonempty(selected.Name, string(selected.ID))
	}
	m.recompute()
}

func (m Model) toggleShowHidden() (tea.Model, tea.Cmd) {
	if m.runtime == nil {
		return m, nil
	}
	m.runtime.Query.ShowHidden = !m.runtime.Query.ShowHidden
	if m.runtime.Query.ShowHidden {
		m.statusLine = "show hidden"
	} else {
		m.statusLine = "hide hidden"
	}
	m.recompute()
	return m, nil
}

func (m Model) showHidden() bool {
	return m.runtime != nil && m.runtime.Query.ShowHidden
}

func (m Model) saveOverlay(machine domain.Machine) {
	if m.runtime == nil || m.runtime.Inventory == nil || m.runtime.Inventory.Store == nil {
		return
	}
	overlay := domain.MachineOverlay{
		MachineID:       machine.ID,
		Pinned:          machine.Pinned,
		Hidden:          machine.Hidden,
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
