package tui

import (
	"context"
	"strings"
	"testing"
	"time"

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
	m := NewModel(nil)
	raw := "operation error SSM: DescribeInstanceInformation, https response error StatusCode: 400, RequestID: 6d6fec41-b934-4298-82fe-a479f2250bd5, api error UnrecognizedClientException: The security token included in the request is invalid"
	m.statuses = []domain.ProviderStatus{{Name: "aws", Status: domain.ProviderDegraded, Message: raw}}
	got := m.render()
	if strings.Contains(got, "RequestID") {
		t.Fatalf("render leaked raw AWS request details: %s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "source aws degraded:") && len(line) > maxProviderWarningRunes {
			t.Fatalf("provider warning too long: %d %q", len(line), line)
		}
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
