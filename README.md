# charm-devtools

> [Leia em Português](README.pt-br.md)

DevTools inspector for [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2 applications. Provides a message log, state viewer, and component tree — all inside your terminal.

## Features

- **Message Log** — ring buffer of the last 100 messages with type name and timestamp
- **State Viewer** — live `fmt.Sprintf("%+v")` dump of your current model
- **Component Tree** — automatic discovery of nested `tea.Model` fields, with support for custom `Name()` methods
- **F12 Toggle** — press F12 at any time to open/close the inspector overlay
- **Cycle Safe** — circular component references are detected and cut, so they can never overflow the stack
- **Low Overhead** — the state snapshot is recomputed on demand (while the panel is open) or every 25 messages, never on every message

## Installation

```bash
go get github.com/junhinhow/charm-devtools@latest
```

## Usage

Wrap your model with `devtools.Middleware` before passing it to `tea.NewProgram`:

```go
package main

import (
    tea "charm.land/bubbletea/v2"
    "github.com/junhinhow/charm-devtools"
)

type myModel struct {
    count int
}

func (m myModel) Init() tea.Cmd { return nil }

func (m myModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if key, ok := msg.(tea.KeyPressMsg); ok {
        if key.Code == 'q' {
            return m, tea.Quit
        }
        m.count++
    }
    return m, nil
}

func (m myModel) View() tea.View {
    return tea.NewView("Press any key to increment, q to quit, F12 for DevTools")
}

func main() {
    wrapped := devtools.Middleware(myModel{})
    p := tea.NewProgram(wrapped)
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

`devtools.Middleware(nil)` is safe: an inert model is used in place of the `nil` model instead of panicking during initialization.

## API Reference

### `devtools.Middleware(model tea.Model) tea.Model`

Wraps a Bubble Tea model with DevTools instrumentation. All messages are intercepted and recorded. Press F12 to toggle the inspector panel.

The returned model also implements `devtools.StateProvider`, which gives programmatic access to the internal tracker:

```go
wrapped := devtools.Middleware(myModel{})

state := wrapped.(devtools.StateProvider).DevState()
fmt.Println(len(state.Messages()))
```

### Inspector Navigation

| Key   | Action              |
|-------|---------------------|
| F12   | Toggle inspector    |
| Tab   | Switch panel        |
| Up/Down | Scroll content   |

`Up` scrolls towards the oldest content and `Down` towards the newest one. The scroll is always limited to the amount of content available.

### `devtools.DevState`

The internal state tracker. Accessible if you need programmatic access:

- `Messages() []MessageEntry` — returns message history (oldest first)
- `CurrentState() string` — returns the last model state snapshot
- `Components() []ComponentNode` — returns a deep copy of the component tree

The snapshot is refreshed on demand while the inspector is open, and at most once every 25 messages otherwise, so tracking state does not slow the message loop down.

### `devtools.ComponentNode`

| Field     | Description                                                          |
|-----------|----------------------------------------------------------------------|
| `Name`    | Display name: the `Name()` method when it returns a non-empty value, the type name otherwise |
| `TypeName`| Reflected type name of the model                                      |
| `Children`| Nested `tea.Model` components                                          |
| `Cycle`   | `true` when the node closes a reference cycle (the recursion is cut)  |

### Named Components

Implement `Name() string` on your models to give them descriptive names in the component tree:

```go
func (m myModel) Name() string {
    return "Counter"
}
```

A `Name()` method that returns an empty string is ignored, and the type name is used instead.

## License

MIT
