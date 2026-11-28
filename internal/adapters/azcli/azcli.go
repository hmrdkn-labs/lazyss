// Package azcli adapts the Azure CLI (`az`) to LazySS inventory, connection,
// and health ports. It never reads or stores tokens: it reuses the user's
// existing `az login` session and always passes argv slices, never shell strings.
package azcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

const (
	azExe = "az"

	inventoryTimeout = 30 * time.Second
	metadataTimeout  = 10 * time.Second
	dialTimeout      = 3 * time.Second

	// MetaBastionName is the Machine.Metadata key carrying the matched Bastion name.
	MetaBastionName = "azure.bastion.name"
	// MetaBastionResourceGroup is the Machine.Metadata key carrying the matched Bastion resource group.
	MetaBastionResourceGroup = "azure.bastion.resourceGroup"

	msgNotFound = "az CLI not found on PATH"
	msgNoLogin  = "az not logged in: run az login (press the login key)"
)

// Runner runs az commands. It matches ports.CommandRunner.
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

// classify turns an az failure into a short operator-facing message.
func classify(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, exec.ErrNotFound) {
		return msgNotFound
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "az timed out"
	}
	var stderr string
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		stderr = string(ee.Stderr)
	}
	if strings.Contains(stderr, "az login") || strings.Contains(stderr, "AADSTS") {
		return msgNoLogin
	}
	if line := firstLine(stderr); line != "" {
		return truncate(line, 200)
	}
	return truncate(firstLine(err.Error()), 200)
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func runJSON(ctx context.Context, r Runner, timeout time.Duration, args []string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := r.RunOutput(ctx, azExe, args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse az output: %w", err)
	}
	return nil
}

func withSub(args []string, subscription string) []string {
	if subscription != "" {
		args = append(args, "--subscription", subscription)
	}
	return args
}

// Inventory lists Azure VMs through the az CLI.
type Inventory struct {
	runner       Runner
	subscription string
}

// NewInventory creates an Azure inventory. An empty subscription uses the az default.
func NewInventory(runner Runner, subscription string) Inventory {
	return Inventory{runner: orDefault(runner), subscription: subscription}
}

// ProviderName returns "azure".
func (Inventory) ProviderName() string { return string(domain.ProviderAzure) }

type azVM struct {
	Name           string            `json:"name"`
	ResourceGroup  string            `json:"resourceGroup"`
	Location       string            `json:"location"`
	ID             string            `json:"id"`
	PowerState     string            `json:"powerState"`
	PublicIPs      string            `json:"publicIps"`
	PrivateIPs     string            `json:"privateIps"`
	Tags           map[string]string `json:"tags"`
	StorageProfile struct {
		OSDisk struct {
			OSType string `json:"osType"`
		} `json:"osDisk"`
	} `json:"storageProfile"`
	NetworkProfile struct {
		NetworkInterfaces []struct {
			ID string `json:"id"`
		} `json:"networkInterfaces"`
	} `json:"networkProfile"`
}

type azSubnetRef struct {
	Subnet struct {
		ID string `json:"id"`
	} `json:"subnet"`
}

type azNIC struct {
	ID               string        `json:"id"`
	IPConfigurations []azSubnetRef `json:"ipConfigurations"`
}

type azBastion struct {
	Name             string        `json:"name"`
	ResourceGroup    string        `json:"resourceGroup"`
	IPConfigurations []azSubnetRef `json:"ipConfigurations"`
}

