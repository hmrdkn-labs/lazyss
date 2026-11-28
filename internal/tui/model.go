// Package tui implements the Bubble Tea cockpit model.
package tui

import (
	"context"
	"fmt"
	"io"
	"os/exec"
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
	Cleanup       *app.CleanupService
	Query         app.InventoryQuery
	Copy          func(string) error
	Preferences   ports.PreferenceStore
	AWSProfiles   ports.AWSProfileProvider
	AWSLogin      ports.AWSLoginRunner
	AWSProfile    string
	AWSRegion     string
	SetAWSProfile func(ctx context.Context, profile string) (*app.InventoryService, error)

	// GCP and Azure scope selection and login. Scope listers and login specs
	// come from the CLI adapters through ports so the TUI never imports them.
	GCPProject string
	// ResolveGCPProject looks up the default project in the background when
	// GCPProject is empty; the cockpit never waits for it.
	ResolveGCPProject    func(ctx context.Context) (string, error)
	AzureSubscription    string
	GCPScopes            ports.ScopeLister
	AzureScopes          ports.ScopeLister
	GCPLogin             ports.CommandSpec
	AzureLogin           ports.CommandSpec
	SetGCPProject        func(ctx context.Context, project string) (*app.InventoryService, error)
	SetAzureSubscription func(ctx context.Context, subscription string) (*app.InventoryService, error)
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
	profiles      []string // picker labels
	pickerValues  []string // picker values parallel to profiles; equals profiles for AWS
	pickerKind    domain.ProviderKind
	profileCursor int
	refreshSeq    int
	statusLine    string
	historyOffset int
	editor        editorState
	cleanup       cleanupState
	inflight      map[domain.MachineID]struct{} // machines with a health check in flight, rendered with a checking glyph
	checkTotal    int                           // machines in the current streamed check batch
	checkDone     int                           // observations received so far in the current batch
}

type machinesMsg struct {
	seq      int
	machines []domain.Machine
	statuses []domain.ProviderStatus
	err      error
	// ch is the inventory stream still delivering snapshots; nil when the
	// message is final.
	ch <-chan app.InventoryResult
	// closed marks the end of the stream: it carries no inventory.
	closed bool
}

type profilesMsg struct {
	provider domain.ProviderKind // zero means AWS
	profiles []string
	values   []string // optional ids parallel to profiles
	err      error
}

type profileSelectedMsg struct {
	provider  domain.ProviderKind // zero means AWS
	profile   string
	inventory *app.InventoryService
	err       error
}

type awsLoginMsg struct {
	err error
}

type cloudLoginMsg struct {
	provider domain.ProviderKind
	err      error
}

type connectFinishedMsg struct {
	err error
}

type healthMsg domain.HealthObservation
type statusMsg string

// healthStreamMsg carries one observation off the G-triggered stream. ok is
// false when the channel closes, signalling the batch is complete.
type healthStreamMsg struct {
	obs domain.HealthObservation
	ch  <-chan domain.HealthObservation
	ok  bool
}

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
	return tea.Batch(m.fetchCmd(m.refreshSeq), m.resolveProjectCmd())
}

type gcpProjectMsg struct{ project string }

func (m Model) resolveProjectCmd() tea.Cmd {
	if m.runtime.GCPProject != "" || m.runtime.ResolveGCPProject == nil {
		return nil
	}
	resolve := m.runtime.ResolveGCPProject
	return func() tea.Msg {
		project, err := resolve(context.Background())
		if err != nil || project == "" {
			return nil
		}
		return gcpProjectMsg{project: project}
	}
}

// Update applies one Bubble Tea message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case gcpProjectMsg:
		if m.runtime.GCPProject == "" {
			m.runtime.GCPProject = msg.project
		}
	case machinesMsg:
		cmd := m.handleMachinesMsg(msg)
		return m, cmd
	case healthMsg:
		m.applyHealth(domain.HealthObservation(msg))
	case healthStreamMsg:
		return m.handleHealthStream(msg)
	case statusMsg:
		m.statusLine = string(msg)
	case profilesMsg:
		m.handleProfilesMsg(msg)
	case profileSelectedMsg:
		return m.handleProfileSelectedMsg(msg)
	case awsLoginMsg:
		return m.handleAWSLoginMsg(msg)
	case cloudLoginMsg:
		return m.handleCloudLoginMsg(msg)
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
	ch := m.runtime.Inventory.Stream(context.Background(), m.runtime.Query)
	return waitInventory(seq, ch)
}

// waitInventory blocks for the next inventory snapshot. Each snapshot carries
// the stream so the model can keep reading until the last provider finishes.
func waitInventory(seq int, ch <-chan app.InventoryResult) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return machinesMsg{seq: seq, closed: true}
		}
		return machinesMsg{seq: seq, machines: r.Machines, statuses: r.Statuses, ch: ch}
	}
}

