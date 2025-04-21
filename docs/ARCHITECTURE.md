# P2P‑IRC Arquitetura de Alto Nível

## Visão Geral dos Módulos

1. cmd/p2p‑irc (entrypoint)
   - main.go: carrega configuração, inicializa módulos (crypto, transport, DHT, CRDT, UI)
   - Flags: porta, config file, nível de log, etc.

2. internal/crypto
   - Geração e armazenamento de chave Ed25519 (assinatura) + X25519 (ECDH)
   - AEAD (ChaCha20‑Poly1305/AES‑GCM) para encriptação de mensagens
   - Load/Save de chaves em $XDG_CONFIG_HOME/p2p‑irc, permissão 0600

3. internal/transport
   - Abstração sobre go‑libp2p: cria Host, gerencia streams cifrados (mTLS)
   - Integra ICE/STUN/TURN via Pion/ICE para NAT traversal
   - Graph de conexões P2P, dial/accept de peers

4. internal/dht
   - Wrapper de go‑libp2p-kad-dht para descoberta global
   - Bootstrap em seed nodes configurados
   - Put/Get de registros por chave `/channel/<nome>` apontando para endereços de peers
   - TTL configurável, renovação periódica

5. internal/crdt
   - Delta‑state CRDT (OR‑Set para membros/ops/ban; LWW‑Register para tópico)
   - Realiza merges automáticos ao receber updates de peers
   - Exposição de API: Join(), Leave(), SetTopic(), AddOp(), RemoveMember(), Snapshot()

6. internal/storage
   - Persistência em disco de logs de mensagens (SQLite ou arquivos JSON/NDJSON)
   - DHT datastore local (padrão libp2p datastore)
   - Histórico de CRDTs para replay e sincronização de novas entradas

7. internal/ui
   - TUI via tview ou gocui
   - Barra de abas no topo: canais (#golang) e DMs (@alice)
   - Navegação: F1–F9 / Alt+Número / clique do mouse
   - Sidebars: lista de canais à esquerda, lista de usuários à direita

8. configs
   - config.toml (TOML)
     – network.bootstrapPeers
     – network.stunServers
     – dht.datastorePath, dht.ttl
     – ui.keybindings, ui.theme

9. docs
   - Diagramas (Mermaid) de fluxo de mensagens e especificações empregadas
   - Detalhes de segurança e threat model

## Fluxo de Inicialização

```
main() → loadConfig() → crypto.LoadKeys() → transport.NewHost(keys)
           ↓                       ↓                      ↓
       dht.Bootstrap()      ice.GatherCandidates()   crdt.NewManager()
           ↓                       ↓                      ↓
      dht.FindPeers(channels) → transport.Dial(peers) → crdt.JoinRoom()
           ↓                       ↓                      ↓
             ui.Run() → renderTabs() → inputHandler() → crdt.Publish()
```

## Fluxo de Mensagens no Canal

1. Usuário digita `/join #chat`
2. CRDT.JoinRoom("chat")
3. // DHT register
   dht.Put("/channel/chat", localPeerRecord)
4. // Periodicamente
   peers := dht.GetPeers("/channel/chat")
   transport.EnsureConnected(peers)
5. Quando usuário envia texto
   crdt.Update(ORSetAddMessage, payload)
   transport.Broadcast(crdt.Delta)
6. Ao receber delta
   crdt.Merge(delta)
   ui.RenderMessage()
```

## Persistência e Permissões

- $XDG_CONFIG_HOME/p2p‑irc/identity.key (0600)
- $XDG_CONFIG_HOME/p2p‑irc/config.toml (0644)
- $XDG_DATA_HOME/p2p‑irc/history/
  - channels/<nome>.log (0600)
  - dms/<peerid>.log (0600)
