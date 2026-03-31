# charm-devtools

> [Leia em Portugues](README.pt-br.md)

DevTools inspector for [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2 applications. Provides a message log, state viewer, and component tree — all inside your terminal.

## Features

- **Message Log** — ring buffer of the last 100 messages with type name and timestamp
- **State Viewer** — live `fmt.Sprintf("%+v")` dump of your current model
- **Component Tree** — automatic discovery of nested `tea.Model` fields, with support for custom `Name()` methods
- **F12 Toggle** — press F12 at any time to open/close the inspector overlay

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

## API Reference

### `devtools.Middleware(model tea.Model) tea.Model`

Wraps a Bubble Tea model with DevTools instrumentation. All messages are intercepted and recorded. Press F12 to toggle the inspector panel.

### Inspector Navigation

| Key   | Action              |
|-------|---------------------|
| F12   | Toggle inspector    |
| Tab   | Switch panel        |
| Up/Down | Scroll content   |

### `devtools.DevState`

The internal state tracker. Accessible if you need programmatic access:

- `Messages() []MessageEntry` — returns message history (oldest first)
- `CurrentState() string` — returns the last model state snapshot
- `Components() []ComponentNode` — returns the component tree

### Named Components

Implement `Name() string` on your models to give them descriptive names in the component tree:

```go
func (m myModel) Name() string {
    return "Counter"
}
```

## License

MIT