func (m *Model) handleMachinesMsg(msg machinesMsg) tea.Cmd {
	if msg.seq != 0 && msg.seq < m.refreshSeq {
		return nil
	}
	if msg.closed {
		return nil
	}
	m.machines = append([]domain.Machine(nil), msg.machines...)
	m.statuses = append([]domain.ProviderStatus(nil), msg.statuses...)
	if msg.err != nil {
		m.statusLine = msg.err.Error()
	}
	m.recompute()
	if msg.ch != nil {
		return waitInventory(msg.seq, msg.ch)
	}
	return nil
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

func (m Model) handleCloudLoginMsg(msg cloudLoginMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusLine = string(msg.provider) + " login: " + msg.err.Error()
		return m, nil
	}
	m.statusLine = string(msg.provider) + " login complete"
	m.refreshSeq++
	return m, m.fetchCmd(m.refreshSeq)
}

// scopeProvider picks the provider P and L act on: the source in view, or for
// source all the selected machine's provider, falling back to AWS.
func (m Model) scopeProvider() domain.ProviderKind {
	if m.runtime == nil {
		return domain.ProviderAWS
	}
	switch m.runtime.Query.Source {
	case "gcp":
		return domain.ProviderGCP
	case "azure":
		return domain.ProviderAzure
	case "", "all":
		if len(m.visible) > 0 {
			if p := m.visible[m.cursor].Provider; p == domain.ProviderGCP || p == domain.ProviderAzure {
				return p
			}
		}
	}
	return domain.ProviderAWS
}

// loginCmd runs the interactive login for the provider P/L currently target.
func (m Model) loginCmd() tea.Cmd {
	provider := m.scopeProvider()
	var spec ports.CommandSpec
	switch provider {
	case domain.ProviderGCP:
		spec = m.runtime.GCPLogin
	case domain.ProviderAzure:
		spec = m.runtime.AzureLogin
	default:
		return m.awsLoginCmd()
	}
	if spec.Executable == "" {
		return func() tea.Msg {
			return cloudLoginMsg{provider: provider, err: fmt.Errorf("login is not configured")}
		}
	}
	return tea.ExecProcess(exec.Command(spec.Executable, spec.Args...), func(err error) tea.Msg {
		return cloudLoginMsg{provider: provider, err: err}
	})
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
	delete(m.inflight, obs.MachineID)
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

// checkSelected marks the selected machine in flight and checks it; the
// resulting healthMsg clears the marker via applyHealth.
func (m Model) checkSelected() (tea.Model, tea.Cmd) {
	if m.runtime == nil || m.runtime.Health == nil || len(m.visible) == 0 {
		return m, nil
	}
	machine := m.visible[m.cursor]
	method := machine.DefaultMethod()
	m.markInflight(machine.ID)
	return m, func() tea.Msg {
		return healthMsg(m.runtime.Health.CheckSelected(context.Background(), machine, method))
	}
}

// checkVisible streams a bounded health check over every visible machine,
// marking each in flight up front so rows show the checking glyph until their
// observation arrives. The app layer owns the concurrency bound.
func (m Model) checkVisible() (tea.Model, tea.Cmd) {
	if m.runtime == nil || m.runtime.Health == nil || len(m.visible) == 0 {
		return m, nil
	}
	machines := append([]domain.Machine(nil), m.visible...)
	m.inflight = make(map[domain.MachineID]struct{}, len(machines))
	for _, machine := range machines {
		m.inflight[machine.ID] = struct{}{}
	}
	m.checkTotal = len(machines)
	m.checkDone = 0
	m.statusLine = fmt.Sprintf("checking %d/%d", 0, m.checkTotal)
	ch := make(chan domain.HealthObservation)
	go m.runtime.Health.CheckVisibleStream(context.Background(), machines, ch)
	return m, waitHealth(ch)
}

func (m *Model) markInflight(id domain.MachineID) {
	if m.inflight == nil {
		m.inflight = make(map[domain.MachineID]struct{}, 1)
	}
	m.inflight[id] = struct{}{}
}

// waitHealth reads the next streamed observation; each received message pumps
// the next read so the UI stays responsive between observations.
func waitHealth(ch <-chan domain.HealthObservation) tea.Cmd {
	return func() tea.Msg {
		obs, ok := <-ch
		return healthStreamMsg{obs: obs, ch: ch, ok: ok}
	}
}

func (m Model) handleHealthStream(msg healthStreamMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		m.inflight = nil
		m.checkTotal, m.checkDone = 0, 0
		m.statusLine = ""
		return m, nil
	}
	m.applyHealth(msg.obs)
	m.checkDone++
	m.statusLine = fmt.Sprintf("checking %d/%d", m.checkDone, m.checkTotal)
	return m, waitHealth(msg.ch)
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
