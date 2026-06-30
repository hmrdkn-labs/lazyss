// Package ports defines application-owned interfaces for external systems.
package ports

import (
	"context"
	"time"

	"github.com/hamardikan/lazyss/internal/domain"
)

// InventoryQuery filters inventory provider results.
type InventoryQuery struct {
	Search     string
	Source     string
	Tags       map[string]string
	NamePrefix string
}

// InventoryProvider lists machines from one source.
type InventoryProvider interface {
	ProviderName() string
	ListMachines(ctx context.Context, q InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error)
}

// ConnectOptions carries interactive connection options.
type ConnectOptions struct{}

// CommandSpec is an executable plus argv; it is never a shell string.
type CommandSpec struct {
	Executable string
	Args       []string
}

// Argv returns the executable and arguments as one argv slice.
func (c CommandSpec) Argv() []string {
	out := []string{c.Executable}
	out = append(out, c.Args...)
	return out
}

// SessionResult reports an interactive session outcome.
type SessionResult struct {
	ExitCode int
}

// Connector builds and runs connection commands for supported methods.
type Connector interface {
	Supports(machine domain.Machine, method domain.AccessMethod) bool
	BuildCommand(machine domain.Machine, method domain.AccessMethod, opts ConnectOptions) (CommandSpec, error)
	RunInteractive(ctx context.Context, cmd CommandSpec) (SessionResult, error)
}

// HealthChecker checks one method-specific access path.
type HealthChecker interface {
	Supports(machine domain.Machine, method domain.AccessMethod) bool
	Check(ctx context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation
}

// StateStore persists local overlays, health, and session history.
type StateStore interface {
	LoadOverlay(ctx context.Context, id domain.MachineID) (domain.MachineOverlay, error)
	SaveOverlay(ctx context.Context, overlay domain.MachineOverlay) error
	RecordHealth(ctx context.Context, obs domain.HealthObservation) error
	RecordSession(ctx context.Context, event domain.SessionEvent) error
}

// Clock provides testable time.
type Clock interface {
	Now() time.Time
}

// CommandRunner runs external commands through argv.
type CommandRunner interface {
	RunInteractive(ctx context.Context, executable string, args []string) error
	RunOutput(ctx context.Context, executable string, args []string) ([]byte, error)
}
