package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/peder1981/p2p-irc/internal/discovery"
	"github.com/peder1981/p2p-irc/internal/irc"
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
	meshUI           *ui.MeshUI
)

// Config representa a estrutura de configuração do aplicativo
type Config struct {
	Network NetworkConfig `toml:"network"`
	UI      UIConfig      `toml:"ui"`
	Mesh    mesh.MeshConfig `toml:"mesh"`
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

// Carrega a configuração do arquivo
func loadConfig(configPath string) (*Config, error) {
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
		Mesh: mesh.MeshConfig{
			Enabled:           false,
			Port:              8081,
			EnableEncryption:  false,
			EnableCompression: false,
			MeshNetworkID:     "p2p-irc-mesh",
			LogLevel:          "info",
		},
	}

	if _, err := os.Stat(configPath); err == nil {
		// Usamos uma struct temporária para decodificar, para evitar o campo BitchatPath
		type TomlMeshConfig struct {
			Enabled           bool   `toml:"enabled"`
			Port              int    `toml:"port"`
			EnableEncryption  bool   `toml:"enableEncryption"`
			EnableCompression bool   `toml:"enableCompression"`
			NetworkID         string `toml:"networkId"`
			LogLevel          string `toml:"logLevel"`
		}
		tempConfig := struct {
			Mesh TomlMeshConfig `toml:"mesh"`
		}{}

		if _, err := toml.DecodeFile(configPath, &tempConfig); err != nil {
			return nil, fmt.Errorf("erro ao decodificar arquivo de configuração: %v", err)
		}

		// Mapeia da struct temporária para a struct de configuração principal
		config.Mesh.Enabled = tempConfig.Mesh.Enabled
		config.Mesh.Port = tempConfig.Mesh.Port
		config.Mesh.EnableEncryption = tempConfig.Mesh.EnableEncryption
		config.Mesh.EnableCompression = tempConfig.Mesh.EnableCompression
		config.Mesh.MeshNetworkID = tempConfig.Mesh.NetworkID
		config.Mesh.LogLevel = tempConfig.Mesh.LogLevel
	}

	return config, nil
}

func main() {
	configFile := flag.String("config", "configs/config.toml", "Caminho para o arquivo de configuração TOML")
	nicknameFlag := flag.String("nick", "", "Seu apelido no IRC")
	channelFlag := flag.String("channel", "#p2p-irc", "Canal para entrar")
	flag.Parse()

	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}

	nickname := *nicknameFlag
	if nickname == "" {
		rand.Seed(time.Now().UnixNano())
		nickname = fmt.Sprintf("user%d", rand.Intn(10000))
	}

	discoveryService, err := discovery.New(nil, cfg.Network.Port)
	if err != nil {
		log.Fatalf("Erro ao inicializar o serviço de descoberta: %v", err)
	}

	// Inicia o serviço de descoberta em uma goroutine para não bloquear a UI
	go func() {
		if err := discoveryService.Start(); err != nil {
			log.Printf("[ERROR] Falha ao iniciar o serviço de descoberta: %v", err)
		}
	}()

	gui := ui.NewGUI(discoveryService)
	gui.SetChannels([]string{*channelFlag})
	gui.SetDebugMode(cfg.UI.DebugMode)

	client := irc.NewClient(nickname, discoveryService, gui)

	meshMessageHandler := func(senderID, channel, sender, content string) {
		targetChannel := channel
		if targetChannel == "" {
			targetChannel = "#mesh"
		}
		gui.AddChannel(targetChannel)
		msg := fmt.Sprintf("[mesh] <%s>: %s", sender, content)
		gui.AddMessageToChannel(targetChannel, msg)
	}

	meshManager := mesh.NewMeshManager(cfg.Mesh, discoveryService, meshMessageHandler)

	if cfg.Mesh.Enabled {
		gui.AddLogMessage("Mesh habilitada na configuração, iniciando automaticamente...")
		if err := meshManager.Start(); err != nil {
			gui.AddLogMessage(fmt.Sprintf("Falha ao iniciar a mesh: %s", err.Error()))
		} else {
			gui.AddLogMessage("Serviço de mesh iniciado com sucesso.")
			go startMeshPeerUpdater(meshManager, gui)
		}
	}

	gui.SetInputHandler(func(command string) {
		if err := handleCommand(command, client, gui, discoveryService, meshManager); err != nil {
			gui.AddMessage(fmt.Sprintf("Erro: %v", err))
		}
	})

	
	go startTCPServer(discoveryService, gui)

	gui.Run()
}

// startMeshPeerUpdater atualiza periodicamente a lista de peers da mesh na UI.
// startMeshPeerUpdater atualiza periodicamente a lista de peers da mesh na UI.
func startMeshPeerUpdater(meshManager *mesh.MeshManager, gui *ui.GUI) {

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {

		peers := meshManager.GetPeers()


		peerInfos := make([]ui.PeerInfo, len(peers))
		for i, p := range peers {
			peerInfos[i] = ui.PeerInfo{
				ID:      p.ID,
				Address: p.Address,
			}
		}


		gui.RunOnMain(func() {

			if meshUI := gui.GetMeshUI(); meshUI != nil {

				meshUI.UpdatePeerList(peerInfos)

			} else {

			}
		})

	}
}