// ListMachines lists VMs and, best effort, matches each to a Bastion host.
//
// Limitation: a VM is offered Bastion access only when a Bastion sits in the
// same VNet as one of its NICs. Bastions reachable through VNet peering are not
// detected. A failing nic or bastion call degrades to az-ssh only and the
// reason is reported in the status message; the provider stays healthy.
func (inv Inventory) ListMachines(ctx context.Context, _ ports.InventoryQuery) ([]domain.Machine, domain.ProviderStatus, error) {
	name := inv.ProviderName()
	if strings.HasPrefix(inv.subscription, "-") {
		return nil, domain.ProviderStatus{Name: name, Status: domain.ProviderDegraded,
			Message: fmt.Sprintf("invalid azure subscription %q: must not start with '-'", inv.subscription)}, errors.New("invalid azure subscription")
	}
	var vms []azVM
	if err := runJSON(ctx, inv.runner, inventoryTimeout, withSub([]string{"vm", "list", "-d", "-o", "json"}, inv.subscription), &vms); err != nil {
		return nil, domain.ProviderStatus{Name: name, Status: domain.ProviderDegraded, Message: classify(err)}, nil
	}

	var notes []string
	nicVNets := map[string][]string{}
	bastions := map[string]azBastion{}
	var nics []azNIC
	var bastionList []azBastion
	nicErr := runJSON(ctx, inv.runner, metadataTimeout, withSub([]string{"network", "nic", "list", "-o", "json"}, inv.subscription), &nics)
	if nicErr != nil {
		notes = append(notes, "nic list failed ("+classify(nicErr)+"); bastion detection off")
	}
	bastionErr := runJSON(ctx, inv.runner, metadataTimeout, withSub([]string{"network", "bastion", "list", "-o", "json"}, inv.subscription), &bastionList)
	if bastionErr != nil {
		notes = append(notes, "bastion list failed ("+classify(bastionErr)+"); bastion detection off")
	}
	if nicErr == nil && bastionErr == nil {
		for _, n := range nics {
			for _, c := range n.IPConfigurations {
				if v := vnetOf(c.Subnet.ID); v != "" {
					key := strings.ToLower(n.ID)
					nicVNets[key] = append(nicVNets[key], v)
				}
			}
		}
		for _, b := range bastionList {
			for _, c := range b.IPConfigurations {
				if v := vnetOf(c.Subnet.ID); v != "" {
					if _, ok := bastions[v]; !ok {
						bastions[v] = b
					}
				}
			}
		}
	}

	machines := make([]domain.Machine, 0, len(vms))
	for _, vm := range vms {
		machines = append(machines, inv.machine(vm, nicVNets, bastions))
	}
	status := domain.ProviderStatus{Name: name, Status: domain.ProviderHealthy, Message: strings.Join(notes, "; ")}
	return machines, status, nil
}

func (inv Inventory) machine(vm azVM, nicVNets map[string][]string, bastions map[string]azBastion) domain.Machine {
	sub := inv.subscription
	if sub == "" {
		sub = subscriptionOf(vm.ID)
	}
	publicIP := firstCSV(vm.PublicIPs)
	addr := publicIP
	if addr == "" {
		addr = firstCSV(vm.PrivateIPs)
	}
	m := domain.Machine{
		ID:       domain.MachineID("azure:" + sub + "/" + vm.ResourceGroup + "/" + vm.Name),
		Name:     vm.Name,
		Provider: domain.ProviderAzure,
		NativeID: vm.ID,
		Address:  addr,
		Platform: vm.StorageProfile.OSDisk.OSType,
		State:    vm.PowerState,
		Scope:    domain.Scope{Account: sub, Region: vm.Location, Profile: vm.ResourceGroup},
		Methods:  []domain.AccessMethod{domain.AccessAzSSH},
		Health:   powerHealth(vm.PowerState),
	}
	m.Health.MachineID = m.ID
	if len(vm.Tags) > 0 {
		m.ProviderTags = make(map[string]string, len(vm.Tags))
		for k, v := range vm.Tags {
			m.ProviderTags[k] = v
		}
	}
	if b, ok := matchBastion(vm, nicVNets, bastions); ok {
		m.Metadata = map[string]string{MetaBastionName: b.Name, MetaBastionResourceGroup: b.ResourceGroup}
		if publicIP == "" {
			m.Methods = []domain.AccessMethod{domain.AccessAzBastionSSH, domain.AccessAzSSH}
		} else {
			m.Methods = append(m.Methods, domain.AccessAzBastionSSH)
		}
	}
	return m
}

func matchBastion(vm azVM, nicVNets map[string][]string, bastions map[string]azBastion) (azBastion, bool) {
	for _, nic := range vm.NetworkProfile.NetworkInterfaces {
		for _, v := range nicVNets[strings.ToLower(nic.ID)] {
			if b, ok := bastions[v]; ok {
				return b, true
			}
		}
	}
	return azBastion{}, false
}

// powerHealth derives the initial health from the power state alone. It does
// not claim login or connectivity works.
func powerHealth(state string) domain.HealthObservation {
	switch {
	case state == "":
		return domain.HealthObservation{Status: domain.HealthUnknown, Label: "azure power state unknown"}
	case state == "VM running":
		return domain.HealthObservation{Status: domain.HealthUp, Label: "azure VM running"}
	default:
		return domain.HealthObservation{Status: domain.HealthDown, Label: "azure " + state}
	}
}

func firstCSV(s string) string {
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			return p
		}
	}
	return ""
}

