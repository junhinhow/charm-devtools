package devtools

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// newStateWithMessages cria um DevState com n mensagens registradas.
func newStateWithMessages(n int) *DevState {
	ds := &DevState{messages: make([]MessageEntry, maxMessages)}
	for i := 0; i < n; i++ {
		ds.record(fmt.Sprintf("msg-%02d", i))
	}
	return ds
}

// showingRange extrai o intervalo "showing X-Y" da aba de mensagens.
func showingRange(t *testing.T, view tea.View) (int, int) {
	t.Helper()

	match := showingRe.FindStringSubmatch(view.Content)
	if match == nil {
		t.Fatal("nenhuma linha com o intervalo exibido")
	}
	start, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatalf("início de intervalo inválido: %q", match[1])
	}
	end, err := strconv.Atoi(match[2])
	if err != nil {
		t.Fatalf("fim de intervalo inválido: %q", match[2])
	}
	return start, end
}

// showingRe captura o intervalo de mensagens exibido no painel.
var showingRe = regexp.MustCompile(`showing (\d+)-(\d+)`)

// TestScrollParaCimaMostraMensagensMaisAntigas verifica que a rolagem não
// está invertida: "cima" mostra mensagens mais antigas.
func TestScrollParaCimaMostraMensagensMaisAntigas(t *testing.T) {
	panel := newInspectorModel(newStateWithMessages(30))

	start, end := showingRange(t, panel.View())
	if start != 11 || end != 30 {
		t.Fatalf("janela inicial incorreta: %d-%d (esperado 11-30)", start, end)
	}

	updated, _ := panel.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	panel = updated.(inspectorModel)

	start, end = showingRange(t, panel.View())
	if start != 10 || end != 29 {
		t.Fatalf("apertar cima deveria mostrar mensagens mais antigas: %d-%d (esperado 10-29)", start, end)
	}

	updated, _ = panel.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	panel = updated.(inspectorModel)

	start, end = showingRange(t, panel.View())
	if start != 11 || end != 30 {
		t.Fatalf("apertar baixo deveria voltar às mensagens mais recentes: %d-%d (esperado 11-30)", start, end)
	}
}

// TestScrollLimitadoAoNumeroDeMensagens verifica que a rolagem nunca ultrapassa
// a quantidade de mensagens disponíveis.
func TestScrollLimitadoAoNumeroDeMensagens(t *testing.T) {
	panel := newInspectorModel(newStateWithMessages(30))

	for i := 0; i < 50; i++ {
		updated, _ := panel.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		panel = updated.(inspectorModel)
	}

	if panel.scroll != 10 {
		t.Fatalf("scroll deveria estar limitado a 10, veio %d", panel.scroll)
	}

	start, end := showingRange(t, panel.View())
	if start != 1 || end != 20 {
		t.Fatalf("janela incorreta no limite: %d-%d (esperado 1-20)", start, end)
	}

	updated, _ := panel.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	panel = updated.(inspectorModel)

	start, end = showingRange(t, panel.View())
	if start != 2 || end != 21 {
		t.Fatalf("janela incorreta após voltar uma linha: %d-%d (esperado 2-21)", start, end)
	}
}

// TestPainelFechaLinhasComBordaDireita garante que todas as linhas do painel
// (incluindo o corpo) terminam com a borda direita.
func TestPainelFechaLinhasComBordaDireita(t *testing.T) {
	panel := newInspectorModel(newStateWithMessages(3))

	for _, tab := range []int{0, 1, 2} {
		panel.tab = tab

		lines := strings.Split(strings.TrimRight(panel.View().Content, "\n"), "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "║") && !strings.HasPrefix(line, "╔") &&
				!strings.HasPrefix(line, "╠") && !strings.HasPrefix(line, "╚") {
				t.Fatalf("aba %d, linha %d não começa com borda: %q", tab, i, line)
			}
			if !strings.HasSuffix(line, "║") && !strings.HasSuffix(line, "╗") &&
				!strings.HasSuffix(line, "╣") && !strings.HasSuffix(line, "╝") {
				t.Fatalf("aba %d, linha %d não fecha com borda: %q", tab, i, line)
			}
			if w := ansi.StringWidth(line); w != panelInnerWidth+2 {
				t.Fatalf("aba %d, linha %d com largura %d (esperado %d): %q", tab, i, w, panelInnerWidth+2, line)
			}
		}
	}
}

// TestTruncateNaoQuebraUTF8 garante que a truncagem não corta caracteres
// multibyte pela metade.
func TestTruncateNaoQuebraUTF8(t *testing.T) {
	casos := []struct {
		in  string
		max int
	}{
		{"ããããã", 4},
		{"maçãçãobão", 5},
		{"日本語のテキスト", 5},
		{"áéíóú", 6},
	}

	for _, c := range casos {
		got := truncate(c.in, c.max)
		if !utf8.ValidString(got) {
			t.Fatalf("truncate(%q, %d) devolveu UTF-8 inválido: %q", c.in, c.max, got)
		}
		if w := ansi.StringWidth(got); w > c.max {
			t.Fatalf("truncate(%q, %d) devolveu largura %d: %q", c.in, c.max, w, got)
		}
	}
}

func TestTruncatePreservaTextoCurto(t *testing.T) {
	if got := truncate("ok", 10); got != "ok" {
		t.Fatalf("texto curto alterado: %q", got)
	}
	if got := truncate("ok", 0); got != "" {
		t.Fatalf("esperado string vazia, veio %q", got)
	}
}

func TestTrocaDeAbaZeraARolagem(t *testing.T) {
	panel := newInspectorModel(newStateWithMessages(30))

	updated, _ := panel.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	panel = updated.(inspectorModel)
	if panel.scroll == 0 {
		t.Fatal("a rolagem deveria ter avançado")
	}

	updated, _ = panel.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	panel = updated.(inspectorModel)
	if panel.scroll != 0 {
		t.Fatalf("a troca de aba deveria zerar a rolagem, veio %d", panel.scroll)
	}
	if panel.tab != 1 {
		t.Fatalf("aba inesperada: %d", panel.tab)
	}
}
