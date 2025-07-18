# Integração Mesh P2P-IRC com Bitchat

## Visão Geral

A integração mesh do P2P-IRC com o Bitchat permite criar uma rede híbrida que combina a comunicação tradicional TCP/mDNS com tecnologias Bluetooth Mesh, proporcionando maior alcance, robustez e capacidades offline.

## Arquitetura

### Componentes Principais

1. **MeshIntegration** (`internal/mesh/mesh_integration.go`)
   - Gerencia peers mesh e transporte híbrido
   - Coordena descoberta híbrida (mDNS + Bluetooth Mesh)
   - Monitora saúde da rede e coleta métricas
   - Fornece interface unificada para envio de mensagens

2. **BitchatBridge** (`internal/mesh/bluetooth_bridge.go`)
   - Ponte de comunicação com o daemon Bitchat
   - Gerencia processo Bitchat e filas de mensagens
   - Converte entre protocolos P2P-IRC e Bitchat
   - Monitora status da conexão Bluetooth

3. **MeshUI** (`internal/ui/mesh_ui.go`)
   - Interface gráfica para informações mesh
   - Exibe peers, métricas e status da rede
   - Notificações visuais para eventos mesh
   - Atualização em tempo real

### Fluxo de Dados

```
P2P-IRC App
    ↓
MeshIntegration ←→ Discovery Service (mDNS/DHT)
    ↓
BitchatBridge ←→ Bitchat Daemon
    ↓
Bluetooth Mesh Network
```

## Configuração

### Arquivo de Configuração

Crie um arquivo `config-mesh.toml` baseado no exemplo em `configs/config-mesh.toml`:

```toml
[mesh]
enabled = true
bitchatPath = "/path/to/bitchat-binary"
enableEncryption = true
enableCompression = true
networkId = "p2p-irc-mesh"
logLevel = "info"
autoFallback = true
```

### Parâmetros de Configuração

- **enabled**: Habilita funcionalidades mesh
- **bitchatPath**: Caminho para o binário do Bitchat
- **enableEncryption**: Ativa criptografia nas mensagens mesh
- **enableCompression**: Ativa compressão LZ4
- **networkId**: Identificador da rede mesh
- **logLevel**: Nível de log (debug, info, warn, error)
- **autoFallback**: Fallback automático para TCP quando Bluetooth indisponível

## Instalação e Uso

### Pré-requisitos

1. **Bitchat Binário**: Compile o projeto Bitchat-Go
   ```bash
   cd /path/to/bitchat-go
   ./build.sh
   ```

2. **Bluetooth**: Sistema Linux com BlueZ instalado
   ```bash
   sudo apt-get install bluez bluez-tools
   ```

3. **Permissões**: Usuário deve ter acesso ao Bluetooth
   ```bash
   sudo usermod -a -G bluetooth $USER
   ```

### Execução

1. **Com arquivo de configuração**:
   ```bash
   ./p2p-irc --config config-mesh.toml
   ```

2. **Com parâmetros de linha de comando**:
   ```bash
   ./p2p-irc --mesh-enabled --bitchat-path /path/to/bitchat
   ```

### Comandos IRC Mesh

- `/mesh status` - Exibe status da rede mesh
- `/mesh enable` - Habilita funcionalidades mesh
- `/mesh disable` - Desabilita funcionalidades mesh
- `/mesh peers` - Lista peers mesh descobertos
- `/mesh metrics` - Exibe métricas da rede

## Interface do Usuário

### Indicadores Visuais

- **Status da Rede**: Verde (conectado), Amarelo (conectando), Vermelho (desconectado)
- **Qualidade do Sinal**: Barras indicando força da conexão
- **Contadores**: Peers ativos, mensagens enviadas/recebidas
- **Uptime**: Tempo de funcionamento da rede mesh

### Notificações

- Novo peer descoberto
- Peer desconectado
- Falhas de conexão
- Mudanças de status da rede

## Desenvolvimento

### Estrutura de Arquivos

