package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

// modelWithMachines returns a cockpit model with n visible machines so mode
// views that index the selection have something to render.
func modelWithMachines(n int) Model {
	m := NewModel(nil)
	for i := range n {
		m.machines = append(m.machines, domain.Machine{
			ID:      domain.MachineID("ssh:1:h" + string(rune('a'+i))),
			Name:    "host-" + string(rune('a'+i)),
			Address: "10.0.0." + string(rune('1'+i)),
			Methods: []domain.AccessMethod{domain.AccessSSH},
		})
	}
	m.recompute()
	return m
}

// TestHistorySurvivesVisibleEmptiedWhileOpen guards the panic where an async
// inventory/health message empties m.visible while the history view is open,
// leaving renderHistory/handleHistoryKey to index an empty slice.
func TestHistorySurvivesVisibleEmptiedWhileOpen(t *testing.T) {
	m := modelWithMachines(2)
	m.mode = modeHistory
	// An out-of-band inventory refresh returns nothing.
	m.handleMachinesMsg(machinesMsg{seq: 0, machines: nil, statuses: nil})
	if len(m.visible) != 0 {
		t.Fatalf("precondition: visible should be empty, got %d", len(m.visible))
	}
	// Neither rendering nor a keypress may panic.
	_ = m.render()
	model, _ := m.handleHistoryKey(keyPress("j"))
	if got := model.(Model); got.mode != modeCockpit {
		t.Fatalf("history key with empty visible should drop to cockpit, mode=%v", got.mode)
	}
}

// TestNoPanicsAtTinySizes drives View() and a stream of keypresses across every
// mode at degenerate terminal sizes; any slice-bounds or negative-width defect
// surfaces as a panic here.
func TestNoPanicsAtTinySizes(t *testing.T) {
	sizes := []struct{ w, h int }{
		{1, 1}, {20, 5}, {40, 10}, {79, 14}, {80, 15}, {92, 8}, {120, 3},
	}
	modes := []mode{modeCockpit, modeProfilePicker, modeHelp, modeHistory, modeEditor, modeCleanup, modeInput}
	keys := []string{"j", "k", "down", "up", "/", "f", "e", "v", "C", "?", "esc", " ", "a", "pgdown", "pgup", "tab", "enter"}

	for _, sz := range sizes {
		for _, md := range modes {
			m := modelWithMachines(3)
			m.details = true
			m.profiles = []string{"default", "prod", "staging"}
			if md == modeEditor {
				em, _ := m.openEditor()
				m = em.(Model)
			}
			if md == modeCleanup {
				m.cleanup = cleanupState{}
			}
			if md == modeInput {
				m.inputKind = "search"
			}
			m.mode = md
			model, _ := m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
			m = model.(Model)
			_ = m.View()
			for _, k := range keys {
				model, _ := m.Update(keyPress(k))
				m = model.(Model)
				_ = m.View()
			}
		}
	}
}
