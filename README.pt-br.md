# charm-devtools

> [Read in English](README.md)

Inspector DevTools para aplicações [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2. Fornece log de mensagens, visualizador de estado e árvore de componentes — tudo dentro do seu terminal.

## Funcionalidades

- **Log de Mensagens** — buffer circular das últimas 100 mensagens com tipo e timestamp
- **Visualizador de Estado** — dump ao vivo via `fmt.Sprintf("%+v")` do modelo atual
- **Árvore de Componentes** — descoberta automática de campos `tea.Model` aninhados, com suporte a método `Name()` customizado
- **Toggle F12** — pressione F12 a qualquer momento para abrir/fechar o painel do inspector
- **Seguro contra ciclos** — referências circulares entre componentes são detectadas e cortadas, nunca estourando a pilha
- **Baixo overhead** — o snapshot do estado é recalculado sob demanda (com o painel aberto) ou a cada 25 mensagens, nunca a cada mensagem

## Instalação

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

`devtools.Middleware(nil)` é seguro: um modelo inerte é usado no lugar do modelo `nil`, em vez de causar panic na inicialização.

## Referência da API

### `devtools.Middleware(model tea.Model) tea.Model`

Envolve um modelo Bubble Tea com instrumentação DevTools. Todas as mensagens são interceptadas e registradas. Pressione F12 para alternar o painel do inspector.

O modelo devolvido também implementa `devtools.StateProvider`, que dá acesso programático ao rastreador interno:

```go
wrapped := devtools.Middleware(meuModelo{})

state := wrapped.(devtools.StateProvider).DevState()
fmt.Println(len(state.Messages()))
```

### Navegação do Inspector

| Tecla   | Ação                  |
|---------|-----------------------|
| F12     | Alternar inspector    |
| Tab     | Trocar painel         |
| Cima/Baixo | Rolar conteúdo  |

`Cima` rola para o conteúdo mais antigo e `Baixo` para o mais recente. A rolagem é sempre limitada à quantidade de conteúdo disponível.

### `devtools.DevState`

O rastreador de estado interno. Acessível para uso programático:

- `Messages() []MessageEntry` — retorna histórico de mensagens (mais antigo primeiro)
- `CurrentState() string` — retorna o último snapshot do estado do modelo
- `Components() []ComponentNode` — retorna uma cópia profunda da árvore de componentes

O snapshot é atualizado sob demanda enquanto o inspector está aberto e, no máximo, uma vez a cada 25 mensagens caso contrário — assim, rastrear o estado não deixa o laço de mensagens mais lento.

### `devtools.ComponentNode`

| Campo      | Descrição                                                                 |
|------------|---------------------------------------------------------------------------|
| `Name`     | Nome de exibição: o método `Name()` quando devolve um valor não vazio, o nome do tipo caso contrário |
| `TypeName` | Nome do tipo do modelo via reflexão                                       |
| `Children` | Componentes `tea.Model` aninhados                                         |
| `Cycle`    | `true` quando o nó fecha um ciclo de referências (a recursão é cortada)   |

### Componentes Nomeados

Implemente `Name() string` nos seus modelos para dar nomes descritivos na árvore de componentes:

```go
func (m meuModelo) Name() string {
    return "Contador"
}
```

Um método `Name()` que devolve uma string vazia é ignorado, e o nome do tipo é usado no lugar.

## Licença

MIT
