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
