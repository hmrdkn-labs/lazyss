package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

type fakeScopes struct{ options []ports.ScopeOption }

func (f fakeScopes) ListScopes(context.Context) ([]ports.ScopeOption, error) {
	return f.options, nil
}

func TestModelCyclesSourceThroughCloudProviders(t *testing.T) {
	runtime := &Runtime{Query: app.InventoryQuery{Source: "all"}}
	m := NewModel(runtime)
	var got []string
	for range 5 {
		model, _ := m.Update(keyPress("s"))
		m = model.(Model)
		got = append(got, runtime.Query.Source)
	}
	if want := []string{"ssh", "aws", "gcp", "azure", "all"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cycle = %v, want %v", got, want)
	}
	runtime.Query.Source = "azure"
	if !strings.Contains(m.render(), "source azure") {
		t.Fatalf("render missing azure source:\n%s", m.render())
	}
}

func TestModelScopePickerUsesProviderOfSourceOrSelection(t *testing.T) {
	gcp := fakeScopes{options: []ports.ScopeOption{{ID: "proj-a", Name: "Project A"}, {ID: "proj-b"}}}
	azure := fakeScopes{options: []ports.ScopeOption{{ID: "sub-1", Name: "Prod"}}}
	cases := []struct {
		name     string
		source   string
		machines []domain.Machine
		kind     domain.ProviderKind
		title    string
	}{
		{"gcp source", "gcp", nil, domain.ProviderGCP, "GCP projects"},
		{"azure source", "azure", nil, domain.ProviderAzure, "Azure subscriptions"},
		{"all selects gcp machine", "all", []domain.Machine{{ID: "g", Provider: domain.ProviderGCP}}, domain.ProviderGCP, "GCP projects"},
		{"all selects azure machine", "all", []domain.Machine{{ID: "z", Provider: domain.ProviderAzure}}, domain.ProviderAzure, "Azure subscriptions"},
		{"all ssh machine falls back to aws", "all", []domain.Machine{{ID: "s", Provider: domain.ProviderSSH}}, domain.ProviderAWS, "AWS profiles"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &Runtime{
				Query:       app.InventoryQuery{Source: tc.source},
				GCPScopes:   gcp,
				AzureScopes: azure,
				AWSProfiles: fakeProfileProvider{profiles: []string{"default"}},
			}
			m := NewModel(runtime)
			m.machines = tc.machines
			m.recompute()
			model, cmd := m.Update(keyPress("P"))
			model, _ = model.Update(cmd())
			m = model.(Model)
			if m.mode != modeProfilePicker || m.pickerKind != tc.kind {
				t.Fatalf("mode=%v kind=%q, want %q", m.mode, m.pickerKind, tc.kind)
			}
			if !strings.Contains(m.render(), tc.title) {
				t.Fatalf("picker missing %q:\n%s", tc.title, m.render())
			}
		})
	}
}

func TestModelSelectsGCPProjectAndAzureSubscriptionPersistsBoth(t *testing.T) {
	prefs := &fakePreferences{}
	var gcpSet, azSet string
	runtime := &Runtime{
		Query:       app.InventoryQuery{Source: "gcp"},
		Preferences: prefs,
		AWSProfile:  "dev",
		GCPScopes:   fakeScopes{options: []ports.ScopeOption{{ID: "proj-a", Name: "Project A"}, {ID: "proj-b", Name: "Project B"}}},
		AzureScopes: fakeScopes{options: []ports.ScopeOption{{ID: "sub-1", Name: "Prod"}}},
		SetGCPProject: func(_ context.Context, id string) (*app.InventoryService, error) {
			gcpSet = id
			return &app.InventoryService{}, nil
		},
		SetAzureSubscription: func(_ context.Context, id string) (*app.InventoryService, error) {
			azSet = id
			return &app.InventoryService{}, nil
		},
	}
	m := NewModel(runtime)
	model, cmd := m.Update(keyPress("P"))
	model, _ = model.Update(cmd())
	if !strings.Contains(model.(Model).render(), "Project B (proj-b)") {
		t.Fatalf("picker should show name and id:\n%s", model.(Model).render())
	}
	model, _ = model.Update(keyPress("down"))
	model, cmd = model.Update(keyPress("enter"))
	model, _ = model.Update(cmd())
	if gcpSet != "proj-b" || runtime.GCPProject != "proj-b" {
		t.Fatalf("gcpSet=%q runtime=%q", gcpSet, runtime.GCPProject)
	}

	runtime.Query.Source = "azure"
	model, cmd = model.Update(keyPress("P"))
	model, _ = model.Update(cmd())
	model, cmd = model.Update(keyPress("enter"))
	model, _ = model.Update(cmd())
	if azSet != "sub-1" || runtime.AzureSubscription != "sub-1" {
		t.Fatalf("azSet=%q runtime=%q", azSet, runtime.AzureSubscription)
	}
	want := domain.OperatorPreferences{AWSProfile: "dev", GCPProject: "proj-b", AzureSubscription: "sub-1"}
	if prefs.saved != want {
		t.Fatalf("saved = %#v, want %#v", prefs.saved, want)
	}
	if !strings.Contains(model.(Model).statusLine, "azure subscription: sub-1") {
		t.Fatalf("status = %q", model.(Model).statusLine)
	}
}

