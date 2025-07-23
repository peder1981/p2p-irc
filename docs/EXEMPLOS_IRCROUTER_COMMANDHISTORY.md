# Exemplos de Uso: IRCRouter e CommandHistory nas UIs do P2P-IRC

Este documento apresenta exemplos práticos de integração e uso dos módulos `IRCRouter` e `CommandHistory` nas diferentes interfaces do sistema P2P-IRC (GUI, BasicUI, TerminalUI). O objetivo é facilitar a extensão, manutenção e testes das UIs, promovendo reutilização e padronização do processamento de comandos e histórico.

---

## 1. IRCRouter: Centralização e Roteamento de Comandos IRC

O `IRCRouter` permite registrar comandos IRC customizados e centralizar o roteamento, eliminando lógica duplicada nas UIs.

### Exemplo de Instanciação e Registro de Comandos
```go
// Instanciando o roteador
router := NewIRCRouter()

// Registrando comandos
router.RegisterCommand("/peers", handlePeersCommand)
router.RegisterCommand("/who", handleWhoCommand)
router.RegisterCommand("/info", handleInfoCommand)

// No loop de input da UI:
if router.HandleCommand(channelManager, peerDiscovery, input, fallbackHandler) {
    return // comando tratado
}
fallbackHandler(input) // mensagem comum
```

### Vantagens
- Permite adicionar/remover comandos facilmente.
- Facilita testes unitários dos handlers.
- Evita duplicidade de lógica entre UIs.

---

## 2. CommandHistory: Histórico de Comandos Navegável

O `CommandHistory` fornece armazenamento thread-safe e navegação (setas) para o histórico de comandos recentes.

### Exemplo de Uso em uma UI
```go
// Instanciando o histórico (limite de 50 comandos)
history := NewCommandHistory(50)

// Adicionando um comando ao histórico
history.Add(input)

// Navegando (exemplo: atrelado a eventos de tecla)
prevCmd := history.Prev() // seta para cima
nextCmd := history.Next() // seta para baixo

// Exemplo de integração com campo de input:
inputField.OnKeyPress = func(key Key) {
    switch key {
    case KeyUp:
        inputField.SetText(history.Prev())
    case KeyDown:
        inputField.SetText(history.Next())
    }
}
```

### Vantagens
- Histórico consistente entre diferentes UIs.
- Fácil de integrar com eventos de teclado.
- Thread-safe para uso concorrente.

---

## 3. Integração Recomendada nas UIs

### GUI (Fyne)
```go
// No construtor:
g.commandHistory = NewCommandHistory(50)
g.router = NewIRCRouter()
g.router.RegisterCommand("/peers", handlePeersCommand)
// ...
// No evento de submit do inputField:
g.commandHistory.Add(input)
if g.router.HandleCommand(g, g.discovery, input, fallback) {
    return
}
fallback(input)
```

### BasicUI/TerminalUI
```go
// Instanciação
ui.commandHistory = NewCommandHistory(50)
ui.router = NewIRCRouter()
ui.router.RegisterCommand("/who", handleWhoCommand)
// ...
// No loop de leitura:
ui.commandHistory.Add(input)
if ui.router.HandleCommand(ui, ui.discovery, input, fallback) {
    continue
}
fallback(input)
```

---

## 4. Testando Handlers de Comando

Os handlers podem ser testados isoladamente, injetando mocks de `ChannelManager` e `PeerDiscovery`:

```go
type mockDiscovery struct { /* ... */ }
type mockChannelManager struct { /* ... */ }

func TestHandlePeersCommand(t *testing.T) {
    g := &GUI{ /* ... */ }
    ok := handlePeersCommand(g, g.discovery, "/peers", fallback)
    // Asserções...
}
```

---

## 5. Boas Práticas
- Sempre registre comandos no roteador durante a inicialização da UI.
- Use o histórico de comandos em todos os pontos de entrada de input.
- Prefira handlers desacoplados, recebendo interfaces (`ChannelManager`, `PeerDiscovery`).
- Escreva testes unitários para cada handler de comando.

---

**Dúvidas ou sugestões? Contribua com exemplos e melhorias neste arquivo!**
