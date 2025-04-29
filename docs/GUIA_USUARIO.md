# Guia do Usuário - P2P-IRC

## Introdução

O P2P-IRC é um cliente IRC peer-to-peer que permite comunicação em tempo real sem a necessidade de um servidor central. Este guia explica como utilizar o cliente e suas funcionalidades.

## Instalação

Para instalar o P2P-IRC, siga os passos abaixo:

1. Clone o repositório:
   ```
   git clone https://github.com/peder1981/p2p-irc.git
   ```

2. Entre no diretório do projeto:
   ```
   cd p2p-irc
   ```

3. Compile o cliente:
   ```
   go build -o p2p-irc ./cmd/p2p-irc/main_unificado.go
   ```

## Execução

Para iniciar o cliente:

```
./p2p-irc [opções]
```

Opções disponíveis:
- `--debug`: Ativa o modo de depuração, exibindo mensagens adicionais na área de logs

## Interface do Usuário

A interface do P2P-IRC é dividida em quatro áreas principais:

1. **Lista de Canais** (esquerda superior): Mostra todos os canais em que você está participando
2. **Lista de Peers** (esquerda inferior): Mostra todos os peers conectados à rede
3. **Área de Chat** (direita superior): Exibe as mensagens do canal ou conversa atual
4. **Área de Logs** (direita inferior): Mostra mensagens do sistema e logs de depuração
5. **Campo de Entrada** (parte inferior): Para digitar mensagens e comandos

## Atalhos de Teclado

O P2P-IRC oferece diversos atalhos de teclado para facilitar a navegação:

- **F1**: Exibe ajuda detalhada com todos os atalhos e comandos
- **Alt+1-9**: Alterna entre os canais disponíveis
- **Ctrl+L**: Limpa a área de logs
- **Ctrl+P**: Alterna o foco entre os diferentes painéis (canais, peers, chat, logs)
- **Ctrl+N**: Cria um novo canal
- **Esc**: Volta para o campo de entrada

## Comandos

Os comandos são iniciados com o caractere `/`. Os principais comandos são:

- `/nick <nome>`: Define seu nickname
- `/join <#canal>`: Entra em um canal
- `/part [#canal]`: Sai do canal atual ou especificado
- `/msg <usuário|#canal> <mensagem>`: Envia mensagem privada para um usuário ou canal
- `/who`: Lista usuários conectados na rede
- `/peers`: Lista todos os peers conectados
- `/quit` ou `/exit`: Encerra a aplicação

## Notificações Visuais

O P2P-IRC inclui indicadores visuais para mensagens não lidas:

- Canais com mensagens não lidas são destacados em amarelo
- Um contador ao lado do nome do canal indica o número de mensagens não lidas

## Histórico de Mensagens

O histórico de mensagens é salvo automaticamente na pasta `history/` e carregado quando você entra em um canal. Isso garante que você não perca mensagens importantes mesmo após reiniciar o cliente.

## Solução de Problemas

### Problemas de Conexão

Se você estiver enfrentando problemas para se conectar a outros peers:

1. Verifique sua conexão de internet
2. Verifique se as portas necessárias estão abertas no seu firewall
3. Tente reiniciar o cliente com a opção `--debug` para obter mais informações

### Outros Problemas

Para outros problemas, consulte a área de logs (parte inferior da interface) para obter informações detalhadas sobre o que está acontecendo.

## Desenvolvimento

O P2P-IRC é um projeto de código aberto e contribuições são bem-vindas. Para contribuir:

1. Faça um fork do repositório
2. Crie uma branch para sua feature (`git checkout -b minha-nova-feature`)
3. Faça commit das suas mudanças (`git commit -am 'Adiciona nova feature'`)
4. Faça push para a branch (`git push origin minha-nova-feature`)
5. Crie um novo Pull Request
