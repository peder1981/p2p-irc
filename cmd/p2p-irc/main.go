package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/peder1981/p2p-irc/internal/discovery"
	"github.com/peder1981/p2p-irc/internal/ui"
)

// Configuração global
var (
	config struct {
		Nickname string
		Port     int
	}
	nickname string // Nome do usuário
	channels      []string
	activeChannel string
)

// Config representa a estrutura de configuração do aplicativo
type Config struct {
	Network NetworkConfig `toml:"network"`
	UI      UIConfig      `toml:"ui"`
}

// NetworkConfig contém configurações de rede
type NetworkConfig struct {
	Port        int      `toml:"port"`
	StunServers []string `toml:"stunServers"`
}

// UIConfig contém configurações da interface do usuário
type UIConfig struct {
	DebugMode    bool   `toml:"debugMode"`
	MaxLogLines  int    `toml:"maxLogLines"`
	HistoryDir   string `toml:"historyDir"`
}

// Carrega a configuração do arquivo
func loadConfig(configPath string) (*Config, error) {
	// Configuração padrão
	config := &Config{
		Network: NetworkConfig{
			Port:        8080,
			StunServers: []string{"stun:stun.l.google.com:19302"},
		},
		UI: UIConfig{
			DebugMode:   false,
			MaxLogLines: 100,
			HistoryDir:  "history",
		},
	}

	// Se o arquivo de configuração existir, carrega-o
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, config); err != nil {
			return nil, fmt.Errorf("erro ao decodificar arquivo de configuração: %v", err)
		}
	}

	return config, nil
}

func main() {
	// Configuração via linha de comando
	configPath := flag.String("config", "configs/config.toml", "Caminho para o arquivo de configuração")
	debugMode := flag.Bool("debug", false, "Ativar modo de depuração")
	port := flag.Int("port", 0, "Porta para o serviço de descoberta (sobrescreve a configuração)")
	bootstrapPeers := flag.String("peers", "", "Lista de peers iniciais separados por vírgula")
	flag.Parse()

	// Carrega a configuração
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Printf("Aviso: %v. Usando configurações padrão.", err)
	}

	// Sobrescreve com argumentos de linha de comando, se fornecidos
	if *debugMode {
		config.UI.DebugMode = true
	}

	// Define a porta a ser usada (prioridade: linha de comando > arquivo de configuração > padrão)
	discoveryPort := config.Network.Port
	if *port > 0 {
		discoveryPort = *port
	}

	// Define o nickname inicial
	nickname = fmt.Sprintf("usuario%d", time.Now().Unix()%1000)

	// Adiciona o canal padrão
	channels = append(channels, "#general")
	activeChannel = "#general"

	// Processa a lista de peers iniciais
	var peersList []string
	if *bootstrapPeers != "" {
		peersList = strings.Split(*bootstrapPeers, ",")
	}

	// Cria o serviço de descoberta
	discoveryService, err := discovery.New(peersList, discoveryPort)
	if err != nil {
		log.Fatalf("Erro ao criar serviço de descoberta: %v", err)
	}

	// Inicia o serviço de descoberta em segundo plano
	if err := discoveryService.Start(); err != nil {
		log.Fatalf("Erro ao iniciar serviço de descoberta: %v", err)
	}
	defer discoveryService.Stop()

	// Cria a interface gráfica (usando Fyne)
	chatUI := ui.NewGUI()
	
	// Configura o modo de depuração
	chatUI.SetDebugMode(config.UI.DebugMode)

	// Define o handler de entrada
	chatUI.SetInputHandler(func(input string) {
		handleInput(input, chatUI, discoveryService)
	})

	// Adiciona o canal padrão
	chatUI.SetChannels([]string{"#general"})
	chatUI.SetActiveChannel("#general")

	// Contexto para gerenciar goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configura o handler de mensagens
	discoveryService.SetMessageHandler(func(msg discovery.Message) {
		// Processa a mensagem recebida de outro peer
		switch msg.Type {
		case discovery.TypeChatMessage:
			// Formata a mensagem
			timestamp := time.Now().Format("[2006-01-02 15:04:05]")
			formattedMsg := fmt.Sprintf("%s <%s> %s", timestamp, msg.Sender, msg.Content)
			
			// Adiciona a mensagem ao canal
			chatUI.AddMessageToChannel(msg.Channel, formattedMsg)
			
			// Log de depuração
			chatUI.AddLogMessage(fmt.Sprintf("Mensagem recebida de %s para canal %s: %s", 
				msg.Sender, msg.Channel, msg.Content))
		}
	})
	
	// Inicia o servidor TCP para receber conexões de outros peers
	go startTCPServer(discoveryService, chatUI)
	
	// Tenta conectar a todos os peers conhecidos
	go func() {
		// Aguarda um pouco para dar tempo de descobrir peers
		time.Sleep(2 * time.Second)
		
		// Conecta a todos os peers conhecidos
		chatUI.AddLogMessage("Tentando conectar a peers conhecidos...")
		peers := discoveryService.GetPeers()
		for _, peer := range peers {
			addr := peer.Addr.String()
			chatUI.AddLogMessage(fmt.Sprintf("Tentando conectar ao peer %s", addr))
			go func(addr string) {
				if err := discoveryService.ConnectToPeer(addr); err != nil {
					chatUI.AddLogMessage(fmt.Sprintf("Erro ao conectar ao peer %s: %v", addr, err))
				} else {
					chatUI.AddLogMessage(fmt.Sprintf("Conectado com sucesso ao peer %s", addr))
				}
			}(addr)
		}
		
		// Inicia uma rotina para reconectar periodicamente
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					discoveryService.ConnectToAllPeers()
				}
			}
		}()
	}()
	
	// Inicia o monitoramento de peers em segundo plano
	go monitorPeers(ctx, discoveryService, chatUI)

	// Exibe mensagem de boas-vindas
	chatUI.AddMessage(fmt.Sprintf("P2P-IRC iniciado na porta %d", config.Network.Port))
	chatUI.AddMessage("Digite /help para ver a lista de comandos disponíveis")

	// Inicia a interface do usuário
	if err := chatUI.Run(); err != nil {
		log.Fatalf("Erro ao executar interface: %v", err)
	}
}

