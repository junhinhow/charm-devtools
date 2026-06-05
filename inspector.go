package devtools

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// inspectorModel is the Bubble Tea model that renders the DevTools panel.
type inspectorModel struct {
	state    *DevState
	scroll   int
	tab      int // 0 = messages, 1 = state, 2 = components
	tabNames []string
}

func newInspectorModel(state *DevState) inspectorModel {
	return inspectorModel{
		state:    state,
		tabNames: []string{"Messages", "State", "Components"},
	}
}

func (m inspectorModel) Init() tea.Cmd {
	return nil
}

func (m inspectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Code == tea.KeyUp:
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Code == tea.KeyDown:
			m.scroll++
		case key.Code == tea.KeyTab:
			m.tab = (m.tab + 1) % len(m.tabNames)
			m.scroll = 0
		}
	}
	return m, nil
}

func (m inspectorModel) View() tea.View {
	var b strings.Builder

	// Header
	b.WriteString("╔══════════════════════════════════════════════════════╗\n")
	b.WriteString("║            Bubble Tea DevTools (F12 close)          ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")

	// Tab bar
	b.WriteString("║ ")
	for i, name := range m.tabNames {
		if i == m.tab {
			b.WriteString(fmt.Sprintf("[%s]", name))
		} else {
			b.WriteString(fmt.Sprintf(" %s ", name))
		}
		if i < len(m.tabNames)-1 {
			b.WriteString(" | ")
		}
	}
	b.WriteString("\n")
	b.WriteString("╠══════════════════════════════════════════════════════╣\n")

	switch m.tab {
	case 0:
		m.renderMessages(&b)
	case 1:
		m.renderState(&b)
	case 2:
		m.renderComponents(&b)
	}

	b.WriteString("╠══════════════════════════════════════════════════════╣\n")
	b.WriteString("║  ↑↓ scroll  |  Tab switch panel  |  F12 close      ║\n")
	b.WriteString("╚══════════════════════════════════════════════════════╝\n")

	return tea.View{Content: b.String()}
}

func (m inspectorModel) renderMessages(b *strings.Builder) {
	messages := m.state.Messages()

	if len(messages) == 0 {
		b.WriteString("║  (no messages recorded yet)\n")
		return
	}

	// Show the most recent messages, with scroll offset.
	visibleLines := 20
	start := len(messages) - visibleLines - m.scroll
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(messages) {
		end = len(messages)
	}

	b.WriteString(fmt.Sprintf("║  Message Log (%d total, showing %d-%d)\n", len(messages), start+1, end))
	b.WriteString("║  ────────────────────────────────────────────────\n")

	for i := start; i < end; i++ {
		entry := messages[i]
		ts := entry.Timestamp.Format("15:04:05.000")
		typeName := truncate(entry.TypeName, 30)
		b.WriteString(fmt.Sprintf("║  %s  %-30s\n", ts, typeName))
	}
}

func (m inspectorModel) renderState(b *strings.Builder) {
	b.WriteString("║  Current Model State\n")
	b.WriteString("║  ────────────────────────────────────────────────\n")

	stateStr := m.state.CurrentState()
	lines := strings.Split(stateStr, "\n")

	visibleLines := 20
	start := m.scroll
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(lines) {
		end = len(lines)
	}

	for i := start; i < end; i++ {
		line := truncate(lines[i], 52)
		b.WriteString(fmt.Sprintf("║  %s\n", line))
	}
}

func (m inspectorModel) renderComponents(b *strings.Builder) {
	b.WriteString("║  Component Tree\n")
	b.WriteString("║  ────────────────────────────────────────────────\n")

	components := m.state.Components()
	for _, node := range components {
		renderNode(b, node, 0)
	}
}

func renderNode(b *strings.Builder, node ComponentNode, depth int) {
	indent := strings.Repeat("  ", depth)
	prefix := "├─"
	if depth == 0 {
		prefix = "●"
	}
	b.WriteString(fmt.Sprintf("║  %s%s %s (%s)\n", indent, prefix, node.Name, node.TypeName))
	for _, child := range node.Children {
		renderNode(b, child, depth+1)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
