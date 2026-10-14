// Package devtools provides a DevTools inspector middleware for Bubble Tea v2 applications.
// It intercepts messages, tracks state changes, and maintains a message history ring buffer.
package devtools

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	// maxMessages define a capacidade do buffer circular do log de mensagens.
	maxMessages = 100

	// maxComponentDepth é a profundidade máxima percorrida ao montar a
	// árvore de componentes. Ela impede que modelos profundamente aninhados
	// (ou com referências circulares) gerem recursão sem fim.
	maxComponentDepth = 16

	// refreshInterval é a quantidade de mensagens que pode passar entre duas
	// atualizações automáticas do snapshot de estado/árvore de componentes.
	// O snapshot também é recalculado sob demanda quando o painel do
	// inspector está visível.
	refreshInterval = 25
)

// MessageEntry represents a single intercepted message with metadata.
type MessageEntry struct {
	Timestamp time.Time
	TypeName  string
	Value     string
}

// ComponentNode represents a named model in the component tree.
type ComponentNode struct {
	Name     string
	TypeName string
	Children []ComponentNode

	// Cycle indica que o nó referencia um modelo já visitado na árvore
	// (ciclo de componentes). A recursão é interrompida nesse ponto.
	Cycle bool
}

// DevState holds the internal state tracked by the middleware.
type DevState struct {
	mu           sync.RWMutex
	messages     []MessageEntry
	head         int
	count        int
	currentState string
	components   []ComponentNode

	// model guarda o último modelo observado pelo middleware e dirty indica
	// que o snapshot ainda precisa ser recalculado.
	model tea.Model
	dirty bool

	// pending conta as mensagens recebidas desde a última atualização
	// automática do snapshot.
	pending int

	// refreshes conta quantas vezes o snapshot foi recalculado. É usado para
	// verificar o efeito do throttling e em testes.
	refreshes int
}

// Messages returns a copy of the message history, oldest first.
func (ds *DevState) Messages() []MessageEntry {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	result := make([]MessageEntry, 0, ds.count)
	for i := 0; i < ds.count; i++ {
		idx := (ds.head + i) % maxMessages
		result = append(result, ds.messages[idx])
	}
	return result
}

// CurrentState returns the last captured model state string.
//
// O snapshot é atualizado de forma throttled (a cada refreshInterval
// mensagens) e sob demanda, quando o painel do inspector está visível.
func (ds *DevState) CurrentState() string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.currentState
}

// Components returns the current component tree snapshot.
//
// A árvore devolvida é uma cópia profunda: alterar os nós retornados não
// afeta o estado interno.
func (ds *DevState) Components() []ComponentNode {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return cloneNodes(ds.components)
}

// cloneNodes copia a árvore de componentes sem compartilhar as fatias de
// filhos com o estado interno.
func cloneNodes(nodes []ComponentNode) []ComponentNode {
	clone := make([]ComponentNode, len(nodes))
	for i, node := range nodes {
		clone[i] = node
		clone[i].Children = cloneNodes(node.Children)
	}
	return clone
}

// record adiciona uma mensagem ao buffer circular. Mensagens nulas são
// registradas com o marcador "<nil>" em vez de causar panic.
func (ds *DevState) record(msg tea.Msg) {
	entry := MessageEntry{
		Timestamp: time.Now(),
		TypeName:  "<nil>",
		Value:     "<nil>",
	}
	if msg != nil {
		entry.TypeName = reflect.TypeOf(msg).String()
		entry.Value = fmt.Sprintf("%+v", msg)
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.messages == nil {
		ds.messages = make([]MessageEntry, maxMessages)
	}

	if ds.count < maxMessages {
		ds.messages[ds.head+ds.count] = entry
		ds.count++
		return
	}
	ds.messages[ds.head] = entry
	ds.head = (ds.head + 1) % maxMessages
}

// observe registra o modelo atual e agenda o recálculo do snapshot.
//
// O recálculo é limitado a uma atualização a cada refreshInterval mensagens
// para não penalizar o laço de eventos do aplicativo.
func (ds *DevState) observe(model tea.Model) {
	ds.mu.Lock()
	ds.model = model
	ds.dirty = true
	ds.pending++
	need := ds.pending >= refreshInterval
	if need {
		ds.pending = 0
	}
	ds.mu.Unlock()

	if need {
		ds.refresh()
	}
}

// refresh recalcula o snapshot de estado e a árvore de componentes quando há
// mudanças pendentes.
func (ds *DevState) refresh() {
	ds.mu.RLock()
	model, dirty := ds.model, ds.dirty
	ds.mu.RUnlock()

	if !dirty || model == nil {
		return
	}
	ds.snapshot(model)
}

// snapshot recalcula e armazena o estado e a árvore de componentes do modelo.
func (ds *DevState) snapshot(model tea.Model) {
	state := "<nil>"
	var components []ComponentNode
	if model != nil {
		state = fmt.Sprintf("%+v", model)
		components = []ComponentNode{buildComponentNode(model)}
	}

	ds.mu.Lock()
	ds.currentState = state
	ds.components = components
	ds.model = model
	ds.dirty = false
	ds.refreshes++
	ds.mu.Unlock()
}

// refreshCount devolve quantas vezes o snapshot foi recalculado.
func (ds *DevState) refreshCount() int {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.refreshes
}

// buildComponentNode monta o nó da árvore de componentes para o modelo,
// percorrendo recursivamente os campos que também são tea.Model.
//
// A recursão é interrompida quando um ponteiro já visitado é encontrado
// (ciclo de componentes) ou quando a profundidade máxima é atingida.
func buildComponentNode(model tea.Model) ComponentNode {
	return buildComponentNodeDepth(model, 0, map[uintptr]bool{})
}

// buildComponentNodeDepth percorre a árvore de componentes com controle de
// ciclos (visited) e de profundidade (depth).
func buildComponentNodeDepth(model tea.Model, depth int, visited map[uintptr]bool) ComponentNode {
	t := reflect.TypeOf(model)
	if t == nil {
		return ComponentNode{Name: "<nil>", TypeName: "<nil>"}
	}

	name := t.String()
	if t.Kind() == reflect.Ptr {
		name = t.Elem().String()
	}

	// Check if the model implements a Name() method for custom naming.
	// Um nome vazio não pode sobrescrever o nome derivado do tipo.
	if named, ok := model.(interface{ Name() string }); ok {
		if custom := named.Name(); custom != "" {
			name = custom
		}
	}

	node := ComponentNode{
		Name:     name,
		TypeName: t.String(),
	}

	if depth >= maxComponentDepth {
		return node
	}

	// Inspect struct fields for nested tea.Model children.
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return node
		}
		// Um ponteiro já visitado indica que o modelo se referencia
		// (diretamente ou indiretamente): o nó é marcado e a recursão para.
		if ptr, ok := pointerOf(v); ok {
			if visited[ptr] {
				node.Cycle = true
				return node
			}
			visited[ptr] = true
			defer delete(visited, ptr)
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return node
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanInterface() {
			continue
		}
		child, ok := field.Interface().(tea.Model)
		if !ok || child == nil {
			continue
		}
		node.Children = append(node.Children, buildComponentNodeDepth(child, depth+1, visited))
	}

	return node
}

