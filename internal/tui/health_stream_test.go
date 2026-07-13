package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func streamMachines() []domain.Machine {
	return []domain.Machine{
		{ID: "ssh:a", Name: "alpha", Provider: domain.ProviderSSH, Methods: []domain.AccessMethod{domain.AccessSSH}},
		{ID: "ssh:b", Name: "beta", Provider: domain.ProviderSSH, Methods: []domain.AccessMethod{domain.AccessSSH}},
		{ID: "ssh:c", Name: "gamma", Provider: domain.ProviderSSH, Methods: []domain.AccessMethod{domain.AccessSSH}},
	}
}

// A streamed observation resolves its own row while the rest stay in flight, so
// the list reflects partial results before the batch finishes.
func TestHealthStreamAppliesPartialResults(t *testing.T) {
	m := NewModel(nil)
	m.machines = streamMachines()
	m.recompute()
	m.inflight = map[domain.MachineID]struct{}{"ssh:a": {}, "ssh:b": {}, "ssh:c": {}}
	m.checkTotal = 3

	obs := domain.NewHealthObservation("ssh:a", domain.AccessSSH, domain.HealthUp, "tcp alpha:22", time.Millisecond, time.Now())
	next, cmd := m.handleHealthStream(healthStreamMsg{obs: obs, ch: make(chan domain.HealthObservation), ok: true})
	m = next.(Model)

	if cmd == nil {
		t.Fatalf("stream should pump the next read while machines remain in flight")
	}
	if _, ok := m.inflight["ssh:a"]; ok {
		t.Fatalf("observed machine still marked in flight")
	}
	if _, ok := m.inflight["ssh:b"]; !ok {
		t.Fatalf("pending machine cleared prematurely")
	}
	if m.statusLine != "checking 1/3" {
		t.Fatalf("status line = %q, want %q", m.statusLine, "checking 1/3")
	}

	cols := allocColumns(80)
	if got := m.row(0, m.visible[0], cols); !strings.Contains(got, "up") {
		t.Fatalf("resolved row missing result: %q", got)
	}
	if got := m.row(1, m.visible[1], cols); !strings.Contains(got, "checking") {
		t.Fatalf("pending row missing checking indicator: %q", got)
	}
}

// The final closed-channel message clears the in-flight set and status line.
func TestHealthStreamCompletionClears(t *testing.T) {
	m := NewModel(nil)
	m.machines = streamMachines()
	m.recompute()
	m.inflight = map[domain.MachineID]struct{}{"ssh:a": {}}
	m.checkTotal, m.checkDone, m.statusLine = 3, 3, "checking 3/3"

	next, cmd := m.handleHealthStream(healthStreamMsg{ok: false})
	m = next.(Model)
	if cmd != nil {
		t.Fatalf("closed stream should not pump another read")
	}
	if len(m.inflight) != 0 || m.statusLine != "" {
		t.Fatalf("completion did not clear state: inflight=%d status=%q", len(m.inflight), m.statusLine)
	}
}

func TestInflightRowIndicator(t *testing.T) {
	m := Model{cursor: -1, inflight: map[domain.MachineID]struct{}{"ssh:a": {}}}
	cols := allocColumns(80)
	row := m.row(0, domain.Machine{ID: "ssh:a", Name: "alpha"}, cols)
	if !strings.Contains(row, "…") || !strings.Contains(row, "checking") {
		t.Fatalf("in-flight row missing indicator: %q", row)
	}
	idle := m.row(0, domain.Machine{ID: "ssh:z", Name: "zeta"}, cols)
	if strings.Contains(idle, "checking") {
		t.Fatalf("idle row shows checking indicator: %q", idle)
	}
}

func TestProfilePickerScrollsAndClamps(t *testing.T) {
	m := NewModel(nil)
	m.mode = modeProfilePicker
	m.profiles = make([]string, 15)
	for i := range m.profiles {
		m.profiles[i] = fmt.Sprintf("profile-%02d", i)
	}

	m.profileCursor = 14
	view := m.renderProfilePicker()
	if strings.Contains(view, "profile-00") {
		t.Fatalf("window at bottom still shows first profile:\n%s", view)
	}
	if !strings.Contains(view, "profile-14") {
		t.Fatalf("cursor profile not shown:\n%s", view)
	}
	if !strings.Contains(view, "6-15 of 15") {
		t.Fatalf("missing position indicator when scrolled:\n%s", view)
	}

	m.profileCursor = 0
	top := m.renderProfilePicker()
	if !strings.Contains(top, "profile-00") || strings.Contains(top, "profile-14") {
		t.Fatalf("window at top wrong:\n%s", top)
	}
	if !strings.Contains(top, "1-10 of 15") {
		t.Fatalf("missing position indicator at top:\n%s", top)
	}

	// j past the end clamps to the last profile rather than overscrolling.
	for range 20 {
		next, _ := m.handleProfileKey(keyPress("j"))
		m = next.(Model)
	}
	if m.profileCursor != 14 {
		t.Fatalf("cursor overran list: %d", m.profileCursor)
	}
}
