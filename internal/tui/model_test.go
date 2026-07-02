package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
	"github.com/hamardikan/lazyss/internal/ports"
)

func TestModelSearchClampAndStaleRefresh(t *testing.T) {
	m := NewModel(nil)
	m.machines = []domain.Machine{{ID: "1", Name: "alpha"}, {ID: "2", Name: "beta"}}
	m.cursor = 1
	m.applySearch("zzz")
	if m.cursor != 0 || len(m.visible) != 0 {
		t.Fatalf("search clamp failed: cursor=%d visible=%d", m.cursor, len(m.visible))
	}
	m.refreshSeq = 2
	m.handleMachinesMsg(machinesMsg{seq: 1, machines: []domain.Machine{{ID: "old"}}})
	if len(m.machines) != 2 {
		t.Fatalf("stale refresh changed machines")
	}
}

func TestModelCycleMethodAndApplyHealth(t *testing.T) {
	m := NewModel(nil)
	m.machines = []domain.Machine{{ID: "1", Name: "alpha", Methods: []domain.AccessMethod{domain.AccessSSH, domain.AccessAWSSSMShell}}}
	m.recompute()
	m.cycleMethod()
	if got := m.machines[0].SelectedMethod; got != domain.AccessAWSSSMShell {
		t.Fatalf("selected method = %s", got)
	}
	m.applyHealth(domain.NewHealthObservation("1", domain.AccessAWSSSMShell, domain.HealthUp, "ssm Online ec2 running", 0, time.Now()))
	if m.machines[0].Health.Label == "" {
		t.Fatalf("health not applied")
	}
}

func TestModelDetailHistoryAndCopyCommand(t *testing.T) {
	copied := ""
	runtime := &Runtime{
		Connect: &app.ConnectService{Connectors: []ports.Connector{copyConnector{}}},
		Copy: func(s string) error {
			copied = s
			return nil
		},
	}
	m := NewModel(runtime)
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	m.machines = []domain.Machine{{
		ID:       "ssh:1:prod",
		Name:     "prod",
		NativeID: "prod",
		Methods:  []domain.AccessMethod{domain.AccessSSH},
		HealthHistory: []domain.HealthObservation{
			domain.NewHealthObservation("ssh:1:prod", domain.AccessSSH, domain.HealthUp, "tcp prod:22", time.Millisecond, now),
		},
		SessionHistory: []domain.SessionEvent{{MachineID: "ssh:1:prod", Method: domain.AccessSSH, EndedAt: now, Success: true}},
	}}
	m.recompute()
	m.details = true
	if got := m.render(); !strings.Contains(got, "Details") || !strings.Contains(got, "tcp prod:22") {
		t.Fatalf("detail render missing history: %s", got)
	}
	msg := m.copySelectedCmd()()
	if copied != "ssh prod" || string(msg.(statusMsg)) != "copied: ssh prod" {
		t.Fatalf("copy result copied=%q msg=%v", copied, msg)
	}
}

func TestModelRendersBoundedProviderDegradedWarnings(t *testing.T) {
	m := NewModel(&Runtime{AWSProfile: "default"})
	raw := "operation error SSM: DescribeInstanceInformation, https response error StatusCode: 400, RequestID: 6d6fec41-b934-4298-82fe-a479f2250bd5, api error UnrecognizedClientException: The security token included in the request is invalid"
	m.statuses = []domain.ProviderStatus{{Name: "aws", Status: domain.ProviderDegraded, Message: raw}}
	got := m.render()
	if strings.Contains(got, "RequestID") {
		t.Fatalf("render leaked raw AWS request details: %s", got)
	}
	if !strings.Contains(got, "source aws default degraded: auth failed; P profile / L login") {
		t.Fatalf("render missing profile-aware auth hint: %s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "source aws") && len(line) > maxProviderWarningRunes {
			t.Fatalf("provider warning too long: %d %q", len(line), line)
		}
	}
}

func TestModelRenderShowsControlsAndAWSOnboarding(t *testing.T) {
	m := NewModel(&Runtime{Query: app.InventoryQuery{Source: "all"}})
	m.machines = []domain.Machine{{
		ID:       "ssh:1:prod",
		Name:     "prod",
		Provider: domain.ProviderSSH,
		Address:  "prod.example",
		Methods:  []domain.AccessMethod{domain.AccessSSH},
	}}
	m.recompute()

	got := m.render()
	for _, want := range []string{
		"AWS: no profile selected",
		"P profile",
		"L login",
		"s source",
		"f filter",
		"Enter connect",
		"g check",
		"/ search",
		"q quit",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("render missing %q:\n%s", want, got)
		}
	}
}