// handleInput processa a entrada do usuário
func handleInput(input string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	// Verifica se é um comando
	if strings.HasPrefix(input, "/") {
		// Processa o comando
		parts := strings.SplitN(input, " ", 2)
		cmd := parts[0]
		
		switch cmd {
		case "/join":
			// Comando para entrar em um canal
			if len(parts) < 2 {
				chatUI.AddLogMessage("Uso: /join <canal>")
				return
			}
			
			channel := strings.TrimSpace(parts[1])
			if !strings.HasPrefix(channel, "#") {
				channel = "#" + channel
			}
			
			// Adiciona o canal à lista de canais
			chatUI.AddChannel(channel)
			
			// Adiciona o canal ao serviço de descoberta
			discoveryService.JoinChannel(channel)
			
			// Notifica o usuário
			chatUI.AddMessageToChannel(channel, fmt.Sprintf("[%s] Entrou no canal %s", 
				time.Now().Format("2006-01-02 15:04:05"), channel))
			
			// Define o canal como ativo
			chatUI.SetActiveChannel(channel)
			
			// Log de depuração
			chatUI.AddLogMessage(fmt.Sprintf("Entrou no canal %s", channel))
			
		case "/part", "/leave":
			// Comando para sair de um canal
			var channel string
			if len(parts) < 2 {
				// Se não especificou o canal, usa o canal ativo
				channel = chatUI.GetActiveChannel()
			} else {
				channel = strings.TrimSpace(parts[1])
				if !strings.HasPrefix(channel, "#") {
					channel = "#" + channel
				}
			}
			
			// Remove o canal da lista de canais
			chatUI.RemoveChannel(channel)
			
			// Remove o canal do serviço de descoberta
			discoveryService.PartChannel(channel)
			
			// Notifica o usuário
			chatUI.AddLogMessage(fmt.Sprintf("Saiu do canal %s", channel))
			
		case "/nick":
			// Comando para alterar o nickname
			if len(parts) < 2 {
				chatUI.AddLogMessage("Uso: /nick <nickname>")
				return
			}
			
			nickname = strings.TrimSpace(parts[1])
			
			// Notifica o usuário
			chatUI.AddLogMessage(fmt.Sprintf("Nickname alterado: %s", nickname))
			
			// Notifica os canais
			channels := chatUI.GetChannelList()
			for _, channel := range channels {
				chatUI.AddMessageToChannel(channel, fmt.Sprintf("[%s] Nickname alterado: %s", 
					time.Now().Format("2006-01-02 15:04:05"), nickname))
			}
			
		case "/msg":
			// Comando para enviar mensagem privada
			if len(parts) < 2 {
				chatUI.AddLogMessage("Uso: /msg <destinatário> <mensagem>")
				return
			}
			
			msgParts := strings.SplitN(parts[1], " ", 2)
			if len(msgParts) < 2 {
				chatUI.AddLogMessage("Uso: /msg <destinatário> <mensagem>")
				return
			}
			
			recipient := strings.TrimSpace(msgParts[0])
			message := strings.TrimSpace(msgParts[1])
			
			// Envia a mensagem privada
			// TODO: Implementar mensagens privadas
			
			// Notifica o usuário
			chatUI.AddLogMessage(fmt.Sprintf("Mensagem privada para %s: %s", recipient, message))
			
		case "/who":
			// Comando para listar usuários conectados
			// TODO: Implementar listagem de usuários
			
			// Temporariamente, lista os peers conectados
			peers := discoveryService.GetPeers()
			chatUI.AddLogMessage("Peers conectados:")
			for _, peer := range peers {
				chatUI.AddLogMessage(fmt.Sprintf("  - %s", peer.Addr.String()))
			}
			
		case "/peers":
			// Comando para listar peers conectados
			peers := discoveryService.GetPeers()
			chatUI.AddLogMessage("Peers conectados:")
			for _, peer := range peers {
				chatUI.AddLogMessage(fmt.Sprintf("  - %s", peer.Addr.String()))
			}
			
			// Lista peers por canal
			activeChannel := chatUI.GetActiveChannel()
			if activeChannel != "" {
				peersInChannel := discoveryService.GetPeersInChannel(activeChannel)
				chatUI.AddLogMessage(fmt.Sprintf("Peers no canal %s:", activeChannel))
				for _, peer := range peersInChannel {
					chatUI.AddLogMessage(fmt.Sprintf("  - %s", peer))
				}
			}
			
		case "/help":
			// Comando para exibir ajuda
			chatUI.AddLogMessage("Comandos disponíveis:")
			chatUI.AddLogMessage("  /join <canal> - Entra em um canal")
			chatUI.AddLogMessage("  /part [<canal>] - Sai de um canal (usa o canal ativo se não especificado)")
			chatUI.AddLogMessage("  /nick <nickname> - Altera o nickname")
			chatUI.AddLogMessage("  /msg <destinatário> <mensagem> - Envia mensagem privada")
			chatUI.AddLogMessage("  /who - Lista usuários conectados")
			chatUI.AddLogMessage("  /peers - Lista peers conectados")
			chatUI.AddLogMessage("  /quit - Encerra a aplicação")
			chatUI.AddLogMessage("  /help - Exibe esta ajuda")
			chatUI.AddLogMessage("")
			chatUI.AddLogMessage("Atalhos de teclado:")
			chatUI.AddLogMessage("  Ctrl+C: Encerra a aplicação")
			
		case "/quit":
			// Comando para encerrar a aplicação
			os.Exit(0)
			
		case "/sync":
			// Comando para forçar a sincronização de canais
			chatUI.AddLogMessage("Forçando sincronização de canais e peers...")
			
			// Força a sincronização de canais
			discoveryService.SyncPeers()
			
			// Corrige problemas de sincronização
			discoveryService.FixChannelSync()
			
			// Exibe informações de depuração
			discoveryService.DebugConnections()
			
			chatUI.AddLogMessage("Sincronização concluída!")
			
		case "/debug":
			// Comando para exibir informações de depuração
			chatUI.AddLogMessage("Informações de depuração:")
			
			// Exibe informações sobre as conexões
			discoveryService.DebugConnections()
			
			// Lista peers por canal
			activeChannel := chatUI.GetActiveChannel()
			if activeChannel != "" {
				peersInChannel := discoveryService.GetPeersInChannel(activeChannel)
				chatUI.AddLogMessage(fmt.Sprintf("Peers no canal %s:", activeChannel))
				for _, peer := range peersInChannel {
					chatUI.AddLogMessage(fmt.Sprintf("  - %s", peer))
				}
			}
			
		default:
			// Comando desconhecido
			chatUI.AddLogMessage(fmt.Sprintf("Comando desconhecido: %s", cmd))
		}
	} else {
		// Não é um comando, envia como mensagem para o canal ativo
		activeChannel := chatUI.GetActiveChannel()
		if activeChannel == "" {
			chatUI.AddLogMessage("Você precisa entrar em um canal primeiro")
			return
		}
		
		// Adiciona a mensagem ao canal
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		formattedMsg := fmt.Sprintf("%s <%s> %s", timestamp, nickname, input)
		chatUI.AddMessageToChannel(activeChannel, formattedMsg)
		
		// Envia a mensagem para os peers
		discoveryService.SendChatMessageToChannel(activeChannel, input, nickname)
		
		// Log de depuração
		chatUI.AddLogMessage(fmt.Sprintf("Mensagem enviada para canal %s: %s", activeChannel, input))
	}
}

