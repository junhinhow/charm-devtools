package devtools

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	// panelInnerWidth é a largura interna do painel (sem as bordas laterais).
	panelInnerWidth = 54

	// panelPadding é o espaço entre a borda lateral e o conteúdo.
	panelPadding = 2

	// contentWidth é a largura máxima do conteúdo de uma linha do painel.
	contentWidth = panelInnerWidth - panelPadding

	// visibleLines é a quantidade de linhas de conteúdo visíveis por aba.
	visibleLines = 20

	// separator é a linha divisória usada entre as seções do painel.
	separator = "────────────────────────────────────────────────────"
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
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch key.Code {
	case tea.KeyUp:
		// Rola para o conteúdo mais antigo, respeitando o limite de linhas.
		if m.scroll < m.maxScroll() {
			m.scroll++
		}
	case tea.KeyDown:
		// Rola para o conteúdo mais recente.
		if m.scroll > 0 {
			m.scroll--
		}
	case tea.KeyTab:
		m.tab = (m.tab + 1) % len(m.tabNames)
		m.scroll = 0
	}
	return m, nil
}

// maxScroll devolve o maior deslocamento de rolagem possível na aba atual.
func (m inspectorModel) maxScroll() int {
	total := m.contentCount()
	if total <= visibleLines {
		return 0
	}
	return total - visibleLines
}

// contentCount devolve a quantidade de linhas de conteúdo da aba atual.
func (m inspectorModel) contentCount() int {
	switch m.tab {
	case 0:
		return len(m.state.Messages())
	case 1:
		return len(m.stateLines())
	default:
		return len(m.componentLines())
	}
}

func (m inspectorModel) View() tea.View {
	var b strings.Builder

	// Cabeçalho.
	b.WriteString("╔" + strings.Repeat("═", panelInnerWidth) + "╗\n")
	writeBoxCentered(&b, "Bubble Tea DevTools (F12 close)")
	b.WriteString("╠" + strings.Repeat("═", panelInnerWidth) + "╣\n")

	// Barra de abas.
	writeBoxLine(&b, m.tabsLine())
	b.WriteString("╠" + strings.Repeat("═", panelInnerWidth) + "╣\n")

	// Conteúdo da aba atual.
	switch m.tab {
	case 0:
		m.renderMessages(&b)
	case 1:
		m.renderState(&b)
	default:
		m.renderComponents(&b)
	}

	// Rodapé.
	b.WriteString("╠" + strings.Repeat("═", panelInnerWidth) + "╣\n")
	writeBoxLine(&b, "↑↓ scroll  |  Tab switch panel  |  F12 close")
	b.WriteString("╚" + strings.Repeat("═", panelInnerWidth) + "╝\n")

	return tea.View{Content: b.String()}
}

// tabsLine devolve a linha com as abas do painel.
func (m inspectorModel) tabsLine() string {
	var b strings.Builder
	for i, name := range m.tabNames {
		if i == m.tab {
			b.WriteString("[" + name + "]")
		} else {
			b.WriteString(" " + name + " ")
		}
		if i < len(m.tabNames)-1 {
			b.WriteString(" | ")
		}
	}
	return b.String()
}

func (m inspectorModel) renderMessages(b *strings.Builder) {
	messages := m.state.Messages()

	if len(messages) == 0 {
		writeBoxLine(b, "Message Log (0 total)")
		writeBoxLine(b, separator)
		writeBoxLine(b, "(no messages recorded yet)")
		return
	}

	// Mostra as mensagens mais recentes, com deslocamento de rolagem.
	start, end := visibleWindow(len(messages), m.scroll)
	writeBoxLine(b, fmt.Sprintf("Message Log (%d total, showing %d-%d)", len(messages), start+1, end))
	writeBoxLine(b, separator)

	for i := start; i < end; i++ {
		entry := messages[i]
		ts := entry.Timestamp.Format("15:04:05.000")
		typeName := padRight(truncate(entry.TypeName, 30), 30)
		writeBoxLine(b, fmt.Sprintf("%s  %s", ts, typeName))
	}
}

