# P2P-IRC

Cliente IRC peer-to-peer com interface gráfica moderna e funcional.

## Principais Funcionalidades

- **Interface Gráfica Nativa**: Interface gráfica em janela própria usando Fyne
- **Layout de Duas Colunas**: Canais e peers à esquerda, chat à direita
- **Comunicação P2P**: Comunicação direta entre peers sem servidor central
- **Descoberta Automática**: Descoberta automática de peers na rede local
- **Suporte Multi-instância**: Comunicação entre múltiplas instâncias no mesmo host
- **Sincronização de Canais**: Coordenação robusta de canais entre peers
- **Comandos IRC Completos**: Suporte para todos os comandos IRC padrão
- **Configuração Automática**: Geração automática de porta para facilitar o uso
- **Modo de Depuração**: Opção para ativar logs detalhados para diagnóstico

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
```

## Opções de Linha de Comando

- `--debug`: Ativa o modo de depuração, exibindo mensagens adicionais
- `--port`: Define a porta para o serviço de descoberta (padrão: porta aleatória)
- `--peers`: Lista de peers iniciais separados por vírgula (ex: 192.168.1.10:8080,192.168.1.11:8080)

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

## Novidades na Versão 1.1.0

### Melhorias na Comunicação P2P

- **Comunicação entre instâncias no mesmo host**: Agora é possível executar múltiplas instâncias no mesmo computador e elas se comunicarão corretamente.
- **Melhor detecção de identidade**: O sistema agora identifica corretamente peers usando instanceIDs únicos, permitindo conexões entre diferentes instâncias mesmo com IPs idênticos.
- **Sincronização robusta de canais**: Implementada verificação bidirecional para garantir que todos os peers em um canal estejam cientes uns dos outros.
- **Prevenção de loops de mensagens**: Adicionado mecanismo para evitar ciclos infinitos de mensagens de confirmação entre peers.

### Melhorias na Interface

- **Correções na UI**: Atualizações de interface agora são executadas no thread correto do Fyne, eliminando erros de threading.
- **Feedback visual aprimorado**: Melhor feedback sobre conexões e atividades de peers no sistema.

### Estabilidade

- **Recuperação de falhas**: Implementada proteção contra pânico em goroutines de comunicação.
- **Timeouts ajustados**: Melhor gerenciamento de timeouts para leitura e escrita de mensagens.
- **Detecção de peers inativos**: Sistema de ping/pong mais robusto para detectar e desconectar peers inativos.

## Estrutura do Projeto

```
p2p-irc/
├── cmd/
│   └── p2p-irc/           # Aplicação principal
├── configs/               # Arquivos de configuração
├── internal/
│   ├── config/            # Gerenciamento de configurações
│   ├── discovery/         # Descoberta de peers
│   └── ui/                # Interface do usuário
│       ├── gui.go         # Interface gráfica (Fyne)
│       ├── terminal_ui.go # Interface de terminal (fallback)
│       └── interface.go   # Interface comum
└── build.sh               # Script de compilação
```

## Contribuindo

Contribuições são bem-vindas! Sinta-se à vontade para abrir issues ou enviar pull requests.

## Licença

Este projeto está licenciado sob a licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.
