package devtools

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// simpleModel é um modelo mínimo usado nos testes.
type simpleModel struct {
	count int
}

func (m *simpleModel) Init() tea.Cmd { return nil }

func (m *simpleModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	m.count++
	return m, nil
}

func (m *simpleModel) View() tea.View { return tea.NewView("simple") }

// namedModel implementa Name() com um nome customizado.
type namedModel struct {
	simpleModel
}

func (m *namedModel) Name() string { return "Named" }

// emptyNameModel implementa Name() devolvendo uma string vazia.
type emptyNameModel struct {
	simpleModel
}

func (m *emptyNameModel) Name() string { return "" }

// parentModel possui um filho exportado, usado para inspecionar a árvore.
type parentModel struct {
	Child *simpleModel
}

func (m *parentModel) Init() tea.Cmd { return nil }

func (m *parentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m *parentModel) View() tea.View { return tea.NewView("parent") }

// cycleA e cycleB se referenciam mutuamente, formando um ciclo de componentes.
type cycleA struct {
	B *cycleB
}

func (m *cycleA) Init() tea.Cmd { return nil }

func (m *cycleA) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m *cycleA) View() tea.View { return tea.NewView("a") }

type cycleB struct {
	A *cycleA
}

func (m *cycleB) Init() tea.Cmd { return nil }

func (m *cycleB) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m *cycleB) View() tea.View { return tea.NewView("b") }

// chainModel referencia a si mesmo e é usado para testar o limite de
// profundidade da árvore de componentes.
type chainModel struct {
	Next *chainModel
}

func (m *chainModel) Init() tea.Cmd { return nil }

func (m *chainModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m *chainModel) View() tea.View { return tea.NewView("chain") }

// bigModel simula um modelo com muitos campos, como o medido na auditoria.
type bigModel struct {
	values [500]int
	label  string
}

func (m *bigModel) Init() tea.Cmd { return nil }

func (m *bigModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m *bigModel) View() tea.View { return tea.NewView("big") }

func TestMiddlewareNilNaoPanica(t *testing.T) {
	var model tea.Model

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Middleware(nil) causou panic: %v", r)
		}
	}()

	wrapped := Middleware(model)
	if wrapped == nil {
		t.Fatal("Middleware(nil) devolveu nil")
	}

	if _, cmd := wrapped.Update(tea.KeyPressMsg{Code: 'a'}); cmd != nil {
		t.Fatal("esperado comando nil do modelo inerte")
	}
	if got := wrapped.View().Content; got != "" {
		t.Fatalf("esperado view vazia do modelo inerte, veio %q", got)
	}
}

func TestRecordNilNaoPanica(t *testing.T) {
	ds := &DevState{messages: make([]MessageEntry, maxMessages)}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("record(nil) causou panic: %v", r)
		}
	}()

	ds.record(nil)

	messages := ds.Messages()
	if len(messages) != 1 {
		t.Fatalf("esperado 1 mensagem registrada, veio %d", len(messages))
	}
	if messages[0].TypeName != "<nil>" {
		t.Fatalf("tipo inesperado para mensagem nula: %q", messages[0].TypeName)
	}
	if messages[0].Value != "<nil>" {
		t.Fatalf("valor inesperado para mensagem nula: %q", messages[0].Value)
	}
}

// TestCicloDeComponentesNaoEstoura garante que modelos com referências
// circulares não derrubem o aplicativo com "fatal error: stack overflow".
func TestCicloDeComponentesNaoEstoura(t *testing.T) {
	a := &cycleA{}
	b := &cycleB{}
	a.B = b
	b.A = a

	wrapped := Middleware(a)
	if wrapped == nil {
		t.Fatal("Middleware devolveu nil")
	}

	comps := wrapped.(*wrappedModel).state.Components()
	if len(comps) != 1 {
		t.Fatalf("esperado 1 nó raiz, veio %d", len(comps))
	}
	root := comps[0]
	if len(root.Children) != 1 {
		t.Fatalf("esperado 1 filho na raiz, veio %d", len(root.Children))
	}
	child := root.Children[0]
	if len(child.Children) != 1 {
		t.Fatalf("esperado 1 neto, veio %d", len(child.Children))
	}
	cycle := child.Children[0]
	if !cycle.Cycle {
		t.Fatal("o nó que fecha o ciclo deveria estar marcado como ciclo")
	}
	if len(cycle.Children) != 0 {
		t.Fatal("o nó de ciclo não deveria ter filhos")
	}
}

func TestArvoreDeComponentesLimitaProfundidade(t *testing.T) {
	root := &chainModel{}
	current := root
	for i := 0; i < maxComponentDepth*3; i++ {
		current.Next = &chainModel{}
		current = current.Next
	}

	node := buildComponentNode(root)
	depth := 0
	for len(node.Children) > 0 {
		node = node.Children[0]
		depth++
		if depth > maxComponentDepth {
			t.Fatalf("profundidade %d maior que o limite %d", depth, maxComponentDepth)
		}
	}
	if depth != maxComponentDepth {
		t.Fatalf("profundidade %d, esperado %d", depth, maxComponentDepth)
	}
}