```
internal/mesh/
├── mesh_integration.go    # Integração principal
├── bluetooth_bridge.go    # Ponte Bitchat
└── types.go              # Tipos e estruturas

internal/ui/
├── mesh_ui.go            # Interface mesh
└── components.go         # Componentes UI

tests/
├── mesh_integration_test.go  # Testes de integração
└── mesh_benchmark_test.go    # Benchmarks

configs/
└── config-mesh.toml      # Configuração exemplo
```

### Testes

Execute os testes de integração:

```bash
# Testes básicos
go test ./tests -v

# Testes com Bitchat real (se disponível)
go test ./tests -v -tags=integration

# Benchmarks
go test ./tests -bench=.
```

### Debugging

Habilite logs detalhados:

```bash
./p2p-irc --config config-mesh.toml --debug
```

Ou configure no arquivo TOML:
```toml
[mesh]
logLevel = "debug"
```

## Interoperabilidade

### Comunicação com Clientes Bitchat

A integração permite comunicação direta com clientes Bitchat nativos através da ponte de protocolo que:

1. Converte mensagens P2P-IRC para formato Bitchat
2. Mantém compatibilidade com criptografia Ed25519/Curve25519
3. Gerencia fragmentação de pacotes grandes
4. Implementa retry automático para mensagens perdidas

### Fallback Automático

Quando Bluetooth não está disponível:

1. Sistema detecta falha automaticamente
2. Reverte para modo TCP/mDNS tradicional
3. Mantém funcionalidade completa
4. Tenta reconectar periodicamente

## Monitoramento e Métricas

### Métricas Coletadas

- **NetworkUptime**: Tempo de funcionamento
- **ActivePeers**: Peers atualmente conectados
- **TotalPeersDiscovered**: Total de peers descobertos
- **MessagesSent/Received**: Contadores de mensagens
- **ConnectionQuality**: Qualidade da conexão (0-100)
- **LastActivity**: Timestamp da última atividade

### Exportação de Métricas

As métricas podem ser exportadas para sistemas de monitoramento:

```go
metrics := meshIntegration.GetMetrics()
fmt.Printf("Uptime: %v, Peers: %d\n", 
    metrics.NetworkUptime, metrics.ActivePeers)
```

## Solução de Problemas

### Problemas Comuns

1. **Bitchat não inicia**
   - Verifique o caminho do binário
   - Confirme permissões de execução
   - Verifique logs de erro

2. **Bluetooth não funciona**
   - Confirme BlueZ instalado
   - Verifique permissões do usuário
   - Teste com `bluetoothctl`

3. **Peers não se descobrem**
   - Verifique firewall
   - Confirme mesmo networkId
   - Teste conectividade básica

### Logs de Debug

Principais logs para investigação:

```
[MESH] Iniciando integração mesh...
[BRIDGE] Conectando ao Bitchat daemon...
[DISCOVERY] Peer descoberto via mesh: <peer-id>
[TRANSPORT] Enviando mensagem via mesh para <peer-id>
```

## Roadmap

### Funcionalidades Futuras

- [ ] Suporte para Windows e macOS
- [ ] Interface web para monitoramento
- [ ] Métricas Prometheus nativas
- [ ] Roteamento mesh inteligente
- [ ] Sincronização de histórico via mesh
- [ ] Criptografia end-to-end aprimorada

### Melhorias Planejadas

- [ ] Otimização de performance
- [ ] Redução de latência
- [ ] Melhor tratamento de erros
- [ ] Interface de configuração gráfica
- [ ] Documentação de API

## Contribuição

Para contribuir com a integração mesh:

1. Fork o repositório
2. Crie branch para sua feature
3. Implemente testes adequados
4. Atualize documentação
5. Submeta pull request

### Diretrizes de Código

- Siga padrões Go estabelecidos
- Adicione testes para novas funcionalidades
- Mantenha compatibilidade com versões anteriores
- Documente APIs públicas
- Use logging estruturado

## Licença

Este projeto está sob a mesma licença do P2P-IRC principal.

## Autor

Peder Munksgaard - Integração Mesh P2P-IRC com Bitchat

---

Para mais informações, consulte:
- [README principal](../README.md)
- [Documentação da API](API.md)
- [Guia do usuário](GUIA_USUARIO.md)
