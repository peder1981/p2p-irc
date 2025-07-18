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
	"github.com/peder1981/p2p-irc/internal/mesh"
	"github.com/peder1981/p2p-irc/internal/ui"
)

// Configuração global
var (
	config struct {
		Nickname string
		Port     int
	}
	nickname      string // Nome do usuário
	channels      []string
	activeChannel string

	// Componentes mesh
	discoveryService *discovery.Discovery
	meshIntegration  *mesh.MeshIntegration
	bitchatBridge    *mesh.BitchatBridge
	meshUI           *ui.MeshUI
)

// Config representa a estrutura de configuração do aplicativo
type Config struct {
	Network NetworkConfig `toml:"network"`
	UI      UIConfig      `toml:"ui"`
	Mesh    MeshConfig    `toml:"mesh"`
}

// NetworkConfig contém configurações de rede
type NetworkConfig struct {
	Port        int      `toml:"port"`
	StunServers []string `toml:"stunServers"`
}

// UIConfig contém configurações da interface do usuário
type UIConfig struct {
	DebugMode   bool   `toml:"debugMode"`
	MaxLogLines int    `toml:"maxLogLines"`
	HistoryDir  string `toml:"historyDir"`
}

// MeshConfig contém configurações para funcionalidades mesh
type MeshConfig struct {
	Enabled           bool   `toml:"enabled"`
	BitchatPath       string `toml:"bitchatPath"`
	EnableEncryption  bool   `toml:"enableEncryption"`
	EnableCompression bool   `toml:"enableCompression"`
	NetworkID         string `toml:"networkId"`
	LogLevel          string `toml:"logLevel"`
	AutoFallback      bool   `toml:"autoFallback"`
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
		Mesh: MeshConfig{
			Enabled:           false,
			BitchatPath:       "",
			EnableEncryption:  false,
			EnableCompression: false,
			NetworkID:         "",
			LogLevel:          "info",
			AutoFallback:      false,
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
	enableMesh := flag.Bool("mesh", false, "Ativar funcionalidades mesh")
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
	if *enableMesh {
		config.Mesh.Enabled = true
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
	discoveryService, err = discovery.New(peersList, discoveryPort)
	if err != nil {
		log.Fatalf("Erro ao criar serviço de descoberta: %v", err)
	}

	// Inicia o serviço de descoberta em segundo plano
	if err := discoveryService.Start(); err != nil {
		log.Fatalf("Erro ao iniciar serviço de descoberta: %v", err)
	}
	defer discoveryService.Stop()

	// Inicializa interface do usuário
	var chatUI ui.Interface
	if config.Mesh.Enabled {
		chatUI = ui.NewGUI()
	} else {
		chatUI = ui.NewGUI()
	}

	// Configura o modo de depuração
	chatUI.SetDebugMode(config.UI.DebugMode)

	// Inicializa componentes mesh se habilitado
	if config.Mesh.Enabled {
		if err := initializeMesh(config, discoveryService); err != nil {
			log.Printf("Aviso: Erro ao inicializar mesh: %v. Continuando sem mesh.", err)
		} else {
			log.Println("Funcionalidades mesh inicializadas com sucesso")
		}
	}

	// Define o handler de entrada
	chatUI.SetInputHandler(func(input string) {
		handleInput(input, chatUI, discoveryService)
	})

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
			if config.UI.DebugMode {
				chatUI.AddLogMessage(fmt.Sprintf("Mensagem recebida de %s no canal %s: %s", msg.Sender, msg.Channel, msg.Content))
			}

			// Envia para rede mesh se habilitado
			if config.Mesh.Enabled && bitchatBridge != nil {
				bitchatBridge.SendToBitchat(msg)
			}

		case discovery.TypeJoinChannel:
			if config.UI.DebugMode {
				chatUI.AddLogMessage(fmt.Sprintf("Peer %s entrou no canal %s", msg.Sender, msg.Channel))
			}

		case discovery.TypePartChannel:
			if config.UI.DebugMode {
				chatUI.AddLogMessage(fmt.Sprintf("Peer %s saiu do canal %s", msg.Sender, msg.Channel))
			}

		case discovery.TypePing:
			if config.UI.DebugMode {
				chatUI.AddLogMessage(fmt.Sprintf("Ping recebido de %s", msg.Sender))
			}

		case discovery.TypePong:
			if config.UI.DebugMode {
				chatUI.AddLogMessage(fmt.Sprintf("Pong recebido de %s", msg.Sender))
			}
		}
	})

	// Inicia o monitoramento de peers
	go monitorPeers(ctx, discoveryService, chatUI)

	// Inicia o servidor TCP
	go startTCPServer(discoveryService, chatUI)

	// Mensagem de boas-vindas
	chatUI.AddMessage("=== P2P-IRC ===")
	chatUI.AddMessage(fmt.Sprintf("Bem-vindo, %s!", nickname))
	chatUI.AddMessage(fmt.Sprintf("Porta de descoberta: %d", discoveryService.GetPort()))
	chatUI.AddMessage(fmt.Sprintf("ID da instância: %s", discoveryService.GetInstanceID()))

	if config.Mesh.Enabled {
		chatUI.AddMessage("Funcionalidades mesh ativadas")
		if bitchatBridge != nil && bitchatBridge.IsConnected() {
			chatUI.AddMessage("Conectado à rede Bitchat")
		}
	}

	chatUI.AddMessage("Digite /help para ver os comandos disponíveis")
	chatUI.AddMessage("---")

	// Entra no canal padrão
	discoveryService.JoinChannel(activeChannel)

	// Exibe a interface
	chatUI.Run()
}