func TestModelLoginDispatchPerProvider(t *testing.T) {
	gcpSpec := ports.CommandSpec{Executable: "gcloud", Args: []string{"auth", "login"}}
	azSpec := ports.CommandSpec{Executable: "az", Args: []string{"login"}}
	login := &fakeLogin{}
	for _, tc := range []struct {
		source string
		exec   bool
	}{{"gcp", true}, {"azure", true}} {
		runtime := &Runtime{Query: app.InventoryQuery{Source: tc.source}, GCPLogin: gcpSpec, AzureLogin: azSpec, AWSLogin: login, AWSProfile: "dev"}
		m := NewModel(runtime)
		model, cmd := m.Update(keyPress("L"))
		if got := reflect.TypeOf(cmd()).String(); !strings.Contains(got, "execMsg") {
			t.Fatalf("%s: expected exec message, got %s", tc.source, got)
		}
		_ = model
	}
	if login.profile != "" {
		t.Fatalf("aws login must not run for cloud sources: %q", login.profile)
	}

	// source all follows the selected machine.
	runtime := &Runtime{Query: app.InventoryQuery{Source: "all"}, GCPLogin: gcpSpec, AzureLogin: azSpec, AWSLogin: login, AWSProfile: "dev"}
	m := NewModel(runtime)
	m.machines = []domain.Machine{{ID: "z", Provider: domain.ProviderAzure}}
	m.recompute()
	if m.scopeProvider() != domain.ProviderAzure {
		t.Fatalf("scope provider = %q", m.scopeProvider())
	}
	m.machines = nil
	m.recompute()
	_, cmd := m.Update(keyPress("L"))
	if _, ok := cmd().(awsLoginMsg); ok {
		t.Fatalf("expected tea exec for aws with profile")
	}

	// Unconfigured login reports instead of crashing.
	m = NewModel(&Runtime{Query: app.InventoryQuery{Source: "gcp"}})
	_, cmd = m.Update(keyPress("L"))
	msg, ok := cmd().(cloudLoginMsg)
	if !ok || msg.err == nil {
		t.Fatalf("expected cloudLoginMsg error, got %#v", cmd())
	}

	// A completed login refreshes; a failed one reports.
	refreshRuntime := &Runtime{Query: app.InventoryQuery{Source: "gcp"}, Inventory: &app.InventoryService{}}
	m = NewModel(refreshRuntime)
	model, cmd := m.Update(cloudLoginMsg{provider: domain.ProviderGCP})
	if cmd == nil || !strings.Contains(model.(Model).statusLine, "gcp login complete") {
		t.Fatalf("status=%q cmd nil=%v", model.(Model).statusLine, cmd == nil)
	}
	model, cmd = m.Update(cloudLoginMsg{provider: domain.ProviderAzure, err: context.Canceled})
	if cmd != nil || !strings.Contains(model.(Model).statusLine, "azure login:") {
		t.Fatalf("status=%q", model.(Model).statusLine)
	}
}

func TestModelCloudProviderWarningsAndHeader(t *testing.T) {
	m := NewModel(&Runtime{Query: app.InventoryQuery{Source: "all"}, GCPProject: "proj-a", AzureSubscription: "sub-1"})
	m.statuses = []domain.ProviderStatus{
		{Name: "gcp", Status: domain.ProviderDegraded, Message: "gcloud not logged in: run gcloud auth login (press the login key)"},
		{Name: "azure", Status: domain.ProviderDegraded, Message: "az CLI not found on PATH"},
	}
	got := m.render()
	for _, want := range []string{
		"source gcp degraded: not logged in; L login",
		"source azure degraded: az CLI not installed; see lazyss doctor",
		"GCP: project proj-a",
		"Azure: subscription sub-1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("render missing %q:\n%s", want, got)
		}
	}
	m.statuses = []domain.ProviderStatus{{Name: "gcp", Status: domain.ProviderDegraded, Message: "no gcloud project configured: run gcloud config set project <id>"}}
	if !strings.Contains(m.render(), "source gcp degraded: no project set; P choose") {
		t.Fatalf("missing project hint:\n%s", m.render())
	}
}

