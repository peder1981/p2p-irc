# p2p-irc

Cliente de chat estilo IRC totalmente descentralizado em Go.

Este monorepo contém:

- cmd/p2p-irc: CLI principal com suporte a chat P2P
- cmd/p2p-irc-tui: Interface TUI alternativa (experimental)
- internal/config: Gerenciamento de configurações TOML
- internal/crypto: Gestão de identidade e chaves
- internal/dcc: Transferência de arquivos via DCC
- internal/dht: Descoberta de peers via DHT
- internal/discovery: Descoberta automática via mDNS/DNS-SD
- internal/network: Gerenciamento de conexões
- internal/portmanager: Gerenciamento de portas e UPnP
- internal/scripts: Sistema de scripts e alias
- internal/storage: Persistência de dados
- internal/transport: Camada de transporte com WebRTC
- internal/ui: Interface de usuário (TUI)

## Como começar

```bash
cd p2p-irc
go mod tidy
go run cmd/p2p-irc/main.go --config configs/config.toml
```

## Descoberta Automática de Peers

O p2p-irc oferece dois modos de descoberta automática de peers:

- **LAN (Rede Local):** utiliza mDNS/DNS-SD para descoberta automática na mesma rede local.
  - Descoberta automática de peers na mesma rede
  - Rate limiting para reconexões (máximo 3 tentativas em burst, depois 1 a cada 5 segundos)
  - Logs detalhados para depuração
  - Métricas internas para monitoramento
  - Limpeza automática de peers inativos
  - Suporte a UPnP para mapeamento de portas

- **Internet:** utiliza WebRTC para conexões através de NAT.
  No arquivo de configuração, defina:

  ```toml
  [network]
  bootstrapPeers = ["example.com:9001", "1.2.3.4:9001"]

  # Servidores ICE (STUN/TURN) para NAT traversal
  [[network.iceServers]]
  urls = ["stun:stun.l.google.com:19302"]
  ```

## Monitoramento e Métricas

O p2p-irc inclui um sistema interno de métricas para monitorar o estado da rede P2P:

### Métricas do Serviço de Descoberta
- **Peers Ativos**: número atual de peers conectados
- **Total de Peers**: total de peers descobertos desde o início
- **Tentativas de Busca**: número de tentativas de descoberta de peers
- **Erros de Busca**: número de falhas na descoberta
- **Reconexões**: número de tentativas de reconexão
- **Peers Ignorados**: número de peers filtrados (própria instância)
- **Tempo de Atividade**: tempo desde o início do serviço
- **Última Descoberta**: timestamp da última descoberta bem-sucedida

### Rate Limiting
O serviço implementa rate limiting para reconexões:
- Máximo de 3 tentativas em burst
- Depois limita a 1 reconexão a cada 5 segundos
- Logs detalhados para depuração de problemas de conectividade

### Logging
Logs detalhados são gerados para eventos importantes:
- Início e encerramento do serviço
- Descoberta de novos peers
- Falhas de conexão e reconexões
- Limpeza de cache de peers
- Mapeamento de portas UPnP

## Comandos Suportados

Os seguintes comandos estão implementados e funcionando:

- `/nick <novo>`: Define seu nickname no chat
- `/join <#canal>`: Entra em um canal (adiciona # automaticamente se não fornecido)
- `/part <#canal>`: Sai de um canal (adiciona # automaticamente se não fornecido)
- `/msg <canal> <mensagem>`: Envia mensagem para um canal específico
- `/peers`: Lista todos os peers conectados atualmente
- `/dcc send <arquivo1,arquivo2,...> <usuário>`: Envia arquivos via DCC
- `/dcc list`: Lista todas as transferências DCC ativas
- `/help`: Mostra a lista de comandos disponíveis
- `/quit` ou `/exit`: Encerra a aplicação

Observações:
- Todos os canais começam com #
- Se o canal não for especificado em /msg, a mensagem será enviada para o canal atual
- O comando /peers mostra informações detalhadas sobre cada peer conectado
- Os comandos não são sensíveis a maiúsculas/minúsculas

## DCC (Direct Client-to-Client)

O P2P IRC suporta transferência direta de arquivos entre usuários através do protocolo DCC.

Funcionalidades:
- Transferência direta de arquivos entre usuários
- Suporte a múltiplos arquivos simultâneos
- Gerenciamento de fila de transferências
- Estatísticas em tempo real (velocidade, ETA)
- Logging detalhado para depuração

Comandos disponíveis:
- `/dcc send <arquivo1,arquivo2,...> <usuário>`: Envia arquivos para outro usuário
- `/dcc list`: Lista todas as transferências ativas

## Interface TUI Alternativa

Existe uma interface TUI experimental em `cmd/p2p-irc-tui`.
Para executá-la:

```bash
go run cmd/p2p-irc-tui/main.go
```

## Configuração

O arquivo de configuração (`configs/config.toml`) suporta:

```toml
[network]
# Peers para bootstrap
bootstrapPeers = ["example.com:9001"]

# Servidores ICE para NAT traversal
[[network.iceServers]]
urls = ["stun:stun.l.google.com:19302"]

[dcc]
# Configurações DCC
downloadDir = "./downloads"
maxConcurrent = 5
logEnabled = false
```
