package app

import (
	"context"
	"errors"
	"testing"

	"github.com/hamardikan/lazyss/internal/domain"
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
