package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

// innerWidths maps the terminal totals under test to the list panel's inner
// content width, mirroring renderCockpit's split (compact keeps the full width).
var innerWidths = map[int]int{80: 80, 100: 58, 140: 83, 180: 122}

func TestAllocColumnsFillsWidthNoDeadSpace(t *testing.T) {
	m := Model{cursor: -1}
	for total, inner := range innerWidths {
		cols := allocColumns(inner)
		row := m.row(0, domain.Machine{Name: "host", Provider: domain.ProviderSSH}, cols)
		if w := lipgloss.Width(row); w > inner {
			t.Fatalf("total %d: row width %d exceeds inner %d", total, w, inner)
		}
		if gap := inner - lipgloss.Width(row); gap > 2 {
			t.Fatalf("total %d: dead space %d > 2", total, gap)
		}
	}
}

func TestAllocColumnsNameAbsorbsSurplus(t *testing.T) {
	cols := allocColumns(innerWidths[140])
	if cols.address != 0 {
		t.Fatalf("address column should be absent at 140 (name=%d, addr=%d)", cols.name, cols.address)
	}
	if cols.name < 45 {
		t.Fatalf("name column %d cannot hold a 45-char name", cols.name)
	}
	name := strings.Repeat("n", 45)
	m := Model{cursor: -1}
	row := m.row(0, domain.Machine{Name: name}, cols)
	if !strings.Contains(row, name) {
		t.Fatalf("45-char name truncated at 140: %q", row)
	}
	// A larger terminal earns the address column.
	if allocColumns(innerWidths[180]).address == 0 {
		t.Fatalf("address column should appear at 180")
	}
}

func TestListRowSelectionMarker(t *testing.T) {
	m := Model{cursor: 0}
	cols := allocColumns(innerWidths[140])
	row := m.row(0, domain.Machine{Name: "host"}, cols)
	if !strings.Contains(row, "›") {
		t.Fatalf("selected row missing cursor marker: %q", row)
	}
	other := m.row(1, domain.Machine{Name: "host"}, cols)
	if strings.Contains(other, "›") {
		t.Fatalf("unselected row has cursor marker: %q", other)
	}
}

func TestCockpitPanelsEqualHeight(t *testing.T) {
	m := NewModel(nil)
	m.width, m.height = 140, 38
	m.machines = []domain.Machine{
		{ID: "1", Name: "alpha", Provider: domain.ProviderSSH},
		{ID: "2", Name: "beta", Provider: domain.ProviderAWS},
	}
	m.recompute()

	lines := strings.Split(m.render(), "\n")
	tops, bottoms := 0, 0
	for _, line := range lines {
		if n := strings.Count(line, "╭"); n > 0 {
			tops++
			if n != 2 {
				t.Fatalf("top border row has %d corners, want 2: %q", n, line)
			}
		}
		if n := strings.Count(line, "╰"); n > 0 {
			bottoms++
			if n != 2 {
				t.Fatalf("bottom border row has %d corners, want 2: %q", n, line)
			}
		}
	}
	if tops != 1 || bottoms != 1 {
		t.Fatalf("panels not equal height: top rows=%d bottom rows=%d", tops, bottoms)
	}
}
