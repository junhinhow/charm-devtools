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

// maxMessages defines the ring buffer capacity for message history.
const maxMessages = 100

// maxComponentDepth is the maximum depth explored while building the
// component tree. It keeps deeply nested models from recursing forever.
const maxComponentDepth = 16

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

	// Cycle is true when the node closes a reference cycle in the component
	// tree. The recursion is cut at that point.
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
func (ds *DevState) CurrentState() string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.currentState
}

// Components returns the current component tree snapshot.
func (ds *DevState) Components() []ComponentNode {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	cp := make([]ComponentNode, len(ds.components))
	copy(cp, ds.components)
	return cp
}

func (ds *DevState) record(msg tea.Msg) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	entry := MessageEntry{
		Timestamp: time.Now(),
		TypeName:  "<nil>",
		Value:     "<nil>",
	}
	// Uma mensagem nil não tem tipo: reflect.TypeOf(nil) devolve nil e
	// chamaria String() em um valor inexistente.
	if msg != nil {
		entry.TypeName = reflect.TypeOf(msg).String()
		entry.Value = fmt.Sprintf("%+v", msg)
	}

	if ds.messages == nil {
		ds.messages = make([]MessageEntry, maxMessages)
	}

	if ds.count < maxMessages {
		ds.messages[ds.head+ds.count] = entry
		ds.count++
	} else {
		ds.messages[ds.head] = entry
		ds.head = (ds.head + 1) % maxMessages
	}
}

func (ds *DevState) updateState(model tea.Model) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.currentState = fmt.Sprintf("%+v", model)
}

func (ds *DevState) updateComponents(model tea.Model) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	node := buildComponentNode(model)
	ds.components = []ComponentNode{node}
}

// buildComponentNode builds the component tree node for a model, walking
// recursively into the fields that are also tea.Model.
//
// The recursion is cut when an already visited pointer is found (a component
// cycle) or when the maximum depth is reached.
func buildComponentNode(model tea.Model) ComponentNode {
	return buildComponentNodeDepth(model, 0, map[uintptr]bool{})
}

// buildComponentNodeDepth walks the component tree keeping track of the
// visited pointers (cycle detection) and of the current depth.
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
	if named, ok := model.(interface{ Name() string }); ok {
		name = named.Name()
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
		// An already visited pointer means that the model references itself
		// (directly or indirectly): the node is flagged and the recursion
		// stops, avoiding a stack overflow.
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

// pointerOf returns the address pointed by v when its kind allows it.
//
// reflect.Value.Pointer is only valid for a few kinds; calling it on any
// other kind panics.
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

// wrappedModel is the middleware model that intercepts all messages.
type wrappedModel struct {
	inner tea.Model
	state *DevState
	show  bool
	panel inspectorModel
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
	state.updateState(model)
	state.updateComponents(model)

	return &wrappedModel{
		inner: model,
		state: state,
		show:  false,
		panel: newInspectorModel(state),
	}
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
			switch {
			case key.Code == tea.KeyUp, key.Code == tea.KeyDown,
				key.Code == tea.KeyTab:
				newPanel, cmd := w.panel.Update(msg)
				w.panel = newPanel.(inspectorModel)
				return w, cmd
			}
		}
	}

	// Forward to the inner model.
	newInner, cmd := w.inner.Update(msg)
	w.inner = newInner

	// Update tracked state after every message.
	w.state.updateState(w.inner)
	w.state.updateComponents(w.inner)

	return w, cmd
}

func (w *wrappedModel) View() tea.View {
	if w.show {
		// Update panel state before rendering.
		w.panel.state = w.state
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
