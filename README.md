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

## Comandos suportados

- `/nick <novo>`: define o nickname do usuário
- `/user <ident>`: define a user@host (identificação do usuário)
- `/join <#canal>`: entra em um canal e notifica peers
- `/part <#canal>`: sai de um canal e notifica peers
- `/msg <canal> <mensagem>`: envia mensagem ao canal especificado
- `/peers`: lista peers conectados
- `/connect <host:porta>`: conecta manualmente a outro peer
- `/help`: exibe esta lista de comandos
- `/quit` ou `/exit`: sai da aplicação
