# charm-devtools

> [Read in English](README.md)

Inspector DevTools para aplicacoes [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2. Fornece log de mensagens, visualizador de estado e arvore de componentes — tudo dentro do seu terminal.

## Funcionalidades

- **Log de Mensagens** — buffer circular das ultimas 100 mensagens com tipo e timestamp
- **Visualizador de Estado** — dump ao vivo via `fmt.Sprintf("%+v")` do modelo atual
- **Arvore de Componentes** — descoberta automatica de campos `tea.Model` aninhados, com suporte a metodo `Name()` customizado
- **Toggle F12** — pressione F12 a qualquer momento para abrir/fechar o painel do inspector

## Instalacao

```bash
go get github.com/junhinhow/charm-devtools@latest
```

## Uso

Envolva seu model com `devtools.Middleware` antes de passar para `tea.NewProgram`:

```go
package main

import (
    tea "charm.land/bubbletea/v2"
    "github.com/junhinhow/charm-devtools"
)

type meuModelo struct {
    contador int
}

func (m meuModelo) Init() tea.Cmd { return nil }

func (m meuModelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if key, ok := msg.(tea.KeyPressMsg); ok {
        if key.Code == 'q' {
            return m, tea.Quit
        }
        m.contador++
    }
    return m, nil
}

func (m meuModelo) View() tea.View {
    return tea.NewView("Pressione qualquer tecla para incrementar, q para sair, F12 para DevTools")
}

func main() {
    wrapped := devtools.Middleware(meuModelo{})
    p := tea.NewProgram(wrapped)
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

## Referencia da API

### `devtools.Middleware(model tea.Model) tea.Model`

Envolve um modelo Bubble Tea com instrumentacao DevTools. Todas as mensagens sao interceptadas e registradas. Pressione F12 para alternar o painel do inspector.

### Navegacao do Inspector

| Tecla   | Acao                 |
|---------|----------------------|
| F12     | Alternar inspector   |
| Tab     | Trocar painel        |
| Cima/Baixo | Rolar conteudo  |

### `devtools.DevState`

O rastreador de estado interno. Acessivel para uso programatico:

- `Messages() []MessageEntry` — retorna historico de mensagens (mais antigo primeiro)
- `CurrentState() string` — retorna o ultimo snapshot do estado do modelo
- `Components() []ComponentNode` — retorna a arvore de componentes

### Componentes Nomeados

Implemente `Name() string` nos seus modelos para dar nomes descritivos na arvore de componentes:

```go
func (m meuModelo) Name() string {
    return "Contador"
}
```

## Licenca

MIT
