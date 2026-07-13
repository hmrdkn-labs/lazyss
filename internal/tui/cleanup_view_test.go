package tui

import (
	"strings"
	"testing"

	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

type fakeCleanupPlanner struct {
	plan    ports.CleanupPlan
	applied *ports.CleanupApplyOptions
	result  ports.CleanupApplyResult
}

func (f *fakeCleanupPlanner) PlanCleanup(ports.CleanupOptions) (ports.CleanupPlan, error) {
	return f.plan, nil
}

func (f *fakeCleanupPlanner) ApplyCleanup(opts ports.CleanupApplyOptions) (ports.CleanupApplyResult, error) {
	f.applied = &opts
	return f.result, nil
}

func cleanupPlanFixture() ports.CleanupPlan {
	return ports.CleanupPlan{Items: []ports.CleanupItem{
		{Host: "hidden1", Action: ports.CleanupHide, Reason: "stale host"},
		{Host: "tunnel", Action: ports.CleanupHide, Reason: "port forward alias"},
		{Host: "dup", Action: ports.CleanupDeleteCandidate, Reason: "duplicate target", HostName: "10.0.0.1", User: "ops", Port: 22},
		{Host: "github.com", Action: ports.CleanupDeleteCandidate, Reason: "scm identity", Protected: true},
		{Host: "gitlab.com", Action: ports.CleanupKeep, Reason: "scm identity", Protected: true},
	}}
}

func openCleanupModel(t *testing.T, planner *fakeCleanupPlanner) Model {
	t.Helper()
	m := NewModel(&Runtime{Cleanup: &app.CleanupService{Planner: planner}})
	model, _ := m.openCleanup()
	m = model.(Model)
	if m.mode != modeCleanup {
		t.Fatalf("openCleanup mode = %v", m.mode)
	}
	return m
}

func TestCleanupUnavailableWithoutService(t *testing.T) {
	m := NewModel(nil)
	model, _ := m.Update(keyPress("C"))
	m = model.(Model)
	if m.mode != modeCockpit || m.statusLine != "cleanup unavailable" {
		t.Fatalf("mode=%v status=%q", m.mode, m.statusLine)
	}
}

func TestCleanupPlanRendersAllSections(t *testing.T) {
	m := openCleanupModel(t, &fakeCleanupPlanner{plan: cleanupPlanFixture()})
	got := m.render()
	for _, want := range []string{
		"Hide recommendations",
		"Port-forward recommendations",
		"Delete candidates",
		"Protected",
		"hidden1", "tunnel", "dup", "github.com", "gitlab.com",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plan render missing %q\n%s", want, got)
		}
	}
}

func TestCleanupProtectedDeleteCandidateNotSelectable(t *testing.T) {
	m := openCleanupModel(t, &fakeCleanupPlanner{plan: cleanupPlanFixture()})
	// rows order: hidden1, tunnel, dup, github.com, gitlab.com. github.com is a
	// protected delete candidate at index 3.
	for range 3 {
		model, _ := m.Update(keyPress("down"))
		m = model.(Model)
	}
	model, _ := m.Update(keyPress(" "))
	m = model.(Model)
	if hosts := m.selectedCleanupHosts(); len(hosts) != 0 {
		t.Fatalf("protected host became selectable: %v", hosts)
	}
}

func TestCleanupApplyOnlyAfterConfirmWithSelectedHosts(t *testing.T) {
	planner := &fakeCleanupPlanner{
		plan:   cleanupPlanFixture(),
		result: ports.CleanupApplyResult{BackupPath: "/tmp/config.lazyss-backup-x", RemovedHosts: []string{"dup"}},
	}
	m := openCleanupModel(t, planner)
	// select the "dup" delete candidate at row index 2.
	for range 2 {
		model, _ := m.Update(keyPress("down"))
		m = model.(Model)
	}
	model, _ := m.Update(keyPress(" "))
	m = model.(Model)

	model, _ = m.Update(keyPress("w"))
	m = model.(Model)
	if !m.cleanup.confirm {
		t.Fatal("w did not open confirm modal")
	}
	if got := m.render(); !strings.Contains(got, "dup") || !strings.Contains(got, "lazyss-backup") {
		t.Fatalf("confirm modal missing host or backup note:\n%s", got)
	}
	if planner.applied != nil {
		t.Fatal("apply ran before confirm")
	}

	model, _ = m.Update(keyPress("y"))
	m = model.(Model)
	if planner.applied == nil {
		t.Fatal("apply not called after y")
	}
	if got := planner.applied.Hosts; len(got) != 1 || got[0] != "dup" {
		t.Fatalf("apply hosts = %v, want [dup]", got)
	}
	if m.mode != modeCockpit {
		t.Fatalf("mode after apply = %v", m.mode)
	}
	if !strings.Contains(m.statusLine, "/tmp/config.lazyss-backup-x") || !strings.Contains(m.statusLine, "dup") {
		t.Fatalf("status after apply = %q", m.statusLine)
	}
}

func TestCleanupEscFromConfirmAppliesNothing(t *testing.T) {
	planner := &fakeCleanupPlanner{plan: cleanupPlanFixture()}
	m := openCleanupModel(t, planner)
	for range 2 {
		model, _ := m.Update(keyPress("down"))
		m = model.(Model)
	}
	model, _ := m.Update(keyPress(" "))
	m = model.(Model)
	model, _ = m.Update(keyPress("w"))
	m = model.(Model)

	model, _ = m.Update(keyPress("esc"))
	m = model.(Model)
	if planner.applied != nil {
		t.Fatal("esc from confirm still applied")
	}
	if m.cleanup.confirm || m.mode != modeCleanup {
		t.Fatalf("esc should return to plan list: confirm=%v mode=%v", m.cleanup.confirm, m.mode)
	}
}
