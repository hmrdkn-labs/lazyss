// Package sshconfig adapts OpenSSH config, direct SSH launch, and TCP checks to
// LazySS ports.
package sshconfig

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hamardikan/lazyss/internal/app"
	"github.com/hamardikan/lazyss/internal/domain"
	"github.com/hamardikan/lazyss/internal/ports"
)

// Inventory reads machines from an OpenSSH config file without mutating it.
type Inventory struct {
	path string
}

// NewInventory creates an SSH config inventory adapter.
func NewInventory(path string) Inventory {
	return Inventory{path: path}
}

// ProviderName returns the app-level SSH provider key.
func (i Inventory) ProviderName() string { return "ssh" }

// ListMachines maps SSH host entries to provider-neutral machines.
func (i Inventory) ListMachines(_ context.Context, _ app.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	file, err := os.Open(i.path)
	if err != nil {
		return nil, domain.ProviderStatus{Name: "ssh", Status: domain.ProviderDegraded, Message: err.Error()}, err
	}
	defer func() { _ = file.Close() }()
	hosts := parse(fileScanner{Scanner: bufio.NewScanner(file)})
	machines := make([]domain.Machine, 0, len(hosts))
	for _, h := range hosts {
		if h.alias == "" || strings.ContainsAny(h.alias, "*?") {
			continue
		}
		if h.isSCMIdentity() || h.isPortForwardAlias() {
			continue
		}
		port := h.port
		if port == 0 {
			port = 22
		}
		address := h.hostname
		if address == "" {
			address = h.alias
		}
		machines = append(machines, domain.Machine{
			ID:       domain.NewSSHMachineID(i.path, h.alias),
			Name:     h.alias,
			Provider: domain.ProviderSSH,
			NativeID: h.alias,
			Address:  address,
			User:     h.user,
			Port:     port,
			Methods:  []domain.AccessMethod{domain.AccessSSH},
			Health:   domain.NewHealthObservation(domain.NewSSHMachineID(i.path, h.alias), domain.AccessSSH, domain.HealthUnknown, "not checked", 0, time.Time{}),
		})
	}
	domain.SortMachines(machines)
	return machines, domain.ProviderStatus{Name: "ssh", Status: domain.ProviderHealthy}, nil
}

type hostBlock struct {
	alias         string
	hostname      string
	user          string
	port          int
	localForwards []string
	identityFiles []string
	proxyCommand  string
}

type scanner interface {
	Scan() bool
	Text() string
}

type fileScanner struct{ *bufio.Scanner }

func parse(sc scanner) []hostBlock {
	var out []hostBlock
	var current *hostBlock
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")
		if key == "host" {
			if current != nil {
				out = append(out, *current)
			}
			current = &hostBlock{alias: fields[1]}
			continue
		}
		if current == nil {
			continue
		}
		switch key {
		case "hostname":
			current.hostname = value
		case "user":
			current.user = value
		case "port":
			if p, err := strconv.Atoi(value); err == nil {
				current.port = p
			}
		case "localforward":
			current.localForwards = append(current.localForwards, value)
		case "identityfile":
			current.identityFiles = append(current.identityFiles, value)
		case "proxycommand":
			current.proxyCommand = value
		}
	}
	if current != nil {
		out = append(out, *current)
	}
	return out
}

func (h hostBlock) isSCMIdentity() bool {
	return strings.EqualFold(h.user, "git") && isSCMHost(h.hostname)
}

func (h hostBlock) isPortForwardAlias() bool {
	return len(h.localForwards) > 0
}

func isSCMHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "github.com", "bitbucket.org", "gitlab.com", "ssh.dev.azure.com":
		return true
	default:
		return false
	}
}

// Runner runs SSH commands for direct sessions and config resolution.
type Runner interface {
	RunInteractive(ctx context.Context, executable string, args []string) error
	RunOutput(ctx context.Context, executable string, args []string) ([]byte, error)
}

