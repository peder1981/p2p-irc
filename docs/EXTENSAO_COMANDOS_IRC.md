# Extensão Dinâmica de Comandos IRC no P2P-IRC

## Visão Geral
O P2P-IRC permite a adição de comandos IRC customizados em tempo de execução, sem necessidade de alterar a interface gráfica (GUI) ou recompilar o sistema. Isso é possível graças ao roteador central de comandos (`IRCRouter`), que oferece uma API thread-safe para registro dinâmico de handlers.

## Como funciona
- O `IRCRouter` mantém um registro de comandos e seus respectivos handlers.
- Novos comandos podem ser registrados a qualquer momento, inclusive por plugins ou scripts.
- A GUI e outras UIs apenas encaminham o input para o roteador, sem acoplamento à lógica dos comandos.

## Exemplo de Registro de Comando Customizado
```go
// Plugin ou módulo externo
func RegisterCustomCommands(router *ui.IRCRouter) {
    router.Register("echo", func(cmd *ui.IRCCommand, g *ui.GUI, fallback func(string)) bool {
        if len(cmd.Args) > 0 {
            fallback("Echo: " + strings.Join(cmd.Args, " "))
            return true
        }
        fallback("Uso: /echo <mensagem>")
        return true
    })
}

// Em algum ponto de inicialização
RegisterCustomCommands(gui.IRCRouter)
```

## Padrão para Plugins/Scripts
- Plugins devem receber uma instância de `IRCRouter` para registrar seus comandos.
- Scripts podem ser carregados e executados em tempo de execução, desde que tenham acesso ao roteador.
- Recomenda-se documentar comandos customizados para o usuário final.

## Boas práticas
- Sempre retorne `true` se o handler tratar o comando, `false` caso contrário.
- Use o parâmetro `fallback` para exibir mensagens ao usuário.
- Não manipule diretamente a GUI dentro do handler, utilize as interfaces expostas.

## Pontos de Extensão
- O registro pode ser feito em tempo de inicialização, carregamento de plugin ou até via interface de scripting.
- Para suporte a scripts, considere expor o roteador via uma camada como Starlark, Lua ou JS.

## Exemplo de uso com scripting (pseudo-código)
```go
// Exemplo hipotético com Starlark
starlark.Exec(`
    def irc_hello(cmd, gui, fallback):
        fallback("Hello from script!")
        return True
    router.register("hello", irc_hello)
`)
```

## Vantagens
- Permite evolução rápida do sistema sem recompilar a GUI.
- Facilita integração de bots, automações e comandos contextuais.
- Mantém a arquitetura desacoplada e extensível.

---

Para mais exemplos, consulte também `docs/EXEMPLOS_IRCROUTER_COMMANDHISTORY.md`.