func TestFilterMethodAliasesForCloudMethods(t *testing.T) {
	for alias, want := range map[string]domain.AccessMethod{
		"gcloud": domain.AccessGCloudSSH, "gcp": domain.AccessGCloudSSH,
		"iap": domain.AccessGCloudIAPSSH,
		"az":  domain.AccessAzSSH, "azure": domain.AccessAzSSH,
		"bastion": domain.AccessAzBastionSSH,
	} {
		f, err := parseFilterExpression("method:" + alias)
		if err != nil || f.Method != string(want) {
			t.Fatalf("method:%s -> %q, %v; want %q", alias, f.Method, err, want)
		}
	}
}

func TestCycleMethodWalksOnlyMachineMethods(t *testing.T) {
	m := NewModel(&Runtime{})
	m.machines = []domain.Machine{{ID: "g", Name: "g", Provider: domain.ProviderGCP,
		Methods: []domain.AccessMethod{domain.AccessGCloudSSH, domain.AccessGCloudIAPSSH}}}
	m.recompute()
	m.cycleMethod()
	if got := m.visible[0].DefaultMethod(); got != domain.AccessGCloudIAPSSH {
		t.Fatalf("method = %q", got)
	}
	m.cycleMethod()
	if got := m.visible[0].DefaultMethod(); got != domain.AccessGCloudSSH {
		t.Fatalf("method = %q", got)
	}
	if methodDisplay(domain.AccessAzBastionSSH) != "az-bastion" || methodDisplay(domain.AccessGCloudIAPSSH) != "gcloud-iap" {
		t.Fatalf("labels wrong")
	}
}

type blockedTUIProvider struct{ release chan struct{} }

func (blockedTUIProvider) ProviderName() string { return "gcp" }
func (b blockedTUIProvider) ListMachines(context.Context, app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	<-b.release
	return nil, domain.ProviderStatus{Name: "gcp", Status: domain.ProviderHealthy}, nil
}

type fastTUIProvider struct{}

func (fastTUIProvider) ProviderName() string { return "ssh" }
func (fastTUIProvider) ListMachines(context.Context, app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	return []domain.Machine{{ID: "ssh:1", Name: "fast", Provider: domain.ProviderSSH}}, domain.ProviderStatus{Name: "ssh", Status: domain.ProviderHealthy}, nil
}

func TestModelRendersFastProviderWhileSlowOneIsBlocked(t *testing.T) {
	release := make(chan struct{})
	runtime := &Runtime{
		Inventory: &app.InventoryService{Providers: []app.InventoryProvider{blockedTUIProvider{release: release}, fastTUIProvider{}}},
		Query:     app.InventoryQuery{Source: "all"},
	}
	m := NewModel(runtime)
	msg := m.fetchCmd(1)()
	first, ok := msg.(machinesMsg)
	if !ok || len(first.machines) != 1 || first.ch == nil {
		close(release)
		t.Fatalf("first message = %#v", msg)
	}
	m.refreshSeq = 1
	cmd := m.handleMachinesMsg(first)
	if len(m.visible) != 1 || cmd == nil {
		close(release)
		t.Fatalf("fast machine not rendered while slow provider blocked: visible=%d", len(m.visible))
	}
	close(release)
	last := cmd().(machinesMsg)
	if len(last.statuses) != 2 {
		t.Fatalf("final statuses = %#v", last.statuses)
	}
}

func TestModelResolvesGCPProjectInBackground(t *testing.T) {
	runtime := &Runtime{
		Inventory:         &app.InventoryService{},
		Query:             app.InventoryQuery{Source: "gcp"},
		ResolveGCPProject: func(context.Context) (string, error) { return "default-proj", nil },
	}
	m := NewModel(runtime)
	cmd := m.resolveProjectCmd()
	if cmd == nil {
		t.Fatal("expected background resolver")
	}
	model, _ := m.Update(cmd())
	if runtime.GCPProject != "default-proj" {
		t.Fatalf("project = %q", runtime.GCPProject)
	}
	runtime.GCPProject = "picked"
	model.(Model).Update(gcpProjectMsg{project: "late"})
	if runtime.GCPProject != "picked" {
		t.Fatalf("late default overwrote picked project: %q", runtime.GCPProject)
	}
}