// initializeMesh inicializa os componentes mesh
func initializeMesh(config *Config, discoveryService *discovery.Discovery) error {
	// Cria integração mesh
	meshIntegration = mesh.NewMeshIntegration(discoveryService)

	// Configura ponte Bitchat se habilitado
	if config.Mesh.BitchatPath != "" {
		bitchatConfig := mesh.BitchatConfig{
			BitchatBinaryPath: config.Mesh.BitchatPath,
			EnableEncryption:  config.Mesh.EnableEncryption,
			EnableCompression: config.Mesh.EnableCompression,
			MeshNetworkID:     config.Mesh.NetworkID,
			LogLevel:          config.Mesh.LogLevel,
		}

		bitchatBridge = mesh.NewBitchatBridge(bitchatConfig, meshIntegration)

		// Inicia ponte Bitchat
		if err := bitchatBridge.Start(); err != nil {
			log.Printf("Aviso: Erro ao iniciar ponte Bitchat: %v", err)
		}
	}

	// Cria interface mesh
	meshUI = ui.NewMeshUI()

	// Inicia integração mesh
	return meshIntegration.Start()
}

func handleInput(input string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	input = strings.TrimSpace(input)

	if input == "" {
		return
	}

	// Verifica se é um comando
	if strings.HasPrefix(input, "/") {
		parts := strings.SplitN(input[1:], " ", 2)
		command := strings.ToLower(parts[0])
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}

		switch command {
		case "help":
			showHelp(chatUI)
		case "nick":
			handleNickCommand(args, chatUI)
		case "join":
			handleJoinCommand(args, chatUI, discoveryService)
		case "part":
			handlePartCommand(args, chatUI, discoveryService)
		case "msg":
			handleMsgCommand(args, chatUI, discoveryService)
		case "who":
			handleWhoCommand(chatUI, discoveryService)
		case "peers":
			handlePeersCommand(chatUI, discoveryService)
		case "quit":
			handleQuitCommand(chatUI)
		case "mesh":
			handleMeshCommand(args, chatUI)
		case "meshstatus":
			handleMeshStatusCommand(chatUI)
		default:
			chatUI.AddMessage(fmt.Sprintf("Comando desconhecido: %s. Digite /help para ver os comandos disponíveis.", command))
		}
	} else {
		// Mensagem normal - envia para o canal ativo
		if activeChannel == "" {
			chatUI.AddMessage("Você não está em nenhum canal. Use /join #canal para entrar em um canal.")
			return
		}

		// Adiciona a mensagem localmente
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		formattedMsg := fmt.Sprintf("%s <%s> %s", timestamp, nickname, input)
		chatUI.AddMessageToChannel(activeChannel, formattedMsg)

		// Envia a mensagem para todos os peers no canal
		discoveryService.SendChatMessage(activeChannel, input, nickname)

		// Log de depuração
		chatUI.AddLogMessage(fmt.Sprintf("Mensagem enviada para %s: %s", activeChannel, input))
	}
}