// Funções auxiliares para processar comandos

func showHelp(chatUI ui.Interface) {
	chatUI.AddMessage("Comandos disponíveis:")
	chatUI.AddMessage("- /nick <novo> - Define seu nickname")
	chatUI.AddMessage("- /join <#canal> - Entra em um canal")
	chatUI.AddMessage("- /part [#canal] - Sai de um canal (usa o atual se não especificado)")
	chatUI.AddMessage("- /msg <destino> <mensagem> - Envia mensagem privada")
	chatUI.AddMessage("- /who - Lista usuários conectados")
	chatUI.AddMessage("- /peers - Lista peers conectados")
	chatUI.AddMessage("- /quit - Encerra a aplicação")
	chatUI.AddMessage("- /help - Exibe esta ajuda")
	chatUI.AddMessage("")
	chatUI.AddMessage("Atalhos de teclado:")
	chatUI.AddMessage("- Ctrl+C: Encerra a aplicação")
}

func handleNickCommand(args string, chatUI ui.Interface) {
	if args == "" {
		chatUI.AddMessage("Uso: /nick <novo>")
		return
	}
	
	oldNick := nickname
	nickname = args
	chatUI.AddMessage(fmt.Sprintf("Nickname alterado: %s -> %s", oldNick, nickname))
	chatUI.AddLogMessage(fmt.Sprintf("Nickname alterado: %s -> %s", oldNick, nickname))
}

