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
		TypeName:  reflect.TypeOf(msg).String(),
		Value:     fmt.Sprintf("%+v", msg),
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

func buildComponentNode(model tea.Model) ComponentNode {
	t := reflect.TypeOf(model)
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

	// Inspect struct fields for nested tea.Model children.
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanInterface() {
				continue
			}
			if child, ok := field.Interface().(tea.Model); ok {
				node.Children = append(node.Children, buildComponentNode(child))
			}
		}
	}

	return node
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
func Middleware(model tea.Model) tea.Model {
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
