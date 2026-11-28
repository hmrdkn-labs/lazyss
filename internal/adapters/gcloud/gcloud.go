// Package gcloud adapts the gcloud CLI to LazySS inventory, health, and
// connection ports. It never reads or stores tokens; it reuses the user's
// existing gcloud login and always runs gcloud through an argv slice.
package gcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

const (
	gcloudExe        = "gcloud"
	inventoryTimeout = 20 * time.Second
	metadataTimeout  = 10 * time.Second
	dialTimeout      = 3 * time.Second
	providerName     = "gcp"

	metaExternalIP = "external_ip"
	metaInternalIP = "internal_ip"
)

// Runner runs gcloud commands. It matches ports.CommandRunner.
type Runner interface {
	RunOutput(ctx context.Context, exe string, args []string) ([]byte, error)
	RunInteractive(ctx context.Context, exe string, args []string) error
}

type osRunner struct{}

func (osRunner) RunInteractive(ctx context.Context, exe string, args []string) error {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (osRunner) RunOutput(ctx context.Context, exe string, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, exe, args...).Output()
}

func orDefault(r Runner) Runner {
	if r == nil {
		return osRunner{}
	}
	return r
}

// Inventory lists Compute Engine instances through `gcloud compute instances list`.
type Inventory struct {
	runner  Runner
	project string
}

// NewInventory creates a GCP inventory. A nil runner runs the real gcloud CLI;
// an empty project uses the gcloud default and the project in each selfLink.
func NewInventory(runner Runner, project string) Inventory {
	return Inventory{runner: orDefault(runner), project: project}
}

// ProviderName returns the app-level GCP provider key.
func (i Inventory) ProviderName() string { return providerName }

type instance struct {
	Name              string            `json:"name"`
	Zone              string            `json:"zone"`
	Status            string            `json:"status"`
	SelfLink          string            `json:"selfLink"`
	Labels            map[string]string `json:"labels"`
	NetworkInterfaces []struct {
		NetworkIP     string `json:"networkIP"`
		AccessConfigs []struct {
			NatIP string `json:"natIP"`
		} `json:"accessConfigs"`
	} `json:"networkInterfaces"`
}

func (in instance) internalIP() string {
	if len(in.NetworkInterfaces) == 0 {
		return ""
	}
	return in.NetworkInterfaces[0].NetworkIP
}

func (in instance) externalIP() string {
	for _, ni := range in.NetworkInterfaces {
		for _, ac := range ni.AccessConfigs {
			if ac.NatIP != "" {
				return ac.NatIP
			}
		}
	}
	return ""
}

// ListMachines maps Compute Engine instances to provider-neutral machines.
func (i Inventory) ListMachines(ctx context.Context, _ ports.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, inventoryTimeout)
	defer cancel()
	args := []string{"compute", "instances", "list", "--format=json"}
	if i.project != "" {
		args = append(args, "--project", i.project)
	}
	out, err := i.runner.RunOutput(ctx, gcloudExe, args)
	if err != nil {
		return nil, degraded(classifyError(err)), err
	}
	var items []instance
	if err := json.Unmarshal(out, &items); err != nil {
		msg := "gcloud returned unreadable inventory JSON"
		return nil, degraded(msg), fmt.Errorf("%s: %w", msg, err)
	}
	machines := make([]domain.Machine, 0, len(items))
	for _, in := range items {
		machines = append(machines, i.toMachine(in))
	}
	domain.SortMachines(machines)
	return machines, domain.ProviderStatus{Name: providerName, Status: domain.ProviderHealthy}, nil
}

func (i Inventory) toMachine(in instance) domain.Machine {
	project := i.project
	if project == "" {
		project = projectFromSelfLink(in.SelfLink)
	}
	zone := lastSegment(in.Zone)
	ext, internal := in.externalIP(), in.internalIP()
	address := ext
	if address == "" {
		address = internal
	}
	id := domain.MachineID("gcp:" + project + "/" + zone + "/" + in.Name)
	methods := []domain.AccessMethod{domain.AccessGCloudIAPSSH, domain.AccessGCloudSSH}
	if ext != "" {
		methods = []domain.AccessMethod{domain.AccessGCloudSSH, domain.AccessGCloudIAPSSH}
	}
	status, label := providerHealth(in.Status)
	meta := map[string]string{}
	if ext != "" {
		meta[metaExternalIP] = ext
	}
	if internal != "" {
		meta[metaInternalIP] = internal
	}
	return domain.Machine{
		ID:           id,
		Name:         in.Name,
		Provider:     domain.ProviderGCP,
		NativeID:     in.Name,
		Address:      address,
		State:        in.Status,
		Scope:        domain.Scope{Account: project, Region: zone},
		ProviderTags: in.Labels,
		Methods:      methods,
		Health:       domain.NewHealthObservation(id, methods[0], status, label, 0, time.Time{}),
		Metadata:     meta,
	}
}