func TestModelFilterModeShowsAvailableFilters(t *testing.T) {
	m := NewModel(&Runtime{Query: app.InventoryQuery{Source: "all"}})
	model, _ := m.Update(keyPress("f"))
	m = model.(Model)

	got := m.render()
	for _, want := range []string{
		"filter:",
		"Available filters",
		"tag:Key=Value",
		"name:prefix",
		"method:ssh|ssm",
		"health:up|down|unknown",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("filter view missing %q:\n%s", want, got)
		}
	}
}

func TestParseFilterExpression(t *testing.T) {
	filter, err := parseFilterExpression("tag:Use=maps name:procal method:ssm health:up api")
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}
	if filter.Tags["Use"] != "maps" {
		t.Fatalf("tags = %#v", filter.Tags)
	}
	if filter.NamePrefix != "procal" || filter.Method != string(domain.AccessAWSSSMShell) || filter.Health != "up" || filter.Text != "api" {
		t.Fatalf("filter = %#v", filter)
	}
}

func TestModelAppliesStructuredFilterAndRefreshes(t *testing.T) {
	provider := &queryCapturingProvider{machines: []domain.Machine{
		{
			ID:           "aws:ssm:123:ap-southeast-3:i-1",
			Name:         "procal-g1-dev-maps",
			Provider:     domain.ProviderAWS,
			Methods:      []domain.AccessMethod{domain.AccessAWSSSMShell},
			ProviderTags: map[string]string{"Use": "maps"},
			Health:       domain.NewHealthObservation("aws:ssm:123:ap-southeast-3:i-1", domain.AccessAWSSSMShell, domain.HealthUp, "ssm Online", 0, time.Now()),
		},
		{
			ID:           "aws:ssm:123:ap-southeast-3:i-2",
			Name:         "procal-g1-dev-api",
			Provider:     domain.ProviderAWS,
			Methods:      []domain.AccessMethod{domain.AccessAWSSSMShell},
			ProviderTags: map[string]string{"Use": "api"},
			Health:       domain.NewHealthObservation("aws:ssm:123:ap-southeast-3:i-2", domain.AccessAWSSSMShell, domain.HealthUp, "ssm Online", 0, time.Now()),
		},
	}}
	runtime := &Runtime{
		Inventory: &app.InventoryService{Providers: []app.InventoryProvider{provider}},
		Query:     app.InventoryQuery{Source: "aws"},
	}
	m := NewModel(runtime)
	m.filterText = "tag:Use=maps name:procal method:ssm health:up"
	model, cmd := m.submitFilter()
	if cmd == nil {
		t.Fatalf("expected refresh command")
	}
	m = model.(Model)
	if m.inputMode != "" {
		t.Fatalf("filter mode should close, got %q", m.inputMode)
	}
	model, _ = m.Update(cmd())
	m = model.(Model)
	if provider.query.Tags["Use"] != "maps" || provider.query.NamePrefix != "procal" {
		t.Fatalf("query = %#v", provider.query)
	}
	if len(m.visible) != 1 || m.visible[0].Name != "procal-g1-dev-maps" {
		t.Fatalf("visible = %#v", m.visible)
	}
}

func TestModelCyclesSourceAndRefreshes(t *testing.T) {
	provider := &queryCapturingProvider{}
	runtime := &Runtime{
		Inventory: &app.InventoryService{Providers: []app.InventoryProvider{provider}},
		Query:     app.InventoryQuery{Source: "all"},
	}
	m := NewModel(runtime)

	model, cmd := m.Update(keyPress("s"))
	m = model.(Model)
	if runtime.Query.Source != "ssh" || cmd == nil {
		t.Fatalf("source=%q cmd nil=%v", runtime.Query.Source, cmd == nil)
	}
	if !strings.Contains(m.render(), "source ssh") {
		t.Fatalf("render missing ssh source: %s", m.render())
	}

	model, cmd = m.Update(keyPress("s"))
	m = model.(Model)
	if runtime.Query.Source != "aws" || cmd == nil {
		t.Fatalf("source=%q cmd nil=%v", runtime.Query.Source, cmd == nil)
	}
	if !strings.Contains(m.render(), "source ssm") {
		t.Fatalf("render missing ssm source: %s", m.render())
	}
}

