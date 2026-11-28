package azcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

const sub = "00000000-0000-0000-0000-000000000001"

type call struct {
	exe  string
	args []string
}

type fakeRunner struct {
	calls       []call
	out         map[string][]byte
	errs        map[string]error
	interactive error
}

func (f *fakeRunner) RunOutput(_ context.Context, exe string, args []string) ([]byte, error) {
	f.calls = append(f.calls, call{exe, args})
	k := strings.Join(args, " ")
	if err, ok := f.errs[k]; ok {
		return nil, err
	}
	if o, ok := f.out[k]; ok {
		return o, nil
	}
	return nil, errors.New("unexpected call: " + k)
}

func (f *fakeRunner) RunInteractive(_ context.Context, exe string, args []string) error {
	f.calls = append(f.calls, call{exe, args})
	return f.interactive
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func inventoryRunner(t *testing.T, subFlag string) *fakeRunner {
	s := ""
	if subFlag != "" {
		s = " --subscription " + subFlag
	}
	return &fakeRunner{out: map[string][]byte{
		"vm list -d -o json" + s:           fixture(t, "vm_list.json"),
		"network nic list -o json" + s:     fixture(t, "nic_list.json"),
		"network bastion list -o json" + s: fixture(t, "bastion_list.json"),
	}, errs: map[string]error{}}
}

func byName(ms []domain.Machine, n string) domain.Machine {
	for _, m := range ms {
		if m.Name == n {
			return m
		}
	}
	return domain.Machine{}
}

func TestListMachines(t *testing.T) {
	r := inventoryRunner(t, sub)
	ms, st, err := NewInventory(r, sub).ListMachines(context.Background(), ports.InventoryQuery{})
	if err != nil || st.Status != domain.ProviderHealthy || st.Message != "" || st.Name != "azure" {
		t.Fatalf("err=%v st=%+v", err, st)
	}
	wantCalls := [][]string{
		{"vm", "list", "-d", "-o", "json", "--subscription", sub},
		{"network", "nic", "list", "-o", "json", "--subscription", sub},
		{"network", "bastion", "list", "-o", "json", "--subscription", sub},
	}
	for i, w := range wantCalls {
		if r.calls[i].exe != "az" || !reflect.DeepEqual(r.calls[i].args, w) {
			t.Errorf("call %d = %v", i, r.calls[i])
		}
	}
	if len(ms) != 3 {
		t.Fatalf("machines = %d", len(ms))
	}
	web := byName(ms, "web-1")
	if web.ID != "azure:"+domain.MachineID(sub)+"/rg-app/web-1" || web.Address != "20.1.2.3" || web.Platform != "Linux" ||
		web.Scope != (domain.Scope{Account: sub, Region: "westeurope", Profile: "rg-app"}) || web.ProviderTags["env"] != "prod" ||
		web.State != "VM running" || web.Health.Status != domain.HealthUp || web.Health.Label != "azure VM running" || web.Health.MachineID != web.ID {
		t.Errorf("web = %+v", web)
	}
	if !reflect.DeepEqual(web.Methods, []domain.AccessMethod{domain.AccessAzSSH, domain.AccessAzBastionSSH}) {
		t.Errorf("web methods = %v", web.Methods)
	}
	db := byName(ms, "db-1")
	if db.Address != "10.0.2.4" || !reflect.DeepEqual(db.Methods, []domain.AccessMethod{domain.AccessAzBastionSSH, domain.AccessAzSSH}) ||
		db.Metadata[MetaBastionName] != "bastion-app" || db.Metadata[MetaBastionResourceGroup] != "rg-net" || db.ProviderTags != nil {
		t.Errorf("db = %+v", db)
	}
	old := byName(ms, "old-1")
	if old.Platform != "Windows" || old.Health.Status != domain.HealthDown || old.Health.Label != "azure VM deallocated" ||
		!reflect.DeepEqual(old.Methods, []domain.AccessMethod{domain.AccessAzSSH}) || old.Metadata != nil {
		t.Errorf("old = %+v", old)
	}
}

func TestListMachinesParsesSubscriptionFromID(t *testing.T) {
	ms, _, _ := NewInventory(inventoryRunner(t, ""), "").ListMachines(context.Background(), ports.InventoryQuery{})
	if got := byName(ms, "web-1"); got.Scope.Account != sub || got.ID != domain.MachineID("azure:"+sub+"/rg-app/web-1") {
		t.Errorf("web = %+v", got)
	}
}

func TestListMachinesDegradesWithoutNICOrBastion(t *testing.T) {
	for _, failing := range []string{"network nic list -o json", "network bastion list -o json"} {
		r := inventoryRunner(t, "")
		r.errs[failing] = errors.New("boom")
		ms, st, err := NewInventory(r, "").ListMachines(context.Background(), ports.InventoryQuery{})
		if err != nil || st.Status != domain.ProviderHealthy || !strings.Contains(st.Message, "bastion detection off") {
			t.Fatalf("%s: err=%v st=%+v", failing, err, st)
		}
		for _, m := range ms {
			if !reflect.DeepEqual(m.Methods, []domain.AccessMethod{domain.AccessAzSSH}) {
				t.Errorf("%s: %s methods = %v", failing, m.Name, m.Methods)
			}
		}
	}
}

func TestListMachinesVMListFailure(t *testing.T) {
	r := &fakeRunner{errs: map[string]error{"vm list -d -o json": exec.ErrNotFound}}
	ms, st, err := NewInventory(r, "").ListMachines(context.Background(), ports.InventoryQuery{})
	if err != nil || ms != nil || st.Status != domain.ProviderDegraded || st.Message != "az CLI not found on PATH" {
		t.Errorf("ms=%v st=%+v err=%v", ms, st, err)
	}
}

func TestListMachinesBadJSON(t *testing.T) {
	r := &fakeRunner{out: map[string][]byte{"vm list -d -o json": []byte("nope")}}
	_, st, _ := NewInventory(r, "").ListMachines(context.Background(), ports.InventoryQuery{})
	if st.Status != domain.ProviderDegraded || !strings.Contains(st.Message, "parse az output") {
		t.Errorf("st = %+v", st)
	}
}

func TestClassify(t *testing.T) {
	long := strings.Repeat("x", 300)
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"not found", &exec.Error{Name: "az", Err: exec.ErrNotFound}, "az CLI not found on PATH"},
		{"login", &exec.ExitError{Stderr: []byte("ERROR: Please run 'az login' to setup account.")}, msgNoLogin},
		{"aadsts", &exec.ExitError{Stderr: []byte("AADSTS70043: token expired")}, msgNoLogin},
		{"other", &exec.ExitError{Stderr: []byte("\nERROR: boom\nsecond line")}, "ERROR: boom"},
		{"truncated", &exec.ExitError{Stderr: []byte(long)}, strings.Repeat("x", 200) + "..."},
		{"timeout", context.DeadlineExceeded, "az timed out"},
		{"no stderr", errors.New("plain failure"), "plain failure"},
	}
	for _, c := range cases {
		if got := classify(c.err); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestCLIStatus(t *testing.T) {
	show := "account show -o json"
	ext := "extension list -o json"
	ok := func() *fakeRunner {
		return &fakeRunner{out: map[string][]byte{show: fixture(t, "account_show.json"), ext: fixture(t, "extension_list.json")}, errs: map[string]error{}}
	}
	r := ok()
	st := CLIStatus(context.Background(), r)
	if st.Status != domain.ProviderHealthy || !strings.Contains(st.Message, "Prod ("+sub+")") || !strings.Contains(st.Message, "ssh extension installed") {
		t.Errorf("st = %+v", st)
	}
	if strings.Contains(st.Message, "secret-tenant") || strings.Contains(st.Message, "x@y") {
		t.Errorf("leaks identity: %q", st.Message)
	}

	r = ok()
	r.out[ext] = []byte(`[{"name":"account"}]`)
	if st := CLIStatus(context.Background(), r); st.Status != domain.ProviderDegraded || !strings.Contains(st.Message, "extension missing") {
		t.Errorf("missing ext: %+v", st)
	}
	r = ok()
	delete(r.out, ext)
	r.errs[ext] = errors.New("x")
	if st := CLIStatus(context.Background(), r); st.Status != domain.ProviderDegraded || !strings.Contains(st.Message, "extension check failed") {
		t.Errorf("ext err: %+v", st)
	}
	r = ok()
	r.errs[show] = &exec.ExitError{Stderr: []byte("Please run 'az login'")}
	if st := CLIStatus(context.Background(), r); st.Status != domain.ProviderDegraded || st.Message != msgNoLogin {
		t.Errorf("not logged in: %+v", st)
	}
}

func TestListSubscriptions(t *testing.T) {
	r := &fakeRunner{out: map[string][]byte{"account list -o json": fixture(t, "account_list.json")}}
	subs, err := ListSubscriptions(context.Background(), r)
	if err != nil || len(subs) != 2 || subs[0] != (Subscription{ID: sub, Name: "Prod", IsDefault: true, State: "Enabled"}) || subs[1].State != "Disabled" {
		t.Errorf("subs=%+v err=%v", subs, err)
	}
	r = &fakeRunner{errs: map[string]error{"account list -o json": exec.ErrNotFound}}
	if _, err := ListSubscriptions(context.Background(), r); err == nil || err.Error() != "az CLI not found on PATH" {
		t.Errorf("err = %v", err)
	}
}

func TestLoginCommand(t *testing.T) {
	if got := LoginCommand(); got.Executable != "az" || !reflect.DeepEqual(got.Args, []string{"login"}) {
		t.Errorf("got %+v", got)
	}
}

func azMachine() domain.Machine {
	return domain.Machine{
		Name: "db-1", Provider: domain.ProviderAzure,
		NativeID: "/subscriptions/" + sub + "/resourceGroups/rg-app/providers/Microsoft.Compute/virtualMachines/db-1",
		Scope:    domain.Scope{Account: sub, Region: "westeurope", Profile: "rg-app"},
		Metadata: map[string]string{MetaBastionName: "bastion-app", MetaBastionResourceGroup: "rg-net"},
	}
}

func TestBuildCommandGolden(t *testing.T) {
	c := NewConnector(nil)
	m := azMachine()
	got, err := c.BuildCommand(m, domain.AccessAzSSH, ports.ConnectOptions{})
	want := []string{"az", "ssh", "vm", "--resource-group", "rg-app", "--name", "db-1", "--subscription", sub}
	if err != nil || !reflect.DeepEqual(got.Argv(), want) {
		t.Errorf("ssh: %v %v", got.Argv(), err)
	}
	got, err = c.BuildCommand(m, domain.AccessAzBastionSSH, ports.ConnectOptions{})
	want = []string{"az", "network", "bastion", "ssh", "--name", "bastion-app", "--resource-group", "rg-net",
		"--target-resource-id", m.NativeID, "--auth-type", "AAD", "--subscription", sub}
	if err != nil || !reflect.DeepEqual(got.Argv(), want) {
		t.Errorf("bastion: %v %v", got.Argv(), err)
	}
}

func TestBuildCommandRejects(t *testing.T) {
	c := NewConnector(nil)
	for _, mut := range []func(*domain.Machine){
		func(m *domain.Machine) { m.Name = "-oProxyCommand=x" },
		func(m *domain.Machine) { m.Scope.Profile = "--evil" },
		func(m *domain.Machine) { m.Scope.Account = "-s" },
		func(m *domain.Machine) { m.Scope.Account = "" },
	} {
		m := azMachine()
		mut(&m)
		if _, err := c.BuildCommand(m, domain.AccessAzSSH, ports.ConnectOptions{}); err == nil {
			t.Errorf("ssh accepted %+v", m)
		}
	}
	bm := azMachine()
	bm.Scope.Account = "-s"
	if _, err := c.BuildCommand(bm, domain.AccessAzBastionSSH, ports.ConnectOptions{}); err == nil {
		t.Error("bastion accepted dash subscription")
	}
	for _, mut := range []func(*domain.Machine){
		func(m *domain.Machine) { m.Metadata[MetaBastionName] = "-x" },
		func(m *domain.Machine) { m.Metadata[MetaBastionResourceGroup] = "--x" },
		func(m *domain.Machine) { m.NativeID = "-x" },
		func(m *domain.Machine) { m.Metadata = nil },
	} {
		m := azMachine()
		mut(&m)
		if _, err := c.BuildCommand(m, domain.AccessAzBastionSSH, ports.ConnectOptions{}); err == nil {
			t.Errorf("bastion accepted %+v", m)
		}
	}
	if _, err := c.BuildCommand(azMachine(), domain.AccessSSH, ports.ConnectOptions{}); err == nil {
		t.Error("accepted unsupported method")
	}
}

func TestConnectorSupportsAndRun(t *testing.T) {
	r := &fakeRunner{}
	c := NewConnector(r)
	m := azMachine()
	if !c.Supports(m, domain.AccessAzSSH) || !c.Supports(m, domain.AccessAzBastionSSH) || c.Supports(m, domain.AccessSSH) {
		t.Error("supports mismatch")
	}
	m.Provider = domain.ProviderAWS
	if c.Supports(m, domain.AccessAzSSH) {
		t.Error("supports non-azure machine")
	}
	if _, err := c.RunInteractive(context.Background(), LoginCommand()); err != nil || len(r.calls) != 1 || r.calls[0].exe != "az" {
		t.Errorf("run: %v %v", err, r.calls)
	}
	r.interactive = errors.New("fail")
	if _, err := c.RunInteractive(context.Background(), LoginCommand()); err == nil {
		t.Error("expected error")
	}
	if res, err := NewConnector(nil).RunInteractive(context.Background(), ports.CommandSpec{Executable: "true"}); err != nil || res.ExitCode != 0 {
		t.Errorf("os runner: %+v %v", res, err)
	}
	res, err := NewConnector(nil).RunInteractive(context.Background(), ports.CommandSpec{Executable: "false"})
	if err == nil || res.ExitCode != 1 {
		t.Errorf("exit code: %+v %v", res, err)
	}
}

func TestOSRunnerOutput(t *testing.T) {
	out, err := osRunner{}.RunOutput(context.Background(), "echo", []string{"hi"})
	if err != nil || strings.TrimSpace(string(out)) != "hi" {
		t.Errorf("out=%q err=%v", out, err)
	}
	if orDefault(nil) == nil {
		t.Error("nil runner not defaulted")
	}
}

type fakeDialer struct {
	addr string
	err  error
}

func (d *fakeDialer) DialContext(_ context.Context, network, address string) error {
	d.addr = network + " " + address
	return d.err
}

func TestCheckerSSH(t *testing.T) {
	m := azMachine()
	m.Address, m.State = "20.1.2.3", "VM running"
	d := &fakeDialer{}
	c := NewChecker(d, time.Hour)
	if !c.Supports(m, domain.AccessAzSSH) || c.Supports(m, domain.AccessSSH) {
		t.Error("supports mismatch")
	}
	obs := c.Check(context.Background(), m, domain.AccessAzSSH)
	if obs.Status != domain.HealthUp || obs.Label != "tcp/22 open" || d.addr != "tcp 20.1.2.3:22" || obs.Method != domain.AccessAzSSH {
		t.Errorf("up: %+v dial=%s", obs, d.addr)
	}
	d.err = errors.New("refused")
	obs = c.Check(context.Background(), m, domain.AccessAzSSH)
	if obs.Status != domain.HealthDown || obs.Error != "refused" {
		t.Errorf("down: %+v", obs)
	}
	if c.timeout != 3*time.Second {
		t.Errorf("timeout = %v", c.timeout)
	}
}

func TestCheckerSSHNoPublicIP(t *testing.T) {
	d := &fakeDialer{}
	c := NewChecker(d, 0)
	for _, tc := range []struct {
		addr, state string
		want        domain.HealthStatus
		label       string
	}{
		{"10.0.2.4", "VM running", domain.HealthUnknown, "no public IP; power state VM running"},
		{"10.5.0.4", "VM deallocated", domain.HealthDown, "no public IP; power state VM deallocated"},
		{"", "", domain.HealthUnknown, "no public IP; power state unknown"},
	} {
		m := azMachine()
		m.Address, m.State = tc.addr, tc.state
		obs := c.Check(context.Background(), m, domain.AccessAzSSH)
		if obs.Status != tc.want || obs.Label != tc.label {
			t.Errorf("%+v -> %+v", tc, obs)
		}
	}
	if d.addr != "" {
		t.Errorf("dialed %s", d.addr)
	}
}

func TestCheckerBastion(t *testing.T) {
	c := NewChecker(nil, 0)
	m := azMachine()
	m.State = "VM running"
	if obs := c.Check(context.Background(), m, domain.AccessAzBastionSSH); obs.Status != domain.HealthUp || obs.Label != "VM running, bastion bastion-app" {
		t.Errorf("up: %+v", obs)
	}
	m.State = "VM deallocated"
	if obs := c.Check(context.Background(), m, domain.AccessAzBastionSSH); obs.Status != domain.HealthDown || obs.Label != "power state VM deallocated, bastion bastion-app" {
		t.Errorf("deallocated: %+v", obs)
	}
	m.State = ""
	if obs := c.Check(context.Background(), m, domain.AccessAzBastionSSH); obs.Status != domain.HealthDown || !strings.Contains(obs.Label, "power state unknown") {
		t.Errorf("unknown state: %+v", obs)
	}
	m.Metadata = nil
	if obs := c.Check(context.Background(), m, domain.AccessAzBastionSSH); obs.Status != domain.HealthDown || obs.Error == "" {
		t.Errorf("no bastion: %+v", obs)
	}
	if obs := c.Check(context.Background(), m, domain.AccessSSH); obs.Status != domain.HealthUnknown || obs.Error == "" {
		t.Errorf("unsupported: %+v", obs)
	}
}

func TestNetDialerRefused(t *testing.T) {
	if err := (netDialer{timeout: time.Second}).DialContext(context.Background(), "tcp", "127.0.0.1:1"); err == nil {
		t.Error("expected refusal")
	}
}

func TestNetDialerSuccess(t *testing.T) {
	ln, err := newListener()
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	if err := (netDialer{timeout: time.Second}).DialContext(context.Background(), "tcp", ln.Addr().String()); err != nil {
		t.Error(err)
	}
}