// vnetOf returns the lowercase VNet resource id of a subnet id, or "".
func vnetOf(subnetID string) string {
	lower := strings.ToLower(subnetID)
	i := strings.Index(lower, "/subnets/")
	if i < 0 {
		return ""
	}
	return lower[:i]
}

func subscriptionOf(resourceID string) string {
	parts := strings.Split(resourceID, "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "subscriptions") {
			return parts[i+1]
		}
	}
	return ""
}

// Subscription is one Azure subscription visible to the logged-in user.
type Subscription struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	State     string `json:"state"`
}

// ListSubscriptions lists subscriptions from `az account list`.
func ListSubscriptions(ctx context.Context, runner Runner) ([]Subscription, error) {
	var subs []Subscription
	if err := runJSON(ctx, orDefault(runner), metadataTimeout, []string{"account", "list", "-o", "json"}, &subs); err != nil {
		return nil, errors.New(classify(err))
	}
	return subs, nil
}

// CLIStatus reports az CLI presence, login, and ssh extension for doctor. The
// message names only the subscription name and id.
func CLIStatus(ctx context.Context, runner Runner) domain.ProviderStatus {
	runner = orDefault(runner)
	st := domain.ProviderStatus{Name: string(domain.ProviderAzure)}
	var acct struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := runJSON(ctx, runner, metadataTimeout, []string{"account", "show", "-o", "json"}, &acct); err != nil {
		st.Status = domain.ProviderDegraded
		st.Message = classify(err)
		return st
	}
	msg := fmt.Sprintf("az logged in: subscription %s (%s)", acct.Name, acct.ID)
	var exts []struct {
		Name string `json:"name"`
	}
	extErr := runJSON(ctx, runner, metadataTimeout, []string{"extension", "list", "-o", "json"}, &exts)
	installed := false
	for _, e := range exts {
		if e.Name == "ssh" {
			installed = true
		}
	}
	switch {
	case extErr != nil:
		st.Status = domain.ProviderDegraded
		st.Message = msg + "; ssh extension check failed: " + classify(extErr)
	case !installed:
		st.Status = domain.ProviderDegraded
		st.Message = msg + "; az ssh extension missing: run az extension add --name ssh"
	default:
		st.Status = domain.ProviderHealthy
		st.Message = msg + "; ssh extension installed"
	}
	return st
}

// LoginCommand returns the interactive `az login` command.
func LoginCommand() ports.CommandSpec {
	return ports.CommandSpec{Executable: azExe, Args: []string{"login"}}
}

// Connector builds and runs Azure CLI sessions.
type Connector struct {
	runner Runner
}

// NewConnector creates an Azure connector.
func NewConnector(runner Runner) Connector {
	return Connector{runner: orDefault(runner)}
}

func isAzureMethod(method domain.AccessMethod) bool {
	return method == domain.AccessAzSSH || method == domain.AccessAzBastionSSH
}

// Supports reports whether method is an Azure method on an Azure machine.
func (Connector) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return machine.Provider == domain.ProviderAzure && isAzureMethod(method)
}

// BuildCommand builds the az argv for the method. Values starting with "-" are rejected.
func (c Connector) BuildCommand(machine domain.Machine, method domain.AccessMethod, _ ports.ConnectOptions) (ports.CommandSpec, error) {
	if !c.Supports(machine, method) {
		return ports.CommandSpec{}, errors.New("azure connector does not support method")
	}
	rg, sub := machine.Scope.Profile, machine.Scope.Account
	values := []string{rg, machine.Name, sub}
	var args []string
	switch method {
	case domain.AccessAzSSH:
		args = []string{"ssh", "vm", "--resource-group", rg, "--name", machine.Name, "--subscription", sub}
	default:
		bn, brg := machine.Metadata[MetaBastionName], machine.Metadata[MetaBastionResourceGroup]
		if bn == "" || brg == "" {
			return ports.CommandSpec{}, errors.New("no bastion matched for machine")
		}
		values = []string{bn, brg, machine.NativeID, sub}
		args = []string{"network", "bastion", "ssh", "--name", bn, "--resource-group", brg,
			"--target-resource-id", machine.NativeID, "--auth-type", "AAD", "--subscription", sub}
	}
	for _, v := range values {
		if v == "" {
			return ports.CommandSpec{}, errors.New("missing machine scope value for az command")
		}
		if strings.HasPrefix(v, "-") {
			return ports.CommandSpec{}, fmt.Errorf("refusing value starting with '-': %q", v)
		}
	}
	return ports.CommandSpec{Executable: azExe, Args: args}, nil
}