func (m inspectorModel) renderState(b *strings.Builder) {
	writeBoxLine(b, "Current Model State")
	writeBoxLine(b, separator)

	lines := m.stateLines()
	if len(lines) == 0 {
		writeBoxLine(b, "(no state captured yet)")
		return
	}

	start, end := visibleWindow(len(lines), m.scroll)
	for i := start; i < end; i++ {
		writeBoxLine(b, lines[i])
	}
}

func (m inspectorModel) renderComponents(b *strings.Builder) {
	writeBoxLine(b, "Component Tree")
	writeBoxLine(b, separator)

	lines := m.componentLines()
	if len(lines) == 0 {
		writeBoxLine(b, "(no components detected)")
		return
	}

	start, end := visibleWindow(len(lines), m.scroll)
	for i := start; i < end; i++ {
		writeBoxLine(b, lines[i])
	}
}

// stateLines devolve o estado do modelo quebrado em linhas que cabem no painel.
func (m inspectorModel) stateLines() []string {
	stateStr := m.state.CurrentState()
	if stateStr == "" {
		return nil
	}

	var lines []string
	for _, raw := range strings.Split(stateStr, "\n") {
		for _, line := range strings.Split(ansi.Hardwrap(raw, contentWidth, false), "\n") {
			lines = append(lines, line)
		}
	}
	return lines
}

// componentLines devolve as linhas da árvore de componentes.
func (m inspectorModel) componentLines() []string {
	var lines []string
	for _, node := range m.state.Components() {
		lines = appendComponentLines(lines, node, 0)
	}
	return lines
}

// appendComponentLines adiciona à lista as linhas de um nó e dos seus filhos.
func appendComponentLines(lines []string, node ComponentNode, depth int) []string {
	indent := strings.Repeat("  ", depth)
	prefix := "├─"
	if depth == 0 {
		prefix = "●"
	}
	label := fmt.Sprintf("%s%s %s (%s)", indent, prefix, node.Name, node.TypeName)
	if node.Cycle {
		label += " [ciclo]"
	}
	lines = append(lines, truncate(label, contentWidth))

	for _, child := range node.Children {
		lines = appendComponentLines(lines, child, depth+1)
	}
	return lines
}

// visibleWindow devolve a janela visível do conteúdo, ancorada no fim.
//
// O deslocamento é limitado ao tamanho do conteúdo, de forma que a rolagem
// nunca ultrapasse a quantidade de linhas disponíveis.
func visibleWindow(total, scroll int) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if scroll < 0 {
		scroll = 0
	}
	if max := total - visibleLines; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}

	end = total - scroll
	start = end - visibleLines
	if start < 0 {
		start = 0
	}
	return start, end
}

// writeBoxLine escreve uma linha do painel, fechando a borda direita.
func writeBoxLine(b *strings.Builder, content string) {
	b.WriteString("║")
	b.WriteString(padRight("  "+content, panelInnerWidth))
	b.WriteString("║\n")
}

// writeBoxCentered escreve uma linha do painel com o conteúdo centralizado.
func writeBoxCentered(b *strings.Builder, content string) {
	b.WriteString("║")
	b.WriteString(center("  "+content, panelInnerWidth))
	b.WriteString("║\n")
}

// padRight preenche s com espaços até atingir a largura de exibição desejada.
func padRight(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// center centraliza s dentro de width, preenchendo com espaços.
func center(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	right := width - w - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// truncate corta o texto para a largura máxima de exibição, adicionando "..."
// quando o texto é maior que o limite.
//
// O corte é feito por largura de exibição (e não por bytes), preservando
// caracteres UTF-8 válidos e sequências ANSI.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxLen {
		return s
	}
	return ansi.Truncate(s, maxLen, "...")
}
