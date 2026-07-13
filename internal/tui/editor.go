package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hmrdkn-labs/lazyss/internal/domain"
)

// editorField identifies which overlay field the modal cursor is on.
type editorField int

const (
	editorNote editorField = iota
	editorTags
	editorMethod
	editorFieldCount
)

// editorState is the in-progress overlay edit; the zero value means closed.
// methods carries the machine's methods plus a trailing "" sentinel for auto,
// so methodIdx selects a concrete SelectedMethod or falls back to DefaultMethod.
type editorState struct {
	machineID domain.MachineID
	note      string
	tags      string
	methods   []domain.AccessMethod
	methodIdx int
	field     editorField
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	machine := m.visible[m.cursor]
	methods := append(append([]domain.AccessMethod{}, machine.Methods...), "")
	idx := len(methods) - 1 // auto unless a concrete method is already selected
	if machine.SelectedMethod != "" {
		for i, method := range methods {
			if method == machine.SelectedMethod {
				idx = i
				break
			}
		}
	}
	m.editor = editorState{
		machineID: machine.ID,
		note:      machine.Note,
		tags:      strings.Join(machine.Tags, ", "),
		methods:   methods,
		methodIdx: idx,
	}
	m.mode = modeEditor
	return m, nil
}

// handleEditorKey drives the modal. Tab/shift-tab (and arrows) move between
// fields; on the non-text method field j/k cycle the value like the profile
// picker, while text fields accept typed input as in inputs.go.
func (m Model) handleEditorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode = modeCockpit
		m.editor = editorState{}
		return m, nil
	case "enter", "ctrl+s":
		return m.saveEditor()
	case "tab", "down":
		m.editor.field = (m.editor.field + 1) % editorFieldCount
		return m, nil
	case "shift+tab", "up":
		m.editor.field = (m.editor.field + editorFieldCount - 1) % editorFieldCount
		return m, nil
	}
	if m.editor.field == editorMethod {
		n := len(m.editor.methods)
		switch key {
		case "left", "k":
			m.editor.methodIdx = (m.editor.methodIdx + n - 1) % n
		case "right", "l", "j", "space":
			m.editor.methodIdx = (m.editor.methodIdx + 1) % n
		}
		return m, nil
	}
	m.editText(msg)
	return m, nil
}

func (m *Model) editText(msg tea.KeyPressMsg) {
	target := &m.editor.note
	if m.editor.field == editorTags {
		target = &m.editor.tags
	}
	if msg.String() == "backspace" {
		if r := []rune(*target); len(r) > 0 {
			*target = string(r[:len(r)-1])
		}
		return
	}
	*target += msg.Text // Text is populated only for printable keys (incl. space)
}

func (m Model) saveEditor() (tea.Model, tea.Cmd) {
	machine, ok := m.machineByID(m.editor.machineID)
	if !ok {
		m.mode = modeCockpit
		m.editor = editorState{}
		return m, nil
	}
	machine.Note = strings.TrimSpace(m.editor.note)
	machine.Tags = parseTagList(m.editor.tags)
	machine.SelectedMethod = m.editor.methods[m.editor.methodIdx] // "" persists as auto
	m.replaceMachine(machine)
	m.saveOverlay(machine)
	m.recompute()
	m.mode = modeCockpit
	m.editor = editorState{}
	m.statusLine = "saved " + nonempty(machine.Name, string(machine.ID))
	return m, nil
}

func (m Model) machineByID(id domain.MachineID) (domain.Machine, bool) {
	for _, machine := range m.machines {
		if machine.ID == id {
			return machine, true
		}
	}
	return domain.Machine{}, false
}

func (m Model) renderEditor() string {
	var b strings.Builder
	b.WriteString(m.titleBar() + "\n")
	var c strings.Builder
	c.WriteString(panelTitleStyle.Render("Edit overlay") + "\n")
	if machine, ok := m.machineByID(m.editor.machineID); ok {
		c.WriteString(dimStyle.Render(nonempty(machine.Name, string(machine.ID))) + "\n")
	}
	c.WriteString("\n")
	c.WriteString(m.editorFieldLine("Note", m.editor.note, editorNote) + "\n")
	c.WriteString(m.editorFieldLine("Tags", m.editor.tags, editorTags) + "\n")
	c.WriteString(m.editorFieldLine("Method", methodLabel(m.editor.methods[m.editor.methodIdx]), editorMethod))
	b.WriteString(panelActiveStyle.Width(48).Render(c.String()))
	b.WriteString("\nKeys: Tab move | type edit | j/k method | Enter/ctrl+s save | esc cancel\n")
	return b.String()
}

func (m Model) editorFieldLine(label, value string, field editorField) string {
	head := fmt.Sprintf("%-7s", label)
	if m.editor.field == field {
		if field != editorMethod {
			value += "_"
		}
		return panelTitleStyle.Render(head) + " " + value
	}
	return dimStyle.Render(head) + " " + value
}

func methodLabel(method domain.AccessMethod) string {
	if method == "" {
		return "auto"
	}
	return string(method)
}

func parseTagList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}
