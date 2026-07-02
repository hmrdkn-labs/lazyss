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
