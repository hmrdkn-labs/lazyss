package gcloud

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

type call struct {
	exe  string
	args []string
}

type fakeRunner struct {
	out         []byte
	err         error
	calls       []call
	interactive []call
}

func (f *fakeRunner) RunOutput(_ context.Context, exe string, args []string) ([]byte, error) {
	f.calls = append(f.calls, call{exe, args})
	return f.out, f.err
}

func (f *fakeRunner) RunInteractive(_ context.Context, exe string, args []string) error {
	f.interactive = append(f.interactive, call{exe, args})
	return f.err
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func byName(ms []domain.Machine, name string) domain.Machine {
	for _, m := range ms {
		if m.Name == name {
			return m
		}
	}
	return domain.Machine{}
}

func TestListMachinesMapsInstances(t *testing.T) {
	r := &fakeRunner{out: fixture(t, "instances.json")}
	inv := NewInventory(r, "")
	if inv.ProviderName() != "gcp" {
		t.Fatal("provider name")
	}
	ms, st, err := inv.ListMachines(context.Background(), ports.InventoryQuery{})
	if err != nil || st.Status != domain.ProviderHealthy {
		t.Fatalf("err=%v status=%+v", err, st)
	}
	want := []call{{"gcloud", []string{"compute", "instances", "list", "--format=json"}}}
	if !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("argv = %v", r.calls)
	}
	if len(ms) != 3 {
		t.Fatalf("machines = %d", len(ms))
	}
	web := byName(ms, "web-1")
	if web.ID != "gcp:acme-prod/europe-west1-b/web-1" || web.Address != "34.1.2.3" ||
		web.Scope.Account != "acme-prod" || web.Scope.Region != "europe-west1-b" ||
		web.Provider != domain.ProviderGCP || web.NativeID != "web-1" ||
		web.ProviderTags["team"] != "web" || web.State != "RUNNING" {
		t.Fatalf("web = %+v", web)
	}
	if !reflect.DeepEqual(web.Methods, []domain.AccessMethod{domain.AccessGCloudSSH, domain.AccessGCloudIAPSSH}) {
		t.Fatalf("web methods = %v", web.Methods)
	}
	if web.Health.Status != domain.HealthUp || web.Health.Label != "gcp RUNNING" {
		t.Fatalf("web health = %+v", web.Health)
	}
	db := byName(ms, "db-1")
	if db.Address != "10.0.1.9" || db.Methods[0] != domain.AccessGCloudIAPSSH {
		t.Fatalf("db = %+v", db)
	}
	old := byName(ms, "old-batch")
	if old.Health.Status != domain.HealthDown || old.Health.Label != "gcp TERMINATED" || old.Address != "10.2.0.7" {
		t.Fatalf("old = %+v", old)
	}
}

