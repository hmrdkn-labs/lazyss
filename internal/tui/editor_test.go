package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/hmrdkn-labs/lazyss/internal/app"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

func newEditorModel(store *fakeStateStore) Model {
	m := NewModel(&Runtime{Inventory: &app.InventoryService{Store: store}})
	m.machines = []domain.Machine{{
		ID:      "ssh:1:prod",
		Name:    "prod",
		Methods: []domain.AccessMethod{domain.AccessSSH, domain.AccessAWSSSMShell},
	}}
	m.recompute()
	return m
}

func typeText(s string) []tea.KeyPressMsg {
	out := make([]tea.KeyPressMsg, 0, len(s))
	for _, r := range s {
		out = append(out, tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
	}
	return out
}

func tabKey() tea.KeyPressMsg { return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}) }

func feed(t *testing.T, m Model, msgs ...tea.KeyPressMsg) Model {
	t.Helper()
	for _, msg := range msgs {
		model, _ := m.Update(msg)
		m = model.(Model)
	}
	return m
}

func TestEditorSaveRoundTripPersistsOverlay(t *testing.T) {
	store := &fakeStateStore{overlays: map[domain.MachineID]domain.MachineOverlay{}}
	m := newEditorModel(store)

	m = feed(t, m, keyPress("e"))
	if m.mode != modeEditor {
		t.Fatalf("expected editor mode, got %v", m.mode)
	}

	m = feed(t, m, typeText("on call")...)
	m = feed(t, m, tabKey())
	m = feed(t, m, typeText("env=prod, team=core")...)
	m = feed(t, m, tabKey())
	m = feed(t, m, keyPress("j")) // method: auto -> ssh
	m = feed(t, m, keyPress("enter"))

	if m.mode != modeCockpit {
		t.Fatalf("expected cockpit after save, got %v", m.mode)
	}
	ov := store.overlays["ssh:1:prod"]
	if ov.Note != "on call" {
		t.Fatalf("note = %q", ov.Note)
	}
	if !reflect.DeepEqual(ov.Tags, []string{"env=prod", "team=core"}) {
		t.Fatalf("tags = %v", ov.Tags)
	}
	if ov.PreferredMethod != domain.AccessSSH {
		t.Fatalf("preferred method = %q", ov.PreferredMethod)
	}
	if m.visible[0].Note != "on call" {
		t.Fatalf("visible row not refreshed: %q", m.visible[0].Note)
	}
}

func TestEditorCtrlSSavesLikeEnter(t *testing.T) {
	store := &fakeStateStore{overlays: map[domain.MachineID]domain.MachineOverlay{}}
	m := newEditorModel(store)
	m = feed(t, m, keyPress("e"))
	m = feed(t, m, typeText("note")...)
	m = feed(t, m, tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	if m.mode != modeCockpit {
		t.Fatalf("ctrl+s should save and return to cockpit, mode=%v", m.mode)
	}
	if store.overlays["ssh:1:prod"].Note != "note" {
		t.Fatalf("ctrl+s did not persist note")
	}
}

func TestEditorEscDiscards(t *testing.T) {
	store := &fakeStateStore{overlays: map[domain.MachineID]domain.MachineOverlay{}}
	m := newEditorModel(store)
	m = feed(t, m, keyPress("e"))
	m = feed(t, m, typeText("temp")...)
	m = feed(t, m, keyPress("esc"))

	if m.mode != modeCockpit {
		t.Fatalf("expected cockpit after esc, got %v", m.mode)
	}
	if _, ok := store.overlays["ssh:1:prod"]; ok {
		t.Fatalf("esc must not persist an overlay")
	}
	if m.machines[0].Note != "" || m.visible[0].Note != "" {
		t.Fatalf("esc must not mutate the machine, note=%q", m.machines[0].Note)
	}
}
