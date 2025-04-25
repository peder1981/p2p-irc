# p2p-irc

Cliente de chat estilo IRC totalmente descentralizado em Go.

Este monorepo contém:

- cmd/p2p-irc: ponto de entrada e CLI/TUI
- internal/crypto: gestão de chaves e criptografia (a adicionar)
- internal/transport: libp2p + Pion/ICE (a adicionar)
- internal/dht: descoberta de peers via Kademlia DHT (a adicionar)
- internal/crdt: replicação de estado de canal (a adicionar)
- internal/storage: persistência de histórico e datastore (a adicionar)
- internal/ui: interface TUI com abas (a adicionar)
- configs: arquivos de configuração em TOML
- docs: diagramas e especificações de arquitetura

## Como começar

```bash
cd p2p-irc
go mod tidy
go run cmd/p2p-irc/main.go --config configs/config.toml --port 9001
```

## Build multi-plataforma

Para gerar executáveis para diferentes sistemas operacionais e arquiteturas, use o script:

```bash
bash build-all.sh
```

Isso produzirá os binários em `bin/`, por exemplo:
```
bin/p2p-irc-linux-amd64
bin/p2p-irc-darwin-arm64
bin/p2p-irc-windows-amd64.exe
...e assim por diante
```

## Comandos suportados

- `/nick <novo>`: define o nickname do usuário
- `/user <ident>`: define a user@host (identificação do usuário)
- `/join <#canal>`: entra em um canal e notifica peers
- `/part <#canal>`: sai de um canal e notifica peers
- `/msg <canal> <mensagem>`: envia mensagem ao canal especificado
- `/peers`: lista peers conectados
- `/topic <#canal> <tópico>`: define o tópico de um canal
- `/list [#canal]`: lista canais disponíveis ou peers em um canal
- `/who [#canal]`: lista usuários em um canal
- `/mode <#canal> <modo>`: define modo de um canal
- `/ctcp <nick> <comando>`: envia um CTCP para outro usuário
- `/dcc`: (não implementado) transferência de arquivos via DCC
- `/alias [add <nome> <expansão> | rm <nome>]`: gerencia alias de comandos
- `/script reload`: recarrega scripts e alias
- `/connect <host:porta>`: conecta manualmente a outro peer
- `/help`: exibe esta lista de comandos
- `/quit` ou `/exit`: sai da aplicação

## Scripts e Alias
O p2p-irc suporta uma engine de scripts e alias.  
Scripts padrão são carregados em `~/.p2p-irc/scripts`.  
Comandos:
- `/alias`: lista todos os alias definidos
- `/alias add <nome> <expansão>`: adiciona um alias
- `/alias rm <nome>`: remove um alias
- `/script reload`: recarrega scripts e alias

## Interface Web
Existe uma interface web experimental em `internal/ui/web_ui.go`.  
Para executá-la, remova a diretiva de build (`//go:build ignore`) e chame:
```bash
go run internal/ui/web_ui.go
```
Isso iniciará um servidor HTTP e WebSocket (endereço definido no código).