func TestNomeCustomizadoUsado(t *testing.T) {
	node := buildComponentNode(&namedModel{})
	if node.Name != "Named" {
		t.Fatalf("nome inesperado: %q", node.Name)
	}
}

func TestNomeCustomizadoVazioNaoSobrescreve(t *testing.T) {
	node := buildComponentNode(&emptyNameModel{})
	if node.Name == "" {
		t.Fatal("Name() vazio não pode sobrescrever o nome do nó")
	}
	if !strings.Contains(node.Name, "emptyNameModel") {
		t.Fatalf("nome inesperado: %q", node.Name)
	}
}

func TestComponentsRetornaCopiaProfunda(t *testing.T) {
	parent := &parentModel{Child: &simpleModel{}}
	wrapped := Middleware(parent).(*wrappedModel)

	comps := wrapped.state.Components()
	if len(comps) != 1 || len(comps[0].Children) != 1 {
		t.Fatalf("árvore inesperada: %+v", comps)
	}

	comps[0].Children[0].Name = "alterado"

	novos := wrapped.state.Components()
	if novos[0].Children[0].Name == "alterado" {
		t.Fatal("Components() deve devolver uma cópia profunda da árvore")
	}
}

func TestBufferCircularMantemUltimasMensagens(t *testing.T) {
	ds := &DevState{messages: make([]MessageEntry, maxMessages)}
	for i := 0; i < maxMessages+50; i++ {
		ds.record(fmt.Sprintf("msg-%03d", i))
	}

	messages := ds.Messages()
	if len(messages) != maxMessages {
		t.Fatalf("esperado %d mensagens, veio %d", maxMessages, len(messages))
	}
	if messages[0].Value != "msg-050" {
		t.Fatalf("primeira mensagem inesperada: %q", messages[0].Value)
	}
	if messages[len(messages)-1].Value != "msg-149" {
		t.Fatalf("última mensagem inesperada: %q", messages[len(messages)-1].Value)
	}
}

// TestSnapshotNaoERecalculadoAMCadaMensagem garante que o snapshot de estado
// não seja recalculado a cada mensagem (custo de formatação + reflexão).
func TestSnapshotNaoERecalculadoAMCadaMensagem(t *testing.T) {
	wrapped := Middleware(&simpleModel{}).(*wrappedModel)
	antes := wrapped.state.refreshCount()

	for i := 0; i < 100; i++ {
		wrapped.Update(fmt.Sprintf("msg-%d", i))
	}

	depois := wrapped.state.refreshCount()
	if maximo := 100/refreshInterval + 1; depois-antes > maximo {
		t.Fatalf("snapshot recalculado %d vezes em 100 mensagens (máximo %d)", depois-antes, maximo)
	}
}

// TestSnapshotAtualizadoSobDemanda garante que o estado continua atualizado
// quando o painel do inspector está visível.
func TestSnapshotAtualizadoSobDemanda(t *testing.T) {
	wrapped := Middleware(&simpleModel{}).(*wrappedModel)

	wrapped.Update("primeira")

	if !strings.Contains(wrapped.state.CurrentState(), "count:0") {
		t.Fatalf("o snapshot deveria permanecer throttled, veio %q", wrapped.state.CurrentState())
	}

	// Abre o painel: o estado passa a ser recalculado sob demanda.
	wrapped.Update(tea.KeyPressMsg{Code: tea.KeyF12})
	wrapped.View()

	if !strings.Contains(wrapped.state.CurrentState(), "count:1") {
		t.Fatalf("o estado deveria ter sido atualizado sob demanda, veio %q", wrapped.state.CurrentState())
	}
}

func TestDevStateAcessivel(t *testing.T) {
	wrapped := Middleware(&simpleModel{})

	provider, ok := wrapped.(StateProvider)
	if !ok {
		t.Fatal("o modelo devolvido por Middleware deveria implementar StateProvider")
	}
	if provider.DevState() == nil {
		t.Fatal("DevState() devolveu nil")
	}
}

func ExampleMiddleware() {
	wrapped := Middleware(&simpleModel{})
	wrapped.Update("mensagem")

	provider := wrapped.(StateProvider)
	fmt.Println(len(provider.DevState().Messages()))
	// Output: 1
}

// BenchmarkUpdateMensagens mede o custo do rastreamento de estado por
// mensagem recebida (o snapshot é throttled).
func BenchmarkUpdateMensagens(b *testing.B) {
	wrapped := Middleware(&bigModel{}).(*wrappedModel)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wrapped.Update(fmt.Sprintf("msg-%d", i))
	}
}

// BenchmarkSnapshot mede o custo de um recálculo completo do snapshot
// (formatação do estado + árvore de componentes).
func BenchmarkSnapshot(b *testing.B) {
	state := &DevState{messages: make([]MessageEntry, maxMessages)}
	model := &bigModel{}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		state.snapshot(model)
	}
}
