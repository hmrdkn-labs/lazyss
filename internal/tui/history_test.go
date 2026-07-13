package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func historyTestModel(t *testing.T, sessions, health int) Model {
	t.Helper()
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	m := NewModel(nil)
	machine := domain.Machine{ID: "ssh:1:prod", Name: "prod", Methods: []domain.AccessMethod{domain.AccessSSH}}
	for i := range sessions {
		machine.SessionHistory = append(machine.SessionHistory, domain.SessionEvent{
			MachineID: machine.ID, Method: domain.AccessSSH, EndedAt: now.Add(time.Duration(i) * time.Minute), Success: i%2 == 0,
		})
	}
	for i := range health {
		machine.HealthHistory = append(machine.HealthHistory, domain.HealthObservation{
			MachineID: machine.ID, Method: domain.AccessSSH, CheckedAt: now.Add(time.Duration(i) * time.Minute),
			Label: "label-" + string(rune('a'+i)),
		})
	}
	m.machines = []domain.Machine{machine}
	m.recompute()
	m.mode = modeHistory
	return m
}

func TestHistoryRendersAllStoredEntriesBeyondPreview(t *testing.T) {
	m := historyTestModel(t, 5, 5)
	got := m.render()
	if !strings.Contains(got, "Sessions") || !strings.Contains(got, "Health") {
		t.Fatalf("missing section titles: %s", got)
	}
	for i := range 5 {
		want := "label-" + string(rune('a'+i))
		if !strings.Contains(got, want) {
			t.Fatalf("health entry %q not rendered (only last 3 previewed elsewhere): %s", want, got)
		}
	}
	if !strings.Contains(got, "ok") || !strings.Contains(got, "fail") {
		t.Fatalf("session outcomes missing: %s", got)
	}
}

func TestHistoryScrollClampsAtBounds(t *testing.T) {
	m := historyTestModel(t, 20, 20)
	m.height = 8
	vp := m.historyViewport()
	total := len(historyLines(m.visible[m.cursor], 0))
	max := historyMaxOffset(total, vp)
	if max == 0 {
		t.Fatalf("expected scrollable content, total=%d vp=%d", total, vp)
	}

	for range total + 10 {
		model, _ := m.Update(keyPress("j"))
		m = model.(Model)
	}
	if m.historyOffset != max {
		t.Fatalf("scroll down clamp: offset=%d want %d", m.historyOffset, max)
	}

	for range total + 10 {
		model, _ := m.Update(keyPress("k"))
		m = model.(Model)
	}
	if m.historyOffset != 0 {
		t.Fatalf("scroll up clamp: offset=%d want 0", m.historyOffset)
	}
}

func TestHistoryEscReturnsToCockpit(t *testing.T) {
	m := historyTestModel(t, 5, 5)
	m.historyOffset = 3
	model, _ := m.Update(keyPress("esc"))
	m = model.(Model)
	if m.mode != modeCockpit {
		t.Fatalf("esc should return to cockpit, mode=%v", m.mode)
	}
	if m.historyOffset != 0 {
		t.Fatalf("esc should reset offset, got %d", m.historyOffset)
	}
}