func providerHealth(state string) (domain.HealthStatus, string) {
	if state == "RUNNING" {
		return domain.HealthUp, "gcp RUNNING"
	}
	if state == "" {
		state = "unknown"
	}
	return domain.HealthDown, "gcp " + state
}

func degraded(msg string) domain.ProviderStatus {
	return domain.ProviderStatus{Name: providerName, Status: domain.ProviderDegraded, Message: msg}
}

func lastSegment(s string) string {
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func projectFromSelfLink(link string) string {
	const marker = "projects/"
	idx := strings.Index(link, marker)
	if idx < 0 {
		return ""
	}
	rest := link[idx+len(marker):]
	if end := strings.Index(rest, "/"); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

const (
	msgNotFound = "gcloud CLI not found on PATH"
	msgNoLogin  = "gcloud not logged in: run gcloud auth login (press the login key)"
	msgNoProj   = "no gcloud project configured: run gcloud config set project <id>"
)

// classifyError turns a gcloud failure into a short operator message.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, exec.ErrNotFound) {
		return msgNotFound
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "gcloud timed out"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := string(exitErr.Stderr)
		for _, marker := range []string{"gcloud auth login", "You do not currently have an active account", "Reauthentication"} {
			if strings.Contains(stderr, marker) {
				return msgNoLogin
			}
		}
		if strings.Contains(stderr, "property [project]") || strings.Contains(stderr, "No project") {
			return msgNoProj
		}
		if line := firstLine(stderr); line != "" {
			return truncate(line, 120)
		}
	}
	return truncate(firstLine(err.Error()), 120)
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func truncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit-3]) + "..."
}

// CLIStatus reports whether gcloud is installed and has an active account, for
// `lazyss doctor`. The message names the active account and nothing else.
func CLIStatus(ctx context.Context, runner Runner) domain.ProviderStatus {
	runner = orDefault(runner)
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	out, err := runner.RunOutput(ctx, gcloudExe, []string{"auth", "list", "--format=json"})
	if err != nil {
		return degraded(classifyError(err))
	}
	var accounts []struct {
		Account string `json:"account"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(out, &accounts); err != nil {
		return degraded("gcloud returned unreadable auth JSON")
	}
	for _, a := range accounts {
		if a.Status == "ACTIVE" {
			return domain.ProviderStatus{Name: providerName, Status: domain.ProviderHealthy, Message: "logged in as " + a.Account}
		}
	}
	return degraded(msgNoLogin)
}

// Project is a GCP project visible to the active gcloud account.
type Project struct {
	ID   string
	Name string
}

// ListProjects runs `gcloud projects list`.
func ListProjects(ctx context.Context, runner Runner) ([]Project, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	out, err := orDefault(runner).RunOutput(ctx, gcloudExe, []string{"projects", "list", "--format=json"})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", classifyError(err), err)
	}
	var raw []struct {
		ProjectID string `json:"projectId"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gcloud projects: %w", err)
	}
	projects := make([]Project, 0, len(raw))
	for _, p := range raw {
		projects = append(projects, Project{ID: p.ProjectID, Name: p.Name})
	}
	return projects, nil
}

// ActiveProject returns the gcloud default project, or "" when unset.
func ActiveProject(ctx context.Context, runner Runner) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	out, err := orDefault(runner).RunOutput(ctx, gcloudExe, []string{"config", "get-value", "project", "--format=json"})
	if err != nil {
		return "", fmt.Errorf("%s: %w", classifyError(err), err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "", nil
	}
	var project string
	if err := json.Unmarshal([]byte(trimmed), &project); err != nil {
		return "", fmt.Errorf("parse gcloud project: %w", err)
	}
	return project, nil
}

// LoginCommand returns the interactive `gcloud auth login` command.
func LoginCommand() ports.CommandSpec {
	return ports.CommandSpec{Executable: gcloudExe, Args: []string{"auth", "login"}}
}

// Connector builds and runs `gcloud compute ssh` sessions.
type Connector struct {
	runner Runner
}

// NewConnector creates a GCP connector. A nil runner runs the real gcloud CLI.
func NewConnector(runner Runner) Connector { return Connector{runner: orDefault(runner)} }

// Supports reports whether the machine is a GCP machine and the method is a GCP one.
func (Connector) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return machine.Provider == domain.ProviderGCP && isGCPMethod(method)
}