// RunInteractive runs the command attached to the terminal.
func (c Connector) RunInteractive(ctx context.Context, cmd ports.CommandSpec) (ports.SessionResult, error) {
	err := c.runner.RunInteractive(ctx, cmd.Executable, cmd.Args)
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ports.SessionResult{ExitCode: ee.ExitCode()}, err
	}
	return ports.SessionResult{}, err
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

// Checker evaluates Azure access paths without shelling out to az.
type Checker struct {
	dialer  Dialer
	timeout time.Duration
}

// NewChecker creates an Azure health checker. A nil dialer uses TCP; timeout is capped at 3 s.
func NewChecker(dialer Dialer, timeout time.Duration) Checker {
	if timeout <= 0 || timeout > dialTimeout {
		timeout = dialTimeout
	}
	if dialer == nil {
		dialer = netDialer{timeout: timeout}
	}
	return Checker{dialer: dialer, timeout: timeout}
}

// Supports reports whether method is an Azure method on an Azure machine.
func (Checker) Supports(machine domain.Machine, method domain.AccessMethod) bool {
	return machine.Provider == domain.ProviderAzure && isAzureMethod(method)
}

// Check dispatches to the method-specific check.
func (c Checker) Check(ctx context.Context, machine domain.Machine, method domain.AccessMethod) domain.HealthObservation {
	now := time.Now()
	switch method {
	case domain.AccessAzSSH:
		return c.checkSSH(ctx, machine, now)
	case domain.AccessAzBastionSSH:
		return checkBastion(machine, now)
	}
	obs := domain.NewHealthObservation(machine.ID, method, domain.HealthUnknown, "unsupported method", 0, now)
	obs.Error = "azure checker does not support method"
	return obs
}

// checkSSH proves only that <public IP>:22 accepted a TCP connection within
// 3 s (the dial context is bounded by the checker timeout, capped at 3 s). A
// failure is labelled "refused" only for ECONNREFUSED; timeouts, cancellation
// and unreachable networks are "probe failed" and prove nothing about the port. It does not prove Entra ID login, RBAC, or that the AADSSHLogin VM
// extension is installed. A VM without a public IP is not dialed; the result
// reports its power state.
func (c Checker) checkSSH(ctx context.Context, m domain.Machine, now time.Time) domain.HealthObservation {
	ip := net.ParseIP(m.Address)
	if ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		state := m.State
		if state == "" {
			state = "unknown"
		}
		status := domain.HealthUnknown
		if m.State != "" && m.State != "VM running" {
			status = domain.HealthDown
		}
		return domain.NewHealthObservation(m.ID, domain.AccessAzSSH, status, "no public IP; power state "+state, 0, now)
	}
	addr := net.JoinHostPort(m.Address, "22")
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	err := c.dialer.DialContext(ctx, "tcp", addr)
	latency := time.Since(now)
	if err != nil {
		label := "tcp/22 probe failed"
		if errors.Is(err, syscall.ECONNREFUSED) {
			label = "tcp/22 refused"
		}
		obs := domain.NewHealthObservation(m.ID, domain.AccessAzSSH, domain.HealthDown, label, latency, time.Now())
		obs.Error = err.Error()
		return obs
	}
	return domain.NewHealthObservation(m.ID, domain.AccessAzSSH, domain.HealthUp, "tcp/22 open", latency, time.Now())
}

// checkBastion reports power state and Bastion presence taken from inventory.
// It proves nothing about the Bastion SKU supporting native-client tunnelling,
// RBAC, or Entra login, and never calls az.
func checkBastion(m domain.Machine, now time.Time) domain.HealthObservation {
	name := m.Metadata[MetaBastionName]
	if name == "" {
		obs := domain.NewHealthObservation(m.ID, domain.AccessAzBastionSSH, domain.HealthDown, "no bastion in VM VNet", 0, now)
		obs.Error = "no bastion matched in inventory"
		return obs
	}
	if m.State != "VM running" {
		state := m.State
		if state == "" {
			state = "unknown"
		}
		return domain.NewHealthObservation(m.ID, domain.AccessAzBastionSSH, domain.HealthDown, "power state "+state+", bastion "+name, 0, now)
	}
	return domain.NewHealthObservation(m.ID, domain.AccessAzBastionSSH, domain.HealthUp, "VM running, bastion "+name, 0, now)
}
