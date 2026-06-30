package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/hamardikan/lazyss/internal/domain"
	"github.com/hamardikan/lazyss/internal/ports"
)

type InventoryQuery = ports.InventoryQuery
type InventoryProvider = ports.InventoryProvider
type ConnectOptions = ports.ConnectOptions
type SessionResult = ports.SessionResult
type HealthChecker = ports.HealthChecker

type InventoryResult struct {
	Machines []domain.Machine
	Statuses []domain.ProviderStatus
}

type InventoryService struct {
	Providers []InventoryProvider
	Store     ports.StateStore
}

func (s InventoryService) List(ctx context.Context, q InventoryQuery) (InventoryResult, error) {
	var result InventoryResult
	for _, provider := range s.Providers {
		if q.Source != "" && q.Source != "all" && provider.ProviderName() != q.Source {
			continue
		}
		machines, status, err := provider.ListMachines(ctx, q)
		if err != nil {
			if status.Name == "" {
				status.Name = provider.ProviderName()
			}
			status.Status = domain.ProviderDegraded
			if status.Message == "" {
				status.Message = err.Error()
			}
			result.Statuses = append(result.Statuses, status)
			continue
		}
		if status.Name == "" {
			status.Name = provider.ProviderName()
		}
		if status.Status == "" {
			status.Status = domain.ProviderHealthy
		}
		result.Statuses = append(result.Statuses, status)
		for _, m := range machines {
			if s.Store != nil {
				overlay, err := s.Store.LoadOverlay(ctx, m.ID)
				if err == nil {
					m.ApplyOverlay(overlay)
				}
			}
			if q.Search == "" || strings.Contains(strings.ToLower(m.Name+" "+m.Address+" "+m.NativeID), strings.ToLower(q.Search)) {
				result.Machines = append(result.Machines, m)
			}
		}
	}
	domain.SortMachines(result.Machines)
	return result, nil
}

type ConnectService struct {
	Connectors []ports.Connector
	Store      ports.StateStore
	Clock      ports.Clock
}

func (s ConnectService) Connect(ctx context.Context, machine domain.Machine, method domain.AccessMethod, opts ConnectOptions) (SessionResult, error) {
	connector := s.connectorFor(machine, method)
	if connector == nil {
		return SessionResult{}, errors.New("no connector supports selected method")
	}
	started := now(s.Clock)
	cmd, err := connector.BuildCommand(machine, method, opts)
	if err != nil {
		return SessionResult{}, err
	}
	result, runErr := connector.RunInteractive(ctx, cmd)
	ended := now(s.Clock)
	event := domain.SessionEvent{
		MachineID: machine.ID,
		Method:    method,
		StartedAt: started,
		EndedAt:   ended,
		Success:   runErr == nil,
		ExitCode:  result.ExitCode,
		Command:   cmd.Argv(),
	}
	if runErr != nil {
		event.Error = runErr.Error()
	}
	if s.Store != nil {
		_ = s.Store.RecordSession(ctx, event)
	}
	return result, runErr
}

func (s ConnectService) BuildCommand(machine domain.Machine, method domain.AccessMethod, opts ConnectOptions) (ports.CommandSpec, error) {
	connector := s.connectorFor(machine, method)
	if connector == nil {
		return ports.CommandSpec{}, errors.New("no connector supports selected method")
	}
	return connector.BuildCommand(machine, method, opts)
}

func (s ConnectService) connectorFor(machine domain.Machine, method domain.AccessMethod) ports.Connector {
	for _, connector := range s.Connectors {
		if connector.Supports(machine, method) {
			return connector
		}
	}
	return nil
}

type HealthService struct {
	Checkers      []HealthChecker
	Store         ports.StateStore
	MaxConcurrent int
}

func (s HealthService) CheckSelected(ctx context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation {
	checker := s.checkerFor(machine, method)
	if checker == nil {
		return domain.NewHealthObservation(machine.ID, method, domain.HealthUnknown, "no checker", 0, time.Now())
	}
	obs := checker.Check(ctx, machine, method)
	if s.Store != nil {
		_ = s.Store.RecordHealth(ctx, obs)
	}
	return obs
}

func (s HealthService) CheckVisible(ctx context.Context, machines []domain.Machine, method domain.AccessMethod) []domain.HealthObservation {
	limit := s.MaxConcurrent
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	out := make([]domain.HealthObservation, len(machines))
	var wg sync.WaitGroup
	for i, machine := range machines {
		i, machine := i, machine
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out[i] = domain.NewHealthObservation(machine.ID, method, domain.HealthUnknown, "cancelled", 0, time.Now())
				return
			}
			out[i] = s.CheckSelected(ctx, machine, method)
		}()
	}
	wg.Wait()
	return out
}

func (s HealthService) checkerFor(machine domain.Machine, method domain.AccessMethod) HealthChecker {
	for _, checker := range s.Checkers {
		if checker.Supports(machine, method) {
			return checker
		}
	}
	return nil
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func now(clock ports.Clock) time.Time {
	if clock == nil {
		return time.Now()
	}
	return clock.Now()
}
