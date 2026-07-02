package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

func TestConnectRecordsSuccessAndFailure(t *testing.T) {
	store := newMemoryStore()
	clock := fixedClock{t: time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)}
	machine := domain.Machine{ID: "ssh:a:prod", Name: "prod", Methods: []domain.AccessMethod{domain.AccessSSH}}
	svc := ConnectService{Connectors: []ports.Connector{fakeConnector{}}, Store: store, Clock: clock}

	if _, err := svc.Connect(context.Background(), machine, domain.AccessSSH, ConnectOptions{}); err != nil {
		t.Fatalf("connect success: %v", err)
	}
	overlay, _ := store.LoadOverlay(context.Background(), machine.ID)
	if overlay.ConnectionCount != 1 || overlay.LastConnectedAt.IsZero() {
		t.Fatalf("success not recorded: %#v", overlay)
	}

	failSvc := ConnectService{Connectors: []ports.Connector{fakeConnector{err: errors.New("dial failed")}}, Store: store, Clock: clock}
	_, _ = failSvc.Connect(context.Background(), machine, domain.AccessSSH, ConnectOptions{})
	overlay2, _ := store.LoadOverlay(context.Background(), machine.ID)
	if overlay2.ConnectionCount != 1 {
		t.Fatalf("failed connect should not increment success count: %#v", overlay2)
	}
	if len(store.sessions[machine.ID]) != 2 || store.sessions[machine.ID][1].Success {
		t.Fatalf("failure event not recorded: %#v", store.sessions[machine.ID])
	}
}

type fakeConnector struct{ err error }

func (f fakeConnector) Supports(domain.Machine, domain.AccessMethod) bool { return true }
func (f fakeConnector) BuildCommand(domain.Machine, domain.AccessMethod, ConnectOptions) (ports.CommandSpec, error) {
	return ports.CommandSpec{Executable: "ssh", Args: []string{"prod"}}, nil
}
func (f fakeConnector) RunInteractive(context.Context, ports.CommandSpec) (SessionResult, error) {
	return SessionResult{ExitCode: 0}, f.err
}