// pointerOf devolve o endereço apontado por v quando o seu kind permite.
//
// reflect.Value.Pointer só é válido para os kinds listados abaixo; chamá-lo
// em outros kinds causa panic.
func pointerOf(v reflect.Value) (uintptr, bool) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func,
		reflect.UnsafePointer, reflect.Chan:
		if v.IsNil() {
			return 0, false
		}
		return v.Pointer(), true
	default:
		return 0, false
	}
}

// StateProvider is implemented by the model returned by Middleware. It allows
// programmatic access to the internal DevState.
type StateProvider interface {
	DevState() *DevState
}

// inspectorPanel is the interface of the inspector panel. Keeping it as an
// interface avoids unsafe type assertions on the panel returned by Update.
type inspectorPanel interface {
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() tea.View
}

// wrappedModel is the middleware model that intercepts all messages.
type wrappedModel struct {
	inner tea.Model
	state *DevState
	show  bool
	panel inspectorPanel
}

// Middleware wraps a tea.Model with DevTools instrumentation.
// The returned model intercepts all messages and tracks state.
// Press F12 to toggle the inspector panel overlay.
//
// A nil model is accepted: an inert model is used in its place so that the
// application does not panic during initialization.
func Middleware(model tea.Model) tea.Model {
	if model == nil {
		model = inertModel{}
	}

	state := &DevState{
		messages: make([]MessageEntry, maxMessages),
	}
	state.snapshot(model)

	return &wrappedModel{
		inner: model,
		state: state,
		show:  false,
		panel: newInspectorModel(state),
	}
}

// DevState returns the state tracker used by the middleware.
func (w *wrappedModel) DevState() *DevState {
	return w.state
}

func (w *wrappedModel) Init() tea.Cmd {
	return w.inner.Init()
}

func (w *wrappedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Record every message in the ring buffer.
	w.state.record(msg)

	// Toggle inspector with F12.
	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.Code == tea.KeyF12 {
			w.show = !w.show
			return w, nil
		}
	}

	// If inspector is visible, let it handle navigation keys.
	if w.show {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.Code {
			case tea.KeyUp, tea.KeyDown, tea.KeyTab:
				newPanel, cmd := w.panel.Update(msg)
				// A asserção precisa ser segura: um painel que devolva um
				// modelo inesperado não pode derrubar o aplicativo.
				if panel, ok := newPanel.(inspectorPanel); ok {
					w.panel = panel
				}
				return w, cmd
			}
		}
	}

	// Forward to the inner model.
	newInner, cmd := w.inner.Update(msg)
	if newInner != nil {
		w.inner = newInner
	}

	// The snapshot is refreshed on demand (when the panel is visible) or
	// every refreshInterval messages, instead of on every message.
	w.state.observe(w.inner)

	return w, cmd
}

func (w *wrappedModel) View() tea.View {
	if w.show {
		// Update panel state before rendering: with the panel visible the
		// snapshot is refreshed on demand, so it always shows the newest state.
		w.state.refresh()
		return w.panel.View()
	}
	return w.inner.View()
}

// inertModel is an empty model used when Middleware receives nil. It keeps
// the application alive instead of panicking during initialization.
type inertModel struct{}

func (inertModel) Init() tea.Cmd { return nil }

func (m inertModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (inertModel) View() tea.View { return tea.NewView("") }
