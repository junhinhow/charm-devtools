package devtools

import (
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

// chainModel referencia a si mesmo e e usado para testar o limite de
// profundidade da arvore de componentes.
type chainModel struct {
	Next *chainModel
}

func (m *chainModel) Init() tea.Cmd { return nil }

func (m *chainModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m *chainModel) View() tea.View { return tea.NewView("chain") }

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

// TestCicloDeComponentesNaoEstoura garante que modelos com referencias
// circulares nao derrubem o aplicativo com "fatal error: stack overflow".
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
		t.Fatalf("esperado 1 no raiz, veio %d", len(comps))
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
		t.Fatal("o no que fecha o ciclo deveria estar marcado como ciclo")
	}
	if len(cycle.Children) != 0 {
		t.Fatal("o no de ciclo nao deveria ter filhos")
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