func showHelp(chatUI ui.Interface) {
	chatUI.AddMessage("=== Comandos Disponíveis ===")
	chatUI.AddMessage("/nick <nome>          - Define seu nickname")
	chatUI.AddMessage("/join <#canal>        - Entra em um canal")
	chatUI.AddMessage("/part [#canal]        - Sai do canal atual ou especificado")
	chatUI.AddMessage("/msg <usuário|#canal> <mensagem> - Envia mensagem privada")
	chatUI.AddMessage("/who                  - Lista usuários na rede")
	chatUI.AddMessage("/peers                - Lista todos os peers conectados")
	chatUI.AddMessage("/mesh [on|off]        - Alterna funcionalidades mesh")
	chatUI.AddMessage("/meshstatus           - Mostra status da rede mesh")
	chatUI.AddMessage("/quit                 - Encerra a aplicação")
	chatUI.AddMessage("/help                 - Exibe esta ajuda")
	chatUI.AddMessage("========================")
}

// handleNickCommand gerencia o comando /nick
func handleNickCommand(args string, chatUI ui.Interface) {
	if args == "" {
		chatUI.AddMessage("Uso: /nick <nome>")
		return
	}

	oldNick := nickname
	nickname = args
	chatUI.AddMessage(fmt.Sprintf("Nickname alterado de %s para %s", oldNick, nickname))
}

// handleJoinCommand gerencia o comando /join
func handleJoinCommand(args string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	if args == "" {
		chatUI.AddMessage("Uso: /join <#canal>")
		return
	}

	if !strings.HasPrefix(args, "#") {
		args = "#" + args
	}

	activeChannel = args
	discoveryService.JoinChannel(args)
	chatUI.AddMessage(fmt.Sprintf("Entrou no canal %s", args))
}

// handlePartCommand gerencia o comando /part
func handlePartCommand(args string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	channel := args
	if channel == "" {
		channel = activeChannel
	}

	if channel == "" {
		chatUI.AddMessage("Você não está em nenhum canal")
		return
	}

	discoveryService.PartChannel(channel)
	chatUI.AddMessage(fmt.Sprintf("Saiu do canal %s", channel))

	if channel == activeChannel {
		activeChannel = ""
	}
}

// handleMsgCommand gerencia o comando /msg
func handleMsgCommand(args string, chatUI ui.Interface, discoveryService *discovery.Discovery) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		chatUI.AddMessage("Uso: /msg <usuário|#canal> <mensagem>")
		return
	}

	target := parts[0]
	message := parts[1]

	if strings.HasPrefix(target, "#") {
		// Mensagem para canal
		discoveryService.SendChatMessage(target, message, nickname)
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		formattedMsg := fmt.Sprintf("%s <%s> %s", timestamp, nickname, message)
		chatUI.AddMessageToChannel(target, formattedMsg)
	} else {
		// Mensagem privada
		chatUI.AddMessage(fmt.Sprintf("Mensagem privada para %s: %s", target, message))
	}
}

// handleWhoCommand gerencia o comando /who
func handleWhoCommand(chatUI ui.Interface, discoveryService *discovery.Discovery) {
	peers := discoveryService.GetPeers()
	chatUI.AddMessage("=== Usuários na Rede ===")
	for _, peer := range peers {
		chatUI.AddMessage(fmt.Sprintf("- %s", peer.Addr))
	}
	chatUI.AddMessage("========================")
}

// handlePeersCommand gerencia o comando /peers
func handlePeersCommand(chatUI ui.Interface, discoveryService *discovery.Discovery) {
	peers := discoveryService.GetPeers()
	chatUI.AddMessage("=== Peers Conectados ===")
	for _, peer := range peers {
		chatUI.AddMessage(fmt.Sprintf("- %s", peer.Addr))
	}
	chatUI.AddMessage("========================")
}