// Connector builds and runs direct SSH sessions.
type Connector struct {
	runner Runner
}

// NewConnector creates a direct SSH connector.
func NewConnector(runner Runner, _ any) Connector {
	if runner == nil {
		runner = osRunner{}
	}
	return Connector{runner: runner}
}

// Supports reports whether this connector can launch direct SSH.
func (c Connector) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return method == domain.AccessSSH && (machine.Provider == "" || machine.Provider == domain.ProviderSSH || hasMethod(machine, method))
}

// BuildCommand builds an `ssh <target>` argv.
func (c Connector) BuildCommand(machine domain.Machine, method domain.AccessMethod, _ app.ConnectOptions) (ports.CommandSpec, error) {
	if !c.Supports(machine, method) {
		return ports.CommandSpec{}, errors.New("ssh connector does not support method")
	}
	target := machine.NativeID
	if target == "" {
		target = machine.Name
	}
	return ports.CommandSpec{Executable: "ssh", Args: []string{target}}, nil
}

// RunInteractive runs the SSH command interactively.
func (c Connector) RunInteractive(ctx context.Context, cmd ports.CommandSpec) (app.SessionResult, error) {
	return app.SessionResult{}, c.runner.RunInteractive(ctx, cmd.Executable, cmd.Args)
}

// Dialer checks TCP readiness.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) error
}

// Checker resolves SSH targets and checks TCP readiness.
type Checker struct {
	runner  Runner
	dialer  Dialer
	timeout time.Duration
}

// NewChecker creates an SSH health checker.
func NewChecker(runner Runner, dialer Dialer, timeout time.Duration) Checker {
	if runner == nil {
		runner = osRunner{}
	}
	if dialer == nil {
		dialer = netDialer{timeout: timeout}
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return Checker{runner: runner, dialer: dialer, timeout: timeout}
}

// Supports reports whether this checker can evaluate direct SSH.
func (c Checker) Supports(_ domain.Machine, method domain.AccessMethod) bool {
	return method == domain.AccessSSH
}

// Check resolves an SSH target and checks its TCP port.
func (c Checker) Check(ctx context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation {
	start := time.Now()
	host, port := c.resolve(ctx, machine)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	err := c.dialer.DialContext(ctx, "tcp", addr)
	latency := time.Since(start)
	label := "tcp " + addr
	if err != nil {
		obs := domain.NewHealthObservation(machine.ID, method, domain.HealthDown, label, latency, time.Now())
		obs.Error = err.Error()
		return obs
	}
	return domain.NewHealthObservation(machine.ID, method, domain.HealthUp, label, latency, time.Now())
}

func (c Checker) resolve(ctx context.Context, machine domain.Machine) (string, int) {
	target := machine.NativeID
	if target == "" {
		target = machine.Name
	}
	out, err := c.runner.RunOutput(ctx, "ssh", []string{"-G", target})
	if err != nil {
		host := machine.Address
		if host == "" {
			host = target
		}
		port := machine.Port
		if port == 0 {
			port = 22
		}
		return host, port
	}
	host, port := "", 0
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "hostname":
			host = fields[1]
		case "port":
			port, _ = strconv.Atoi(fields[1])
		}
	}
	if host == "" {
		host = target
	}
	if port == 0 {
		port = 22
	}
	return host, port
}

type osRunner struct{}

func (osRunner) RunInteractive(ctx context.Context, executable string, args []string) error {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (osRunner) RunOutput(ctx context.Context, executable string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, executable, args...).Output()
}

type netDialer struct {
	timeout time.Duration
}

func (d netDialer) DialContext(ctx context.Context, network, address string) error {
	if d.timeout <= 0 {
		d.timeout = 3 * time.Second
	}
	var nd net.Dialer
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	conn, err := nd.DialContext(ctx, network, address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func hasMethod(machine domain.Machine, method domain.AccessMethod) bool {
	for _, m := range machine.Methods {
		if m == method {
			return true
		}
	}
	return false
}