func handleJoinCommand(args string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	if args == "" {
		chatUI.AddMessage("Uso: /join <#canal>")
		return
	}
	
	// Adiciona # se não estiver presente
	channel := args
	if !strings.HasPrefix(channel, "#") {
		channel = "#" + channel
	}
	
	// Verifica se já está no canal
	for _, ch := range channels {
		if ch == channel {
			chatUI.AddMessage(fmt.Sprintf("Você já está no canal %s", channel))
			chatUI.SetActiveChannel(channel)
			return
		}
	}
	
	// Adiciona o canal à lista
	channels = append(channels, channel)
	
	// Define o canal como ativo
	chatUI.SetActiveChannel(channel)
	
	// Atualiza a lista de canais na interface
	chatUI.SetChannels(channels)
	
	// Adiciona mensagem de entrada no canal
	timestamp := time.Now().Format("[2006-01-02 15:04:05]")
	chatUI.AddMessageToChannel(channel, fmt.Sprintf("%s * %s entrou no canal", timestamp, nickname))
	
	// Notifica o serviço de descoberta que entramos no canal
	discoveryService.JoinChannel(channel)
	
	chatUI.AddLogMessage(fmt.Sprintf("Entrou no canal: %s", channel))
}

func handlePartCommand(args string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	// Se não especificar canal, usa o ativo
	channel := chatUI.GetActiveChannel()
	if args != "" {
		// Adiciona # se não estiver presente
		if !strings.HasPrefix(args, "#") {
			args = "#" + args
		}
		channel = args
	}
	
	// Verifica se está no canal
	found := false
	var newChannels []string
	for _, ch := range channels {
		if ch == channel {
			found = true
		} else {
			newChannels = append(newChannels, ch)
		}
	}
	
	if !found {
		chatUI.AddMessage(fmt.Sprintf("Você não está no canal %s", channel))
		return
	}
	
	// Adiciona mensagem de saída no canal
	timestamp := time.Now().Format("[2006-01-02 15:04:05]")
	chatUI.AddMessageToChannel(channel, fmt.Sprintf("%s * %s saiu do canal", timestamp, nickname))
	
	// Notifica o serviço de descoberta que saímos do canal
	discoveryService.PartChannel(channel)
	
	// Atualiza a lista de canais
	channels = newChannels
	
	// Se saiu do canal ativo, muda para outro canal
	activeChannel := chatUI.GetActiveChannel()
	if channel == activeChannel {
		if len(channels) > 0 {
			activeChannel = channels[0]
			chatUI.SetActiveChannel(activeChannel)
		}
	}
	
	chatUI.SetChannels(channels)
}