func handleCommand(command string, client *irc.Client, gui *ui.GUI, discoveryService *discovery.Discovery, meshManager *mesh.MeshManager) error {
	if !strings.HasPrefix(command, "/") {
		activeChannel := gui.GetActiveChannel()
		if activeChannel == "" {
			return fmt.Errorf("nenhum canal ativo. Use /join #canal")
		}
		// Adiciona a mensagem à UI localmente antes de enviar
		myNick := client.GetNickname()
		gui.AddMessageToChannel(activeChannel, fmt.Sprintf("<%s> %s", myNick, command))
		return client.HandleCommand(fmt.Sprintf("PRIVMSG %s :%s", activeChannel, command))
	}

	parts := strings.Fields(command)
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/join":
		if len(args) < 1 {
			return fmt.Errorf("uso: /join <#canal>")
		}
		channel := strings.ToLower(args[0])
		gui.AddChannel(channel)
		gui.SetActiveChannel(channel)
		// Reconstrói o comando com o nome normalizado para o cliente IRC
		return client.HandleCommand(fmt.Sprintf("/join %s", channel))
	case "/mesh":
		if len(args) == 0 {
			return fmt.Errorf("subcomando mesh ausente. Uso: /mesh [enable|disable|status|peers|send]")
		}
		switch args[0] {
		case "enable":
			gui.AddLogMessage("Iniciando rede mesh em segundo plano...")
			go func() {

				if err := meshManager.Start(); err != nil {
					gui.RunOnMain(func() {
						gui.AddLogMessage(fmt.Sprintf("Falha ao ativar a rede mesh: %v", err))
					})
					return
				}

				// Conecta o logger da mesh à UI
				if integration := meshManager.GetIntegration(); integration != nil {
					integration.SetLogger(gui.GetMeshUI().Log)
				}


				gui.RunOnMain(func() {
					gui.AddLogMessage("Rede mesh ativada com sucesso.")
				})

				go startMeshPeerUpdater(meshManager, gui)
			}()
		case "disable":
			gui.AddLogMessage("Parando a rede mesh...")
			if err := meshManager.Stop(); err != nil {
				return fmt.Errorf("falha ao desativar a rede mesh: %w", err)
			}
			gui.AddLogMessage("Rede mesh desativada.")
		case "status":
			if meshManager.IsRunning() {
				gui.AddLogMessage("Status da Mesh: Ativo")
			} else {
				gui.AddLogMessage("Status da Mesh: Inativo")
			}
		case "peers":
			if !meshManager.IsRunning() {
				return fmt.Errorf("a rede mesh não está ativa")
			}
			peers := meshManager.GetPeers()
			if len(peers) == 0 {
				gui.AddLogMessage("Nenhum peer encontrado na rede mesh.")
			} else {
				gui.AddLogMessage("Peers na rede mesh:")
				for _, peer := range peers {
					gui.AddLogMessage(fmt.Sprintf("- %s (%s)", peer.ID, peer.Address))
				}
			}
		case "send":
			if len(args) < 3 {
				return fmt.Errorf("uso: /mesh send <peer_id> <mensagem>")
			}
			peerID := args[1]
			message := strings.Join(args[2:], " ")
			if err := meshManager.GetIntegration().SendMessage(peerID, message); err != nil {
				return fmt.Errorf("erro ao enviar mensagem via mesh: %w", err)
			}
			gui.AddLogMessage(fmt.Sprintf("Mensagem enviada para %s via mesh.", peerID))
		default:
			return fmt.Errorf("subcomando mesh desconhecido: %s", args[0])
		}
	case "/quit":
		gui.Stop()
	default:
		// Delega para o handler de comandos do cliente IRC para outros comandos como /join, /part, etc.
		return client.HandleCommand(command)
	}
	return nil
}

func startTCPServer(discoveryService *discovery.Discovery, gui ui.Interface) {
	addr := fmt.Sprintf(":%d", discoveryService.GetPort())
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		gui.AddLogMessage(fmt.Sprintf("Erro ao iniciar servidor TCP: %v", err))
		return
	}
	defer ln.Close()

	gui.AddLogMessage(fmt.Sprintf("Servidor TCP iniciado na porta %d", discoveryService.GetPort()))

	for {
		conn, err := ln.Accept()
		if err != nil {
			gui.AddLogMessage(fmt.Sprintf("Erro ao aceitar conexão: %v", err))
			continue
		}
		go discoveryService.HandleNewConnection(conn.RemoteAddr().String(), discovery.NewPeerConnection(conn))
	}
}
