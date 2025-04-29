# P2P-IRC

Cliente IRC peer-to-peer com interface TUI moderna e funcional.

## Principais Funcionalidades

- **Interface Unificada**: Uma única interface intuitiva e bem organizada
- **Layout de Duas Colunas**: Canais e peers à esquerda, chat e logs à direita
- **Notificações Visuais**: Indicadores para canais com mensagens não lidas
- **Histórico Persistente**: Armazenamento e carregamento automático do histórico de mensagens
- **Comandos IRC Completos**: Suporte para todos os comandos IRC padrão
- **Área de Logs Separada**: Separação clara entre mensagens de sistema e chat
- **Recuperação Automática**: Monitor de pings e reconexão automática a peers conhecidos

## Instalação

```bash
# Clone o repositório
git clone https://github.com/peder1981/p2p-irc.git
cd p2p-irc

# Compile o cliente
go build -o p2p-irc ./cmd/p2p-irc/main.go
```

## Execução

```bash
# Execução básica
./p2p-irc

# Com modo de depuração
./p2p-irc --debug

# Especificando porta e peers iniciais
./p2p-irc --port 8081 --peers 192.168.1.10:8080,192.168.1.11:8080
```

## Opções de Linha de Comando

- `--debug`: Ativa o modo de depuração, exibindo mensagens adicionais na área de logs
- `--port`: Define a porta para o serviço de descoberta (padrão: 8080)
- `--peers`: Lista de peers iniciais separados por vírgula (ex: 192.168.1.10:8080,192.168.1.11:8080)

## Comandos Disponíveis

- `/nick <nome>`: Define seu nickname
- `/join <#canal>`: Entra em um canal
- `/part [#canal]`: Sai do canal atual ou especificado
- `/msg <usuário|#canal> <mensagem>`: Envia mensagem privada
- `/who`: Lista usuários na rede
- `/peers`: Lista todos os peers conectados
- `/quit` ou `/exit`: Encerra a aplicação

## Atalhos de Teclado

- **F1**: Exibe ajuda detalhada
- **Alt+1-9**: Alternar entre canais
- **Ctrl+L**: Limpar logs
- **Ctrl+P**: Alternar foco entre painéis
- **Ctrl+N**: Criar novo canal
- **Esc**: Voltar para o campo de entrada

## Arquitetura

O P2P-IRC é construído com uma arquitetura modular:

- **Interface do Usuário**: Baseada em tview para uma experiência TUI moderna
- **Descoberta de Peers**: Utiliza mDNS e DHT para descoberta de peers na rede
- **Transporte P2P**: Baseado em WebRTC para comunicação direta entre peers
- **Histórico Persistente**: Armazenamento local de mensagens para continuidade

## Estrutura do Projeto

```
p2p-irc/
├── cmd/
│   └── p2p-irc/           # Aplicação principal
├── configs/               # Arquivos de configuração
├── docs/                  # Documentação
├── history/               # Histórico de mensagens
├── internal/
│   ├── config/            # Gerenciamento de configurações
│   ├── discovery/         # Descoberta de peers
│   ├── dht/               # Implementação de DHT
│   └── ui/                # Interface do usuário
│       └── unified_ui.go  # Interface unificada
└── tests/                 # Testes automatizados
```

## Contribuindo

Contribuições são bem-vindas! Veja [CONTRIBUTING.md](./docs/CONTRIBUTING.md) para mais detalhes.

## Documentação

Para mais informações, consulte:

- [Guia do Usuário](./docs/GUIA_USUARIO.md)
- [Documentação Técnica](./docs/DOCUMENTACAO_TECNICA.md)

## Licença

Este projeto está licenciado sob a licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.