func isGCPMethod(m domain.AccessMethod) bool {
	return m == domain.AccessGCloudSSH || m == domain.AccessGCloudIAPSSH
}

// BuildCommand builds a `gcloud compute ssh` argv, rejecting values that
// could be parsed as flags.
func (c Connector) BuildCommand(machine domain.Machine, method domain.AccessMethod, _ ports.ConnectOptions) (ports.CommandSpec, error) {
	if !c.Supports(machine, method) {
		return ports.CommandSpec{}, errors.New("gcloud connector does not support method")
	}
	name, zone, project := machine.NativeID, machine.Scope.Region, machine.Scope.Account
	if name == "" {
		name = machine.Name
	}
	for label, v := range map[string]string{"name": name, "zone": zone, "project": project} {
		if v == "" {
			return ports.CommandSpec{}, fmt.Errorf("gcp machine %s is empty", label)
		}
		if strings.HasPrefix(v, "-") {
			return ports.CommandSpec{}, fmt.Errorf("gcp machine %s %q must not start with '-'", label, v)
		}
	}
	args := []string{"compute", "ssh", name, "--zone", zone, "--project", project}
	if method == domain.AccessGCloudIAPSSH {
		args = append(args, "--tunnel-through-iap")
	}
	return ports.CommandSpec{Executable: gcloudExe, Args: args}, nil
}

// RunInteractive runs the gcloud ssh command attached to the terminal.
func (c Connector) RunInteractive(ctx context.Context, cmd ports.CommandSpec) (app.SessionResult, error) {
	return app.SessionResult{}, c.runner.RunInteractive(ctx, cmd.Executable, cmd.Args)
}

// Dialer checks TCP readiness.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) error
}

type netDialer struct{ timeout time.Duration }

func (d netDialer) DialContext(ctx context.Context, network, address string) error {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	var nd net.Dialer
	conn, err := nd.DialContext(ctx, network, address)
	if err != nil {
		return err
	}
	return conn.Close()
}

// Checker produces health observations for GCP access methods.
type Checker struct {
	dialer  Dialer
	timeout time.Duration
}

// NewChecker creates a GCP health checker. A nil dialer uses net; a timeout
// that is non-positive or above 3 s is 3 s.
func NewChecker(dialer Dialer, timeout time.Duration) Checker {
	if timeout <= 0 || timeout > dialTimeout {
		timeout = dialTimeout
	}
	if dialer == nil {
		dialer = netDialer{timeout: timeout}
	}
	return Checker{dialer: dialer, timeout: timeout}
}

// Supports reports whether this checker handles the method on a GCP machine.
func (Checker) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return machine.Provider == domain.ProviderGCP && isGCPMethod(method)
}

// Check evaluates one GCP access method.
//
// gcloud-ssh: TCP connect to <external IP>:22. "tcp/22 open" proves only that
// the port accepted a TCP connection from this host. It does not prove SSH
// authentication, OS Login, or key acceptance. No external IP is unknown.
//
// gcloud-iap-ssh: reports the instance status from inventory only
// ("IAP not probed"). It does not prove the IAP firewall rule, IAM permission,
// or login. No gcloud call is made.
func (c Checker) Check(ctx context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation {
	now := time.Now()
	switch method {
	case domain.AccessGCloudSSH:
		return c.checkDirect(ctx, machine, method, now)
	case domain.AccessGCloudIAPSSH:
		if machine.State == "RUNNING" {
			return domain.NewHealthObservation(machine.ID, method, domain.HealthUp, "gcp RUNNING (IAP not probed)", 0, now)
		}
		_, label := providerHealth(machine.State)
		return domain.NewHealthObservation(machine.ID, method, domain.HealthDown, label+" (IAP not probed)", 0, now)
	}
	return domain.NewHealthObservation(machine.ID, method, domain.HealthUnknown, "unsupported method", 0, now)
}

func (c Checker) checkDirect(ctx context.Context, machine domain.Machine, method domain.AccessMethod, now time.Time) domain.HealthObservation {
	ip := machine.Metadata[metaExternalIP]
	if ip == "" {
		return domain.NewHealthObservation(machine.ID, method, domain.HealthUnknown, "no external IP", 0, now)
	}
	start := time.Now()
	err := c.dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "22"))
	latency := time.Since(start)
	if err != nil {
		obs := domain.NewHealthObservation(machine.ID, method, domain.HealthDown, "tcp/22 closed", latency, now)
		obs.Error = err.Error()
		return obs
	}
	return domain.NewHealthObservation(machine.ID, method, domain.HealthUp, "tcp/22 open", latency, now)
}
