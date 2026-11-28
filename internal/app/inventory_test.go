package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func TestInventoryAggregatesProvidersAndOverlays(t *testing.T) {
	store := newMemoryStore()
	store.overlays["ssh:a:prod"] = domain.MachineOverlay{MachineID: "ssh:a:prod", Pinned: true, Note: "watch"}
	svc := InventoryService{Providers: []InventoryProvider{
		fakeProvider{name: "ssh", machines: []domain.Machine{{ID: "ssh:a:prod", Name: "prod", Provider: domain.ProviderSSH}}},
		fakeProvider{name: "aws", machines: []domain.Machine{{ID: "aws:ssm:1:r:i-1", Name: "prod", Provider: domain.ProviderAWS}}},
	}, Store: store}
	result, err := svc.List(context.Background(), InventoryQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(result.Machines) != 2 {
		t.Fatalf("machines = %d, want 2", len(result.Machines))
	}
	if !result.Machines[0].Pinned || result.Machines[0].Note != "watch" {
		t.Fatalf("overlay not applied to first sorted machine: %#v", result.Machines[0])
	}
	if result.Machines[0].ID == result.Machines[1].ID {
		t.Fatalf("providers should not accidentally dedupe")
	}
}

func TestInventoryPreservesPartialProviderFailure(t *testing.T) {
	svc := InventoryService{Providers: []InventoryProvider{
		fakeProvider{name: "ssh", machines: []domain.Machine{{ID: "ssh:a:prod", Name: "prod", Provider: domain.ProviderSSH}}},
		fakeProvider{name: "aws", err: errors.New("expired credentials")},
	}, Store: newMemoryStore()}
	result, err := svc.List(context.Background(), InventoryQuery{})
	if err != nil {
		t.Fatalf("partial provider failure should not fail whole inventory: %v", err)
	}
	if len(result.Machines) != 1 {
		t.Fatalf("machines = %d, want 1", len(result.Machines))
	}
	if len(result.Statuses) != 2 || result.Statuses[1].Status != domain.ProviderDegraded {
		t.Fatalf("statuses not degraded: %#v", result.Statuses)
	}
}

func TestInventorySkipsHiddenMachinesUnlessRequested(t *testing.T) {
	store := newMemoryStore()
	store.overlays["ssh:a:hidden"] = domain.MachineOverlay{MachineID: "ssh:a:hidden", Hidden: true}
	svc := InventoryService{Providers: []InventoryProvider{
		fakeProvider{name: "ssh", machines: []domain.Machine{
			{ID: "ssh:a:visible", Name: "visible", Provider: domain.ProviderSSH},
			{ID: "ssh:a:hidden", Name: "hidden", Provider: domain.ProviderSSH},
		}},
	}, Store: store}

	result, err := svc.List(context.Background(), InventoryQuery{})
	if err != nil {
		t.Fatalf("list hidden filtered: %v", err)
	}
	if len(result.Machines) != 1 || result.Machines[0].Name != "visible" {
		t.Fatalf("hidden should be filtered by default: %#v", result.Machines)
	}

	result, err = svc.List(context.Background(), InventoryQuery{ShowHidden: true})
	if err != nil {
		t.Fatalf("list hidden shown: %v", err)
	}
	if len(result.Machines) != 2 {
		t.Fatalf("show hidden machines = %#v", result.Machines)
	}
}

type fakeProvider struct {
	name     string
	machines []domain.Machine
	err      error
}

func (f fakeProvider) ProviderName() string { return f.name }
func (f fakeProvider) ListMachines(context.Context, InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	if f.err != nil {
		return nil, domain.ProviderStatus{Name: f.name, Status: domain.ProviderDegraded, Message: f.err.Error()}, f.err
	}
	return f.machines, domain.ProviderStatus{Name: f.name, Status: domain.ProviderHealthy}, nil
}

type slowProvider struct {
	name  string
	delay time.Duration
	m     domain.Machine
}

func (s slowProvider) ProviderName() string { return s.name }
func (s slowProvider) ListMachines(ctx context.Context, _ InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
	}
	return []domain.Machine{s.m}, domain.ProviderStatus{Name: s.name, Status: domain.ProviderHealthy}, nil
}

func TestInventoryQueriesProvidersConcurrentlyInProviderOrder(t *testing.T) {
	const delay = 150 * time.Millisecond
	svc := InventoryService{Providers: []InventoryProvider{
		slowProvider{name: "gcp", delay: delay, m: domain.Machine{ID: "gcp:1", Name: "b", Provider: domain.ProviderGCP}},
		slowProvider{name: "azure", delay: delay, m: domain.Machine{ID: "az:1", Name: "a", Provider: domain.ProviderAzure}},
		fakeProvider{name: "ssh", err: errors.New("boom")},
	}}
	start := time.Now()
	result, err := svc.List(context.Background(), InventoryQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 2*delay {
		t.Fatalf("providers ran serially: %v", elapsed)
	}
	var names []string
	for _, st := range result.Statuses {
		names = append(names, st.Name)
	}
	if got := strings.Join(names, ","); got != "gcp,azure,ssh" {
		t.Fatalf("status order = %s", got)
	}
	if result.Statuses[2].Status != domain.ProviderDegraded {
		t.Fatalf("failing provider should only degrade itself: %#v", result.Statuses)
	}
	if len(result.Machines) != 2 {
		t.Fatalf("machines = %#v", result.Machines)
	}
}

type blockedProvider struct {
	name    string
	release chan struct{}
}

func (b blockedProvider) ProviderName() string { return b.name }
func (b blockedProvider) ListMachines(ctx context.Context, _ InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	select {
	case <-b.release:
	case <-ctx.Done():
	}
	return []domain.Machine{{ID: "slow:1", Name: "a-slow", Provider: domain.ProviderGCP}}, domain.ProviderStatus{Name: b.name, Status: domain.ProviderHealthy}, nil
}

func TestInventoryStreamEmitsFastProvidersBeforeBlockedOne(t *testing.T) {
	release := make(chan struct{})
	svc := InventoryService{Providers: []InventoryProvider{
		blockedProvider{name: "gcp", release: release},
		fakeProvider{name: "ssh", machines: []domain.Machine{{ID: "ssh:1", Name: "z-fast", Provider: domain.ProviderSSH}}},
	}}
	ch := svc.Stream(context.Background(), InventoryQuery{})
	select {
	case first := <-ch:
		if len(first.Statuses) != 1 || first.Statuses[0].Name != "ssh" || len(first.Machines) != 1 {
			t.Fatalf("first snapshot = %#v", first)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("fast provider was withheld by the blocked one")
	}
	close(release)
	final := <-ch
	if len(final.Statuses) != 2 || final.Statuses[0].Name != "gcp" || final.Statuses[1].Name != "ssh" {
		t.Fatalf("final statuses keep provider order: %#v", final.Statuses)
	}
	if len(final.Machines) != 2 || final.Machines[0].Name != "a-slow" {
		t.Fatalf("final machines sorted: %#v", final.Machines)
	}
	if _, ok := <-ch; ok {
		t.Fatal("stream should close after the last snapshot")
	}
}