func TestModelRenderUsesCockpitPanels(t *testing.T) {
	m := NewModel(&Runtime{Query: app.InventoryQuery{Source: "all"}, AWSProfile: "hmrdkn-dev1", AWSRegion: "ap-southeast-3"})
	m.width = 132
	m.height = 34
	now := time.Date(2026, 7, 2, 9, 30, 0, 0, time.UTC)
	m.machines = []domain.Machine{
		{
			ID:              "ssh:1:prod",
			Name:            "prod",
			Provider:        domain.ProviderSSH,
			Address:         "prod.example",
			NativeID:        "prod",
			Methods:         []domain.AccessMethod{domain.AccessSSH},
			Health:          domain.NewHealthObservation("ssh:1:prod", domain.AccessSSH, domain.HealthUp, "tcp prod.example:22", 10*time.Millisecond, now),
			LastConnectedAt: now,
		},
		{
			ID:           "aws:ssm:123:ap-southeast-3:i-1",
			Name:         "api",
			Provider:     domain.ProviderAWS,
			NativeID:     "i-1",
			Scope:        domain.Scope{Account: "123", Region: "ap-southeast-3", Profile: "hmrdkn-dev1"},
			Methods:      []domain.AccessMethod{domain.AccessAWSSSMShell},
			ProviderTags: map[string]string{"Use": "maps"},
			Health:       domain.NewHealthObservation("aws:ssm:123:ap-southeast-3:i-1", domain.AccessAWSSSMShell, domain.HealthUp, "ssm Online ec2 running", 0, now),
		},
	}
	m.cursor = 1
	m.recompute()

	got := m.render()
	for _, want := range []string{
		"Lazy Secure Shell",
		"AWS: hmrdkn-dev1 profile region ap-southeast-3",
		"Machines (2)",
		"Provider",
		"Name",
		"Method",
		"Health",
		"Details",
		"Native",
		"Last connected",
		"Recent health",
		"Tags",
		"Use=maps",
		"prod",
		"api",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cockpit render missing %q:\n%s", want, got)
		}
	}
}

func TestModelTracksWindowSize(t *testing.T) {
	model, cmd := NewModel(nil).Update(tea.WindowSizeMsg{Width: 101, Height: 29})
	if cmd != nil {
		t.Fatalf("window resize should not return command")
	}
	m := model.(Model)
	if m.width != 101 || m.height != 29 {
		t.Fatalf("size = %dx%d, want 101x29", m.width, m.height)
	}
}

func TestModelViewUsesAltScreen(t *testing.T) {
	view := NewModel(nil).View()
	if !view.AltScreen {
		t.Fatal("expected TUI to use alternate screen")
	}
}

func TestModelProfilePickerShowsControls(t *testing.T) {
	m := NewModel(&Runtime{Query: app.InventoryQuery{Source: "all"}})
	m.handleProfilesMsg(profilesMsg{profiles: []string{"default", "hmrdkn-dev1"}})

	got := m.render()
	for _, want := range []string{
		"AWS profiles",
		"> default",
		"Enter select",
		"esc cancel",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("profile picker missing %q:\n%s", want, got)
		}
	}
}

func TestModelSelectsAWSProfilePersistsAndRefreshes(t *testing.T) {
	prefs := &fakePreferences{}
	selected := ""
	runtime := &Runtime{
		AWSProfiles: fakeProfileProvider{profiles: []string{"default", "hmrdkn-dev1"}},
		Preferences: prefs,
		SetAWSProfile: func(_ context.Context, profile string) (*app.InventoryService, error) {
			selected = profile
			return &app.InventoryService{Providers: []app.InventoryProvider{profileInventoryProvider{profile: profile}}}, nil
		},
	}
	m := NewModel(runtime)
	model, cmd := m.Update(keyPress("P"))
	if cmd == nil {
		t.Fatalf("expected profile list command")
	}
	model, cmd = model.Update(cmd())
	m = model.(Model)
	if m.inputMode != "profile" || len(m.profiles) != 2 {
		t.Fatalf("profile mode not opened: mode=%q profiles=%#v", m.inputMode, m.profiles)
	}
	model, _ = m.Update(keyPress("down"))
	m = model.(Model)
	model, cmd = m.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatalf("expected profile select command")
	}
	model, cmd = model.Update(cmd())
	m = model.(Model)
	if selected != "hmrdkn-dev1" || runtime.AWSProfile != "hmrdkn-dev1" {
		t.Fatalf("selected=%q runtime profile=%q", selected, runtime.AWSProfile)
	}
	if prefs.saved.AWSProfile != "hmrdkn-dev1" {
		t.Fatalf("saved preferences = %#v", prefs.saved)
	}
	if cmd == nil {
		t.Fatalf("expected inventory refresh command")
	}
	model, _ = m.Update(cmd())
	m = model.(Model)
	if len(m.machines) != 1 || m.machines[0].Scope.Profile != "hmrdkn-dev1" {
		t.Fatalf("machines = %#v", m.machines)
	}
}