func handleMsgCommand(args string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		chatUI.AddMessage("Uso: /msg <usuário|#canal> <mensagem>")
		return
	}
	
	target := parts[0]
	message := parts[1]
	
	// Adiciona # se for canal e não estiver presente
	if !strings.HasPrefix(target, "@") && !strings.HasPrefix(target, "#") && strings.ToLower(target) != "server" {
		if strings.HasPrefix(target, "#") {
			// É um canal
		} else {
			// É um usuário, adiciona @ para indicar mensagem privada
			target = "@" + target
		}
	}
	
	if strings.HasPrefix(target, "#") {
		// Mensagem para canal
		channel := target
		
		// Verifica se está no canal
		found := false
		for _, ch := range channels {
			if ch == channel {
				found = true
				break
			}
		}
		
		if !found {
			chatUI.AddMessage(fmt.Sprintf("Você não está no canal %s", channel))
			return
		}
		
		// Adiciona a mensagem localmente
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		formattedMsg := fmt.Sprintf("%s <%s> %s", timestamp, nickname, message)
		chatUI.AddMessageToChannel(channel, formattedMsg)
		
		// Envia a mensagem para todos os peers no canal
		discoveryService.SendChatMessage(channel, message, nickname)
		
		chatUI.AddLogMessage(fmt.Sprintf("Mensagem enviada para canal %s: %s", channel, message))
	} else {
		// Mensagem privada para usuário
		targetNick := strings.TrimPrefix(target, "@")
		
		// Adiciona a mensagem localmente
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		formattedMsg := fmt.Sprintf("%s [Mensagem privada para %s]: %s", timestamp, targetNick, message)
		chatUI.AddMessage(formattedMsg)
		chatUI.AddLogMessage(fmt.Sprintf("Mensagem privada enviada para %s: %s", targetNick, message))
	}
}

func handleWhoCommand(chatUI ui.Interface, discoveryService *discovery.Discovery) {
	// Lista usuários localmente
	chatUI.AddMessage("Usuários conectados:")
	chatUI.AddMessage(fmt.Sprintf("- %s (você)", nickname))
	
	// Obtém a lista de peers do serviço de descoberta
	peers := discoveryService.GetPeers()
	for _, peer := range peers {
		chatUI.AddMessage(fmt.Sprintf("- Peer: %s", peer.Addr))
	}
}

func handlePeersCommand(chatUI ui.Interface, discoveryService *discovery.Discovery) {
	// Obtém a lista de peers do serviço de descoberta
	peers := discoveryService.GetPeers()
	
	if len(peers) == 0 {
		chatUI.AddMessage("Nenhum peer conectado")
		return
	}
	
	chatUI.AddMessage("Peers conectados:")
	for _, peer := range peers {
		chatUI.AddMessage(fmt.Sprintf("- ID: %s, Endereço: %s", peer.ID, peer.Addr))
	}
}

func handleQuitCommand(chatUI ui.Interface) {
	// Exibe mensagem de despedida
	chatUI.AddMessage("Encerrando aplicação...")
	
	// Aguarda um pouco para a mensagem ser exibida
	time.Sleep(500 * time.Millisecond)
	
	// Encerra a aplicação
	os.Exit(0)
}

func monitorPeers(ctx context.Context, discoveryService *discovery.Discovery, chatUI ui.Interface) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Atualiza a lista de peers na interface
			peers := discoveryService.GetPeers()
			var peerNames []string
			for _, p := range peers {
				peerNames = append(peerNames, fmt.Sprintf("%s", p.Addr))
			}
			chatUI.SetPeers(peerNames)
			
			// Atualiza as métricas
			metrics := discoveryService.GetMetrics()
			chatUI.AddLogMessage(fmt.Sprintf("Métricas: Peers ativos: %d, Total descobertos: %d", 
				metrics.ActivePeers, metrics.TotalDiscovered))
		}
	}
}

func startTCPServer(discoveryService *discovery.Discovery, chatUI ui.Interface) {
	// Cria o listener na porta configurada
	addr := fmt.Sprintf(":%d", discoveryService.GetPort())
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		chatUI.AddLogMessage(fmt.Sprintf("Erro ao iniciar servidor TCP: %v", err))
		return
	}
	defer ln.Close()

	chatUI.AddLogMessage(fmt.Sprintf("Servidor TCP iniciado na porta %d", discoveryService.GetPort()))

	for {
		// Aceita novas conexões
		conn, err := ln.Accept()
		if err != nil {
			chatUI.AddLogMessage(fmt.Sprintf("Erro ao aceitar conexão: %v", err))
			continue
		}

		// Cria uma nova conexão de peer
		peerConn := discovery.NewPeerConnection(conn)
		
		// Adiciona à lista de conexões (o serviço de descoberta vai gerenciar a conexão)
		remoteAddr := conn.RemoteAddr().String()
		chatUI.AddLogMessage(fmt.Sprintf("Nova conexão recebida de %s", remoteAddr))
		
		// O serviço de descoberta vai gerenciar a leitura de mensagens
		go discoveryService.HandleNewConnection(remoteAddr, peerConn)
	}
}
