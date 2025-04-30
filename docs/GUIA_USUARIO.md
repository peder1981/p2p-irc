# Guia do Usuário - P2P-IRC

## Introdução

O P2P-IRC é um cliente IRC peer-to-peer que permite comunicação em tempo real sem a necessidade de um servidor central. Este guia explica como utilizar o cliente e suas funcionalidades.

## Instalação

### Pré-requisitos

Para compilar o P2P-IRC, você precisa ter instalado:

- Go 1.16 ou superior
- Bibliotecas de desenvolvimento para GTK/X11:
  ```bash
  sudo apt-get install libgl1-mesa-dev xorg-dev
  ```

### Passos para instalação

1. Clone o repositório:
   ```
   git clone https://github.com/peder1981/p2p-irc.git
   ```

2. Entre no diretório do projeto:
   ```
   cd p2p-irc
   ```

3. Compile o cliente usando o script de build:
   ```
   ./build.sh
   ```
   
   Ou compile manualmente:
   ```
   go build -o p2p-irc ./cmd/p2p-irc/main.go
   ```

## Execução

Para iniciar o cliente:

```
./p2p-irc [opções]
```

Opções disponíveis:
- `--debug`: Ativa o modo de depuração, exibindo mensagens adicionais
- `--port`: Define a porta para o serviço de descoberta (padrão: porta aleatória)
- `--peers`: Lista de peers iniciais separados por vírgula (ex: 192.168.1.10:8080,192.168.1.11:8080)

## Interface Gráfica

A interface gráfica do P2P-IRC é composta por:

1. **Lista de Canais** (esquerda superior): Mostra todos os canais em que você está participando
2. **Lista de Peers** (esquerda inferior): Mostra todos os peers conectados à rede
3. **Área de Chat** (direita): Exibe as mensagens do canal atual
4. **Campo de Entrada** (parte inferior): Para digitar mensagens e comandos
5. **Barra de Status** (parte inferior): Exibe informações sobre o estado atual

## Menu da Aplicação

O P2P-IRC possui um menu simples com as seguintes opções:

- **Arquivo**
  - **Sair**: Encerra a aplicação

- **Ajuda**
  - **Comandos**: Exibe uma lista de todos os comandos disponíveis
  - **Sobre**: Exibe informações sobre o P2P-IRC

## Comandos

Os comandos são iniciados com o caractere `/`. Os principais comandos são:

- `/nick <nome>`: Define seu nickname
- `/join <#canal>`: Entra em um canal
- `/part [#canal]`: Sai do canal atual ou especificado
- `/msg <usuário|#canal> <mensagem>`: Envia mensagem privada para um usuário ou canal
- `/who`: Lista usuários conectados na rede
- `/peers`: Lista todos os peers conectados
- `/quit`: Encerra a aplicação
- `/help`: Exibe a lista de comandos disponíveis

## Funcionalidades

### Canais

Para entrar em um canal, use o comando `/join #nome_do_canal`. Se o canal não existir, ele será criado automaticamente.

Para sair de um canal, use o comando `/part` ou `/part #nome_do_canal`.

### Mensagens Privadas

Para enviar uma mensagem privada para outro usuário, use o comando `/msg nome_do_usuário mensagem`.

### Descoberta de Peers

O P2P-IRC descobre automaticamente outros peers na rede local. Você também pode conectar-se a peers específicos usando a opção `--peers` na linha de comando.

## Solução de Problemas

### Problemas de Conexão

Se você estiver enfrentando problemas para se conectar a outros peers:

1. Verifique sua conexão de internet
2. Verifique se as portas necessárias estão abertas no seu firewall
3. Tente reiniciar o cliente com a opção `--debug` para obter mais informações

### Problemas com a Interface Gráfica

Se a interface gráfica não iniciar corretamente:

1. Verifique se as bibliotecas necessárias estão instaladas:
   ```
   sudo apt-get install libgl1-mesa-dev xorg-dev
   ```
2. Tente recompilar o cliente usando o script de build:
   ```
   ./build.sh
   ```

## Desenvolvimento

O P2P-IRC é um projeto de código aberto e contribuições são bem-vindas. Para contribuir:

1. Faça um fork do repositório
2. Crie uma branch para sua feature (`git checkout -b minha-nova-feature`)
3. Faça commit das suas mudanças (`git commit -am 'Adiciona nova feature'`)
4. Faça push para a branch (`git push origin minha-nova-feature`)
5. Crie um novo Pull Request