func TestModelRunsAWSLoginForSelectedProfileAndRefreshes(t *testing.T) {
	login := &fakeLogin{}
	runtime := &Runtime{
		AWSProfile: "hmrdkn-dev1",
		AWSLogin:   login,
		Inventory:  &app.InventoryService{Providers: []app.InventoryProvider{profileInventoryProvider{profile: "hmrdkn-dev1"}}},
	}
	m := NewModel(runtime)
	model, cmd := m.Update(keyPress("L"))
	if cmd == nil {
		t.Fatalf("expected login command")
	}
	if got := reflect.TypeOf(cmd()).String(); !strings.Contains(got, "execMsg") {
		t.Fatalf("expected Bubble Tea exec message, got %s", got)
	}
	if err := (loginExecCommand{runner: login, profile: "hmrdkn-dev1"}).Run(); err != nil {
		t.Fatalf("login exec: %v", err)
	}
	model, cmd = model.Update(awsLoginMsg{})
	m = model.(Model)
	if login.profile != "hmrdkn-dev1" {
		t.Fatalf("login profile = %q", login.profile)
	}
	if !strings.Contains(m.statusLine, "aws login complete") {
		t.Fatalf("status line = %q", m.statusLine)
	}
	if cmd == nil {
		t.Fatalf("expected refresh after login")
	}
	model, _ = m.Update(cmd())
	m = model.(Model)
	if len(m.machines) != 1 || m.machines[0].Provider != domain.ProviderAWS {
		t.Fatalf("machines = %#v", m.machines)
	}
}

func TestModelConnectUsesTerminalExecHandoff(t *testing.T) {
	runtime := &Runtime{
		Connect: &app.ConnectService{Connectors: []ports.Connector{copyConnector{}}},
	}
	m := NewModel(runtime)
	m.machines = []domain.Machine{{
		ID:       "ssh:1:prod",
		Name:     "prod",
		NativeID: "prod",
		Methods:  []domain.AccessMethod{domain.AccessSSH},
	}}
	m.recompute()

	_, cmd := m.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatalf("expected connect command")
	}
	if got := reflect.TypeOf(cmd()).String(); !strings.Contains(got, "execMsg") {
		t.Fatalf("expected Bubble Tea exec handoff, got %s", got)
	}
}

type copyConnector struct{}

func (copyConnector) Supports(domain.Machine, domain.AccessMethod) bool { return true }
func (copyConnector) BuildCommand(domain.Machine, domain.AccessMethod, app.ConnectOptions) (ports.CommandSpec, error) {
	return ports.CommandSpec{Executable: "ssh", Args: []string{"prod"}}, nil
}
func (copyConnector) RunInteractive(context.Context, ports.CommandSpec) (app.SessionResult, error) {
	return app.SessionResult{}, nil
}

func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "down":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})
	case "up":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})
	}
	runes := []rune(s)
	return tea.KeyPressMsg(tea.Key{Text: s, Code: runes[0]})
}

type fakeProfileProvider struct {
	profiles []string
	err      error
}

func (f fakeProfileProvider) ListProfiles(context.Context) ([]string, error) {
	return append([]string(nil), f.profiles...), f.err
}

type fakeLogin struct {
	profile string
	err     error
}

func (f *fakeLogin) Login(_ context.Context, profile string) error {
	f.profile = profile
	return f.err
}

type fakePreferences struct {
	saved domain.OperatorPreferences
	err   error
}

func (f *fakePreferences) LoadPreferences(context.Context) (domain.OperatorPreferences, error) {
	return f.saved, f.err
}

func (f *fakePreferences) SavePreferences(_ context.Context, prefs domain.OperatorPreferences) error {
	f.saved = prefs
	return f.err
}

type profileInventoryProvider struct {
	profile string
}

func (p profileInventoryProvider) ProviderName() string { return "aws" }

func (p profileInventoryProvider) ListMachines(context.Context, app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	return []domain.Machine{{
		ID:       domain.MachineID("aws:ssm:123:ap-southeast-1:i-1"),
		Name:     "ssm-node",
		Provider: domain.ProviderAWS,
		Scope:    domain.Scope{Profile: p.profile},
		Methods:  []domain.AccessMethod{domain.AccessAWSSSMShell},
	}}, domain.ProviderStatus{Name: "aws", Status: domain.ProviderHealthy}, nil
}

type queryCapturingProvider struct {
	query    app.InventoryQuery
	machines []domain.Machine
}

func (p *queryCapturingProvider) ProviderName() string { return "aws" }

func (p *queryCapturingProvider) ListMachines(_ context.Context, q app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	p.query = q
	return append([]domain.Machine(nil), p.machines...), domain.ProviderStatus{Name: "aws", Status: domain.ProviderHealthy}, nil
}
