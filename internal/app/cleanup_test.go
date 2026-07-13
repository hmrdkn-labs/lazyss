package app

import (
	"errors"
	"testing"

	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

type fakePlanner struct {
	plan          ports.CleanupPlan
	applyResult   ports.CleanupApplyResult
	applyErr      error
	lastPlanOpts  ports.CleanupOptions
	lastApplyOpts ports.CleanupApplyOptions
	applyCalled   bool
}

func (f *fakePlanner) PlanCleanup(opts ports.CleanupOptions) (ports.CleanupPlan, error) {
	f.lastPlanOpts = opts
	return f.plan, nil
}

func (f *fakePlanner) ApplyCleanup(opts ports.CleanupApplyOptions) (ports.CleanupApplyResult, error) {
	f.applyCalled = true
	f.lastApplyOpts = opts
	return f.applyResult, f.applyErr
}

func TestCleanupServicePlanPassesThroughAndNeverWrites(t *testing.T) {
	planner := &fakePlanner{plan: ports.CleanupPlan{Items: []ports.CleanupItem{
		{Host: "prod", Action: ports.CleanupKeep, Reason: "machine"},
	}}}
	svc := CleanupService{Planner: planner}

	plan, err := svc.Plan(ports.CleanupOptions{Check: true})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Items) != 1 || plan.Items[0].Host != "prod" {
		t.Fatalf("plan not passed through: %#v", plan.Items)
	}
	if !planner.lastPlanOpts.Check {
		t.Fatalf("plan options not forwarded")
	}
	if planner.applyCalled {
		t.Fatalf("dry-run planning must not write")
	}
}

func TestCleanupServiceApplyRefusesProtectedHost(t *testing.T) {
	planner := &fakePlanner{applyErr: errors.New(`host "github.com" is protected scm identity`)}
	svc := CleanupService{Planner: planner}

	if _, err := svc.Apply(ports.CleanupApplyOptions{Hosts: []string{"github.com"}}); err == nil {
		t.Fatalf("expected protected host refusal")
	}
	if planner.lastApplyOpts.Hosts[0] != "github.com" {
		t.Fatalf("host allow-list not forwarded: %#v", planner.lastApplyOpts.Hosts)
	}
}