// handleQuitCommand gerencia o comando /quit
func handleQuitCommand(chatUI ui.Interface) {
	chatUI.AddMessage("Encerrando aplicação...")
	os.Exit(0)
}

// handleMeshCommand gerencia comandos relacionados ao mesh
func handleMeshCommand(args string, chatUI ui.Interface) {
	args = strings.TrimSpace(args)

	switch args {
	case "enable", "on":
		// Inicializa componentes mesh se ainda não estiverem inicializados
		if meshIntegration == nil {
			// Carrega a configuração atual
			configPath := "configs/config.toml"
			config, err := loadConfig(configPath)
			if err != nil {
				chatUI.AddMessage(fmt.Sprintf("Erro ao carregar configuração: %v", err))
				return
			}
			
			// Habilita mesh na configuração
			config.Mesh.Enabled = true
			
			// Inicializa componentes mesh
			if err := initializeMesh(config, discoveryService); err != nil {
				chatUI.AddMessage(fmt.Sprintf("Erro ao inicializar mesh: %v", err))
				return
			}
			
			chatUI.AddMessage("Componentes mesh inicializados com sucesso")
		}
		
		chatUI.AddMessage("Funcionalidades mesh ativadas")
		if meshUI != nil {
			meshUI.Show()
		}
	case "disable", "off":
		if meshIntegration == nil {
			chatUI.AddMessage("Funcionalidades mesh não estão disponíveis")
			return
		}
		
		chatUI.AddMessage("Funcionalidades mesh desativadas")
		if meshUI != nil {
			meshUI.Hide()
		}
	case "status", "":
		handleMeshStatusCommand(chatUI)
	case "peers":
		handleMeshPeersCommand(chatUI)
	default:
		chatUI.AddMessage("Uso: /mesh [enable|disable|status|peers]")
	}
}

// handleMeshPeersCommand mostra peers mesh
func handleMeshPeersCommand(chatUI ui.Interface) {
	if meshIntegration == nil {
		chatUI.AddMessage("Funcionalidades mesh não estão disponíveis")
		return
	}

	peers := meshIntegration.GetPeers()
	chatUI.AddMessage("=== Peers Mesh ===")
	for _, peer := range peers {
		status := "Desconectado"
		if time.Since(peer.LastActivity) < 2*time.Minute {
			status = "Conectado"
		}
		chatUI.AddMessage(fmt.Sprintf("- %s (%s)", peer.ID, status))
	}
	chatUI.AddMessage("==================")
}

// handleMeshStatusCommand mostra o status da rede mesh
func handleMeshStatusCommand(chatUI ui.Interface) {
	if meshIntegration == nil {
		chatUI.AddMessage("Funcionalidades mesh não estão disponíveis")
		return
	}

	peers := meshIntegration.GetPeers()
	metrics := meshIntegration.GetMetrics()

	chatUI.AddMessage("=== Status da Rede Mesh ===")
	chatUI.AddMessage(fmt.Sprintf("Peers ativos: %d", len(peers)))
	chatUI.AddMessage(fmt.Sprintf("Total descobertos: %d", metrics.TotalPeersDiscovered))
	chatUI.AddMessage(fmt.Sprintf("Mensagens enviadas: %d", metrics.MessagesSent))
	chatUI.AddMessage(fmt.Sprintf("Mensagens recebidas: %d", metrics.MessagesReceived))
	chatUI.AddMessage(fmt.Sprintf("Tempo ativo: %v", metrics.NetworkUptime))

	if bitchatBridge != nil {
		status := bitchatBridge.GetStatus()
		if connected, ok := status["connected"].(bool); ok {
			chatUI.AddMessage(fmt.Sprintf("Bitchat conectado: %v", connected))
		}
		if path, ok := status["bitchat_path"].(string); ok {
			chatUI.AddMessage(fmt.Sprintf("Caminho Bitchat: %s", path))
		}
	}

	chatUI.AddMessage("===========================")
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
