# P2P-IRC

Cliente IRC peer-to-peer com interface gráfica moderna e funcional.

## Principais Funcionalidades

- **Interface Gráfica Nativa**: Interface gráfica em janela própria usando Fyne
- **Layout de Duas Colunas**: Canais e peers à esquerda, chat à direita
- **Comunicação P2P**: Comunicação direta entre peers sem servidor central
- **Descoberta Automática**: Descoberta automática de peers na rede local
- **Comandos IRC Completos**: Suporte para todos os comandos IRC padrão
- **Configuração Automática**: Geração automática de porta para facilitar o uso
- **Modo de Depuração**: Opção para ativar logs detalhados para diagnóstico
- **Sincronização de Canais**: Sistema robusto para manter canais sincronizados entre peers
- **Prevenção de Loops**: Mecanismo para evitar loops de mensagens na rede P2P

## Instalação

### Pré-requisitos

Para compilar o P2P-IRC, você precisa ter instalado:

- Go 1.16 ou superior
- Bibliotecas de desenvolvimento para GTK/X11:
  ```bash
  sudo apt-get install libgl1-mesa-dev xorg-dev
  ```

### Compilação

```bash
# Clone o repositório
git clone https://github.com/peder1981/p2p-irc.git
cd p2p-irc

# Compile usando o script de build (recomendado)
./build.sh

# Ou compile manualmente
go build -o p2p-irc ./cmd/p2p-irc/main.go
```

## Execução

```bash
# Execução básica
./p2p-irc

# Com modo de depuração
./p2p-irc --debug

# Especificando peers iniciais
./p2p-irc --peers 192.168.1.10:8080,192.168.1.11:8080

# Especificando porta (importante para múltiplas instâncias)
./p2p-irc --port 8081
```

## Opções de Linha de Comando

- `--debug`: Ativa o modo de depuração, exibindo mensagens adicionais
- `--port`: Define a porta para o serviço de descoberta (padrão: porta aleatória)
- `--peers`: Lista de peers iniciais separados por vírgula (ex: 192.168.1.10:8080,192.168.1.11:8080)
- `--config`: Caminho para o arquivo de configuração (padrão: configs/config.toml)

## Comandos Disponíveis

- `/nick <nome>`: Define seu nickname
- `/join <#canal>`: Entra em um canal
- `/part [#canal]`: Sai do canal atual ou especificado
- `/msg <usuário|#canal> <mensagem>`: Envia mensagem privada
- `/who`: Lista usuários na rede
- `/peers`: Lista todos os peers conectados
- `/quit`: Encerra a aplicação
- `/help`: Exibe ajuda com todos os comandos disponíveis

## Interface Gráfica

A interface gráfica do P2P-IRC é composta por:

- **Lista de Canais**: Exibe todos os canais disponíveis
- **Lista de Peers**: Mostra os peers conectados
- **Área de Chat**: Exibe as mensagens do canal ativo
- **Campo de Entrada**: Para digitar mensagens e comandos
- **Barra de Status**: Exibe informações sobre o estado atual
- **Menu**: Acesso a funções como ajuda e saída

## Arquitetura

O P2P-IRC é construído com uma arquitetura modular:

- **Interface do Usuário**: Baseada em Fyne para uma experiência GUI nativa
- **Descoberta de Peers**: Sistema de descoberta de peers na rede local
- **Comunicação P2P**: Comunicação direta entre peers sem servidor central
- **Sincronização de Estado**: Mecanismo para manter o estado sincronizado entre peers
- **Identificação de Mensagens**: Sistema para identificar mensagens únicas e evitar duplicação

## Estrutura do Projeto

```
p2p-irc/
├── cmd/
│   └── p2p-irc/           # Aplicação principal
├── configs/               # Arquivos de configuração
├── docs/                  # Documentação
│   └── GUIA_USUARIO.md    # Guia do usuário
├── internal/
│   ├── config/            # Gerenciamento de configurações
│   ├── discovery/         # Descoberta de peers
│   │   ├── discovery.go   # Serviço principal de descoberta
│   │   ├── message.go     # Estruturas de mensagens
│   │   ├── peer_communication.go # Comunicação entre peers
│   │   └── peer_sync.go   # Sincronização de estado
│   └── ui/                # Interface do usuário
│       ├── gui.go         # Interface gráfica (Fyne)
│       ├── basic_ui.go    # Interface básica
│       └── interface.go   # Interface comum
└── build.sh               # Script de compilação
```

## Comunicação entre Peers

O P2P-IRC implementa um sistema robusto de comunicação entre peers:

1. **Identificação Única**: Cada instância possui um ID único para identificação na rede
2. **Prevenção de Loops**: Mecanismo para evitar processamento duplicado de mensagens
3. **Sincronização de Canais**: Manutenção automática da lista de canais entre peers
4. **Verificação de Conexão**: Sistema de ping/pong para verificar se os peers estão ativos
5. **Propagação de Mensagens**: Envio eficiente de mensagens apenas para peers relevantes

## Solução de Problemas

### Peers não se conectam

- Verifique se as portas estão abertas no firewall
- Use o modo de depuração (`--debug`) para ver mensagens detalhadas
- Certifique-se de que os peers estão na mesma rede

### Mensagens não são recebidas

- Verifique se ambos os peers estão no mesmo canal
- Reinicie as aplicações para forçar uma nova sincronização
- Use o comando `/peers` para verificar se os peers estão conectados

## Contribuindo

Contribuições são bem-vindas! Sinta-se à vontade para abrir issues ou enviar pull requests.

## Autor

Peder Munksgaard (peder@munksgaard.me)

## Licença

Este projeto está licenciado sob a licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.