func TestListMachinesProjectFlag(t *testing.T) {
	r := &fakeRunner{out: fixture(t, "instances.json")}
	ms, _, err := NewInventory(r, "other").ListMachines(context.Background(), ports.InventoryQuery{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compute", "instances", "list", "--format=json", "--project", "other"}
	if !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("argv = %v", r.calls[0].args)
	}
	if byName(ms, "web-1").ID != "gcp:other/europe-west1-b/web-1" {
		t.Fatalf("id = %s", byName(ms, "web-1").ID)
	}
}

func TestInventoryRejectsFlagShapedProject(t *testing.T) {
	r := &fakeRunner{out: fixture(t, "instances.json")}
	ms, st, err := NewInventory(r, "--evil").ListMachines(context.Background(), ports.InventoryQuery{})
	if err == nil || st.Status != domain.ProviderDegraded || ms != nil || st.Message == "" {
		t.Fatalf("ms=%v st=%+v err=%v", ms, st, err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("runner called: %v", r.calls)
	}
}

func TestListMachinesErrors(t *testing.T) {
	_, st, err := NewInventory(&fakeRunner{err: exec.ErrNotFound}, "p").ListMachines(context.Background(), ports.InventoryQuery{})
	if err == nil || st.Status != domain.ProviderDegraded || st.Message != msgNotFound {
		t.Fatalf("err=%v st=%+v", err, st)
	}
	_, st, err = NewInventory(&fakeRunner{out: []byte("not json")}, "p").ListMachines(context.Background(), ports.InventoryQuery{})
	if err == nil || st.Status != domain.ProviderDegraded {
		t.Fatalf("err=%v st=%+v", err, st)
	}
}

func TestClassifyError(t *testing.T) {
	exit := func(stderr string) error { return &exec.ExitError{Stderr: []byte(stderr)} }
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"not found", &exec.Error{Name: "gcloud", Err: exec.ErrNotFound}, msgNotFound},
		{"timeout", context.DeadlineExceeded, "gcloud timed out"},
		{"login hint", exit("ERROR: Please run:\n  gcloud auth login\n"), msgNoLogin},
		{"no account", exit("You do not currently have an active account selected."), msgNoLogin},
		{"reauth", exit("ERROR: Reauthentication failed."), msgNoLogin},
		{"no project", exit("ERROR: (gcloud) The required property [project] is not currently set."), msgNoProj},
		{"generic", exit("\nERROR: boom happened\nmore"), "ERROR: boom happened"},
		{"long", exit(strings.Repeat("x", 300)), strings.Repeat("x", 117) + "..."},
		{"plain error", errors.New("odd failure"), "odd failure"},
		{"empty stderr", exit(""), "exit status 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if tt.name == "empty stderr" {
				if got == "" {
					t.Fatal("empty message")
				}
				return
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestCLIStatus(t *testing.T) {
	ctx := context.Background()
	r := &fakeRunner{out: fixture(t, "auth_active.json")}
	st := CLIStatus(ctx, r)
	if st.Status != domain.ProviderHealthy || st.Message != "logged in as me@example.com" {
		t.Fatalf("st = %+v", st)
	}
	if !reflect.DeepEqual(r.calls[0].args, []string{"auth", "list", "--format=json"}) {
		t.Fatalf("argv = %v", r.calls[0].args)
	}
	if st := CLIStatus(ctx, &fakeRunner{out: fixture(t, "auth_none.json")}); st.Status != domain.ProviderDegraded || st.Message != msgNoLogin {
		t.Fatalf("none = %+v", st)
	}
	if st := CLIStatus(ctx, &fakeRunner{err: exec.ErrNotFound}); st.Message != msgNotFound {
		t.Fatalf("missing = %+v", st)
	}
	if st := CLIStatus(ctx, &fakeRunner{out: []byte("{")}); st.Status != domain.ProviderDegraded {
		t.Fatalf("bad json = %+v", st)
	}
}

func TestListProjects(t *testing.T) {
	r := &fakeRunner{out: fixture(t, "projects.json")}
	ps, err := ListProjects(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ps, []Project{{"acme-prod", "Acme Production"}, {"acme-dev", "Acme Dev"}}) {
		t.Fatalf("ps = %v", ps)
	}
	if !reflect.DeepEqual(r.calls[0].args, []string{"projects", "list", "--format=json"}) {
		t.Fatalf("argv = %v", r.calls[0].args)
	}
	if _, err := ListProjects(context.Background(), &fakeRunner{err: exec.ErrNotFound}); err == nil {
		t.Fatal("want error")
	}
	if _, err := ListProjects(context.Background(), &fakeRunner{out: []byte("x")}); err == nil {
		t.Fatal("want parse error")
	}
}

func TestActiveProject(t *testing.T) {
	ctx := context.Background()
	r := &fakeRunner{out: []byte("\"acme-prod\"\n")}
	p, err := ActiveProject(ctx, r)
	if err != nil || p != "acme-prod" {
		t.Fatalf("p=%q err=%v", p, err)
	}
	if !reflect.DeepEqual(r.calls[0].args, []string{"config", "get-value", "project", "--format=json"}) {
		t.Fatalf("argv = %v", r.calls[0].args)
	}
	for _, out := range []string{"", "\n"} {
		if p, err := ActiveProject(ctx, &fakeRunner{out: []byte(out)}); err != nil || p != "" {
			t.Fatalf("unset p=%q err=%v", p, err)
		}
	}
	if _, err := ActiveProject(ctx, &fakeRunner{err: exec.ErrNotFound}); err == nil {
		t.Fatal("want error")
	}
	if _, err := ActiveProject(ctx, &fakeRunner{out: []byte("{")}); err == nil {
		t.Fatal("want parse error")
	}
}

func TestLoginCommand(t *testing.T) {
	if got := LoginCommand().Argv(); !reflect.DeepEqual(got, []string{"gcloud", "auth", "login"}) {
		t.Fatalf("argv = %v", got)
	}
}

func gcpMachine() domain.Machine {
	return domain.Machine{
		ID: "gcp:p/z/vm", Name: "vm", NativeID: "vm", Provider: domain.ProviderGCP,
		Scope: domain.Scope{Account: "p", Region: "z"}, State: "RUNNING",
		Metadata: map[string]string{metaExternalIP: "34.1.2.3"},
	}
}

func TestConnectorBuildCommandGolden(t *testing.T) {
	c := NewConnector(nil)
	tests := []struct {
		method domain.AccessMethod
		want   []string
	}{
		{domain.AccessGCloudSSH, []string{"gcloud", "compute", "ssh", "vm", "--zone", "z", "--project", "p"}},
		{domain.AccessGCloudIAPSSH, []string{"gcloud", "compute", "ssh", "vm", "--zone", "z", "--project", "p", "--tunnel-through-iap"}},
	}
	for _, tt := range tests {
		spec, err := c.BuildCommand(gcpMachine(), tt.method, ports.ConnectOptions{})
		if err != nil || !reflect.DeepEqual(spec.Argv(), tt.want) {
			t.Fatalf("%s: argv=%v err=%v", tt.method, spec.Argv(), err)
		}
	}
}

func TestConnectorRejects(t *testing.T) {
	c := NewConnector(nil)
	mut := map[string]func(*domain.Machine){
		"name":    func(m *domain.Machine) { m.NativeID = "--evil" },
		"zone":    func(m *domain.Machine) { m.Scope.Region = "-z" },
		"project": func(m *domain.Machine) { m.Scope.Account = "-p" },
		"empty":   func(m *domain.Machine) { m.Scope.Account = "" },
	}
	for name, f := range mut {
		m := gcpMachine()
		f(&m)
		if _, err := c.BuildCommand(m, domain.AccessGCloudSSH, ports.ConnectOptions{}); err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
	if _, err := c.BuildCommand(gcpMachine(), domain.AccessSSH, ports.ConnectOptions{}); err == nil {
		t.Fatal("unsupported method accepted")
	}
	m := gcpMachine()
	m.NativeID = ""
	m.Name = "named"
	spec, err := c.BuildCommand(m, domain.AccessGCloudSSH, ports.ConnectOptions{})
	if err != nil || spec.Args[2] != "named" {
		t.Fatalf("fallback name: %v %v", spec, err)
	}
}

func TestConnectorSupportsAndRun(t *testing.T) {
	r := &fakeRunner{}
	c := NewConnector(r)
	if !c.Supports(gcpMachine(), domain.AccessGCloudIAPSSH) || c.Supports(gcpMachine(), domain.AccessSSH) ||
		c.Supports(domain.Machine{Provider: domain.ProviderAWS}, domain.AccessGCloudSSH) {
		t.Fatal("Supports")
	}
	if _, err := c.RunInteractive(context.Background(), LoginCommand()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.interactive, []call{{"gcloud", []string{"auth", "login"}}}) {
		t.Fatalf("interactive = %v", r.interactive)
	}
}

type fakeDialer struct {
	err      error
	addr     string
	deadline time.Time
	hasDL    bool
}

func (f *fakeDialer) DialContext(ctx context.Context, _ string, address string) error {
	f.addr = address
	f.deadline, f.hasDL = ctx.Deadline()
	return f.err
}

func TestChecker(t *testing.T) {
	ctx := context.Background()
	d := &fakeDialer{}
	c := NewChecker(d, time.Second)
	if !c.Supports(gcpMachine(), domain.AccessGCloudSSH) || c.Supports(gcpMachine(), domain.AccessSSH) {
		t.Fatal("Supports")
	}
	obs := c.Check(ctx, gcpMachine(), domain.AccessGCloudSSH)
	if obs.Status != domain.HealthUp || obs.Label != "tcp/22 open" || d.addr != "34.1.2.3:22" {
		t.Fatalf("open = %+v addr=%s", obs, d.addr)
	}
	if !d.hasDL || time.Until(d.deadline) > time.Second {
		t.Fatalf("dialer deadline = %v (has=%v), want <= 1s", d.deadline, d.hasDL)
	}
	d.err = errors.New("boom")
	obs = c.Check(ctx, gcpMachine(), domain.AccessGCloudSSH)
	if obs.Status != domain.HealthDown || obs.Error != "boom" || obs.Label != "tcp/22 probe failed" {
		t.Fatalf("failed = %+v", obs)
	}
	for _, e := range []error{context.DeadlineExceeded, context.Canceled, syscall.ENETUNREACH} {
		d.err = e
		if obs := c.Check(ctx, gcpMachine(), domain.AccessGCloudSSH); obs.Label != "tcp/22 probe failed" {
			t.Fatalf("%v label = %q", e, obs.Label)
		}
	}
	d.err = &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	if obs := c.Check(ctx, gcpMachine(), domain.AccessGCloudSSH); obs.Label != "tcp/22 refused" {
		t.Fatalf("refused label = %q", obs.Label)
	}
	noIP := gcpMachine()
	noIP.Metadata = nil
	if obs := c.Check(ctx, noIP, domain.AccessGCloudSSH); obs.Status != domain.HealthUnknown || obs.Label != "no external IP" {
		t.Fatalf("no ip = %+v", obs)
	}
	if obs := c.Check(ctx, gcpMachine(), domain.AccessGCloudIAPSSH); obs.Status != domain.HealthUp || obs.Label != "gcp RUNNING (IAP not probed)" {
		t.Fatalf("iap running = %+v", obs)
	}
	stopped := gcpMachine()
	stopped.State = "TERMINATED"
	if obs := c.Check(ctx, stopped, domain.AccessGCloudIAPSSH); obs.Status != domain.HealthDown || obs.Label != "gcp TERMINATED (IAP not probed)" {
		t.Fatalf("iap stopped = %+v", obs)
	}
	if obs := c.Check(ctx, gcpMachine(), domain.AccessSSH); obs.Status != domain.HealthUnknown {
		t.Fatalf("unsupported = %+v", obs)
	}
}

func TestDefaultsAndHelpers(t *testing.T) {
	if c := NewChecker(nil, time.Minute); c.timeout != 3*time.Second || c.dialer == nil {
		t.Fatalf("checker = %+v", c)
	}
	if _, ok := orDefault(nil).(osRunner); !ok {
		t.Fatal("nil runner should default to osRunner")
	}
	if projectFromSelfLink("nothing") != "" || projectFromSelfLink("x/projects/abc") != "abc" {
		t.Fatal("projectFromSelfLink")
	}
	if _, label := providerHealth(""); label != "gcp unknown" {
		t.Fatal(label)
	}
	if got := lastSegment("plain"); got != "plain" {
		t.Fatal(got)
	}
}

func TestOSRunnerAndNetDialer(t *testing.T) {
	out, err := osRunner{}.RunOutput(context.Background(), "echo", []string{"hi"})
	if err != nil || strings.TrimSpace(string(out)) != "hi" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if err := (osRunner{}).RunInteractive(context.Background(), "true", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (netDialer{timeout: time.Second}).DialContext(ctx, "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("want dial error")
	}
}
