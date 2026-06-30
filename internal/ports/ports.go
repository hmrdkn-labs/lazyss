package ports

import (
	"context"
	"time"

	"github.com/hamardikan/lazyss/internal/domain"
)

type InventoryQuery struct {
	Search     string
	Source     string
	Tags       map[string]string
	NamePrefix string
}

type InventoryProvider interface {
	ProviderName() string
	ListMachines(ctx context.Context, q InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error)
}

type ConnectOptions struct{}

type CommandSpec struct {
	Executable string
	Args       []string
}

func (c CommandSpec) Argv() []string {
	out := []string{c.Executable}
	out = append(out, c.Args...)
	return out
}

type SessionResult struct {
	ExitCode int
}

type Connector interface {
	Supports(machine domain.Machine, method domain.AccessMethod) bool
	BuildCommand(machine domain.Machine, method domain.AccessMethod, opts ConnectOptions) (CommandSpec, error)
	RunInteractive(ctx context.Context, cmd CommandSpec) (SessionResult, error)
}

type HealthChecker interface {
	Supports(machine domain.Machine, method domain.AccessMethod) bool
	Check(ctx context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation
}

type StateStore interface {
	LoadOverlay(ctx context.Context, id domain.MachineID) (domain.MachineOverlay, error)
	SaveOverlay(ctx context.Context, overlay domain.MachineOverlay) error
	RecordHealth(ctx context.Context, obs domain.HealthObservation) error
	RecordSession(ctx context.Context, event domain.SessionEvent) error
}

type Clock interface {
	Now() time.Time
}

type CommandRunner interface {
	RunInteractive(ctx context.Context, executable string, args []string) error
	RunOutput(ctx context.Context, executable string, args []string) ([]byte, error)
}
