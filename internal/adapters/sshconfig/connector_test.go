package sshconfig

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
)

func TestConnectorBuildsArgvWithoutShell(t *testing.T) {
	conn := NewConnector(fakeRunner{}, nil)
	m := domain.Machine{Name: "prod; rm -rf /", Methods: []domain.AccessMethod{domain.AccessSSH}}
	cmd, err := conn.BuildCommand(m, domain.AccessSSH, app.ConnectOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if cmd.Executable != "ssh" || len(cmd.Args) != 1 || cmd.Args[0] != "prod; rm -rf /" {
		t.Fatalf("unexpected argv: %#v", cmd)
	}
	if strings.Contains(strings.Join(cmd.Args, " "), " sh ") {
		t.Fatalf("should not shell out: %#v", cmd)
	}
}

func TestCheckerUsesResolvedTCPLabel(t *testing.T) {
	runner := fakeRunner{output: "hostname prod.example.com\nport 2222\n"}
	dialer := fakeDialer{}
	checker := NewChecker(runner, dialer, time.Second)
	obs := checker.Check(context.Background(), domain.Machine{Name: "prod"}, domain.AccessSSH)
	if obs.Status != domain.HealthUp || obs.Label != "tcp prod.example.com:2222" || obs.Latency == nil {
		t.Fatalf("observation = %#v", obs)
	}
}

type fakeRunner struct {
	output string
	err    error
}

func (f fakeRunner) RunInteractive(context.Context, string, []string) error { return f.err }
func (f fakeRunner) RunOutput(context.Context, string, []string) ([]byte, error) {
	return []byte(f.output), f.err
}

type fakeDialer struct{ err error }

func (f fakeDialer) DialContext(context.Context, string, string) error { return f.err }
