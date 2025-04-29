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

## Descoberta Automática de Peers

O p2p-irc oferece dois modos de descoberta automática de peers:

- **LAN (Rede Local):** utiliza broadcast UDP na mesma porta TCP especificada (por exemplo, `--port 9001`).  
  Ao executar em várias máquinas na mesma rede local com a mesma porta, elas se encontrarão e conectarão automaticamente.

- **Internet:** baseia-se em peers sementes configurados em `configs/config.toml`.  
  No arquivo de configuração, defina:

  ```toml
  [network]
  bootstrapPeers = ["example.com:9001", "1.2.3.4:9001"]

  # Servidores ICE (STUN/TURN) para NAT traversal
  [[network.iceServers]]
  urls = ["stun:stun.l.google.com:19302"]

  [[network.iceServers]]
  urls = ["turn:turn.exemplo.com:3478"]
  username = "usuario"
  credential = "senha"
  ```

  O cliente usará essas seeds para:
  1. Estabelecer uma conexão TCP de sinalização com cada peer listado.
  2. Negociar um canal WebRTC DataChannel via ICE (STUN/TURN) para NAT traversal.
  Inclua servidores TURN para suportar NATs simétricos ou quando STUN não for suficiente.

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
- `/dcc`: transferência de arquivos via DCC
- `/alias [add <nome> <expansão> | rm <nome>]`: gerencia alias de comandos
- `/script reload`: recarrega scripts e alias
- `/connect <host:porta>`: conecta manualmente a outro peer
- `/help`: exibe esta lista de comandos
- `/quit` ou `/exit`: sai da aplicação

## Funcionalidades

- Chat em grupo P2P
- Descoberta automática de peers via mDNS
- Suporte a aliases
- Transferência de arquivos via DCC
- Suporte a scripts
- Persistência de mensagens
- Criptografia (em desenvolvimento)

## Comandos

### Básicos
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
- `/alias [add <nome> <expansão> | rm <nome>]`: gerencia alias de comandos
- `/script reload`: recarrega scripts e alias
- `/connect <host:porta>`: conecta manualmente a outro peer
- `/help`: exibe esta lista de comandos
- `/quit` ou `/exit`: sai da aplicação

### DCC (Direct Client-to-Client)

O P2P IRC suporta transferência direta de arquivos entre usuários através do protocolo DCC.

Funcionalidades:
- Transferência direta de arquivos entre usuários
- Transferência de diretórios completos
- Barra de progresso visual
- Pause/Resume de transferências em andamento
- Retomada de transferências interrompidas
- Verificação de integridade via SHA-256
- Suporte a múltiplos arquivos simultâneos
- Gerenciamento de fila de transferências
- Estatísticas em tempo real (velocidade, ETA)
- Logging detalhado para depuração
- Persistência de estado para recuperação após reinicialização

Comandos disponíveis:

- `/dcc send <arquivo1,arquivo2,...> <usuário>`: Envia um ou mais arquivos para outro usuário
- `/dcc senddir <diretório> <usuário>`: Envia um diretório completo para outro usuário
- `/dcc list`: Lista todas as transferências ativas
- `/dcc pause <id>`: Pausa uma transferência em andamento
- `/dcc resume <id>`: Retoma uma transferência pausada
- `/dcc cancel <id>`: Cancela uma transferência
- `/dcc info <id>`: Exibe informações detalhadas sobre uma transferência
- `/dcc verify <id>`: Verifica a integridade de uma transferência concluída
- `/dcc clear`: Remove transferências completadas ou falhas da lista

#### Configuração Avançada

O DCC pode ser configurado através do arquivo de configuração:

```toml
[dcc]
downloadDir = "./downloads"     # Diretório para downloads
bufferSize = 32768              # Tamanho do buffer em bytes (32KB padrão)
maxConcurrent = 5               # Máximo de transferências simultâneas
verifyHash = true               # Verifica integridade via hash SHA-256
logEnabled = false              # Habilita logging detalhado
logFile = "./dcc.log"           # Arquivo de log (vazio para stdout)
autoResume = true               # Retoma automaticamente transferências interrompidas
persistenceFile = "dcc_state.json" # Arquivo para persistência de estado
```

#### Exemplos

```bash
# Enviar um arquivo
/dcc send foto.jpg usuario1

# Enviar múltiplos arquivos
/dcc send foto1.jpg,foto2.jpg,doc.pdf usuario1

# Enviar um diretório completo
/dcc senddir ./fotos usuario1

# Listar transferências
/dcc list

# Pausar uma transferência
/dcc pause 12345

# Retomar uma transferência
/dcc resume 12345

# Cancelar uma transferência
/dcc cancel 12345

# Ver informações detalhadas
/dcc info 12345

# Verificar integridade de um arquivo recebido
/dcc verify 12345
```

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
