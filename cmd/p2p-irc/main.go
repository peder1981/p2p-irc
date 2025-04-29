package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/peder1981/p2p-irc/internal/discovery"
	"github.com/peder1981/p2p-irc/internal/ui"
)

// Configuração global
var (
	nickname      string
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

	// Inicia o serviço de descoberta
	if err := discoveryService.Start(); err != nil {
		log.Fatalf("Erro ao iniciar serviço de descoberta: %v", err)
	}
	defer discoveryService.Stop()

	// Cria a interface unificada
	chatUI := ui.NewUnifiedUI()
	
	// Configura o modo de depuração
	chatUI.SetDebugMode(config.UI.DebugMode)

	// Define o handler de entrada
	chatUI.SetInputHandler(func(input string) {
		handleInput(input, chatUI, discoveryService)
	})

	// Adiciona o canal padrão
	chatUI.SetChannels([]string{"#general"})
	chatUI.SetActiveChannel("#general")

	// Inicia a descoberta de peers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Adiciona mensagem de boas-vindas
	chatUI.AddMessage("[yellow]Bem-vindo ao P2P-IRC![white]")
	chatUI.AddMessage(fmt.Sprintf("[yellow]Conectado na porta %d[white]", discoveryPort))
	chatUI.AddMessage("[yellow]Use /help para ver a lista de comandos disponíveis.[white]")
	
	// Adiciona informações iniciais aos logs
	chatUI.AddLogMessage(fmt.Sprintf("[INFO] Iniciando serviço de descoberta de peers (instanceID: %s, porta: %d)", 
		discoveryService.GetInstanceID(), discoveryService.GetPort()))

	// Monitora peers conhecidos
	go monitorPeers(ctx, discoveryService, chatUI)

	// Configura tratamento de sinais para encerramento limpo
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		chatUI.AddLogMessage("[INFO] Encerrando aplicação...")
		cancel()
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	// Inicia a interface do usuário
	if err := chatUI.Run(); err != nil {
		log.Fatalf("Erro na interface do usuário: %v", err)
	}
}

// handleInput processa a entrada do usuário
func handleInput(input string, chatUI *ui.UnifiedUI, discoveryService *discovery.Discovery) {
	// Verifica se é um comando
	if strings.HasPrefix(input, "/") {
		parts := strings.SplitN(input, " ", 2)
		command := strings.ToLower(parts[0])
		
		var args string
		if len(parts) > 1 {
			args = parts[1]
		}
		
		// Processa o comando
		switch command {
		case "/help":
			showHelp(chatUI)
		case "/nick":
			handleNickCommand(args, chatUI)
		case "/join":
			handleJoinCommand(args, chatUI)
		case "/part":
			handlePartCommand(args, chatUI)
		case "/msg":
			handleMsgCommand(args, chatUI)
		case "/who":
			handleWhoCommand(chatUI, discoveryService)
		case "/peers":
			handlePeersCommand(chatUI, discoveryService)
		case "/quit", "/exit":
			handleQuitCommand(chatUI)
		default:
			chatUI.AddMessage(fmt.Sprintf("[red]Comando desconhecido: %s[white]", command))
			chatUI.AddMessage("[yellow]Use /help para ver a lista de comandos disponíveis.[white]")
		}
	} else {
		// Mensagem normal para o canal ativo
		channel := chatUI.GetActiveChannel()
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		message := fmt.Sprintf("%s <%s> %s", timestamp, nickname, input)
		chatUI.AddMessageToChannel(channel, message)
		
		// Envia a mensagem para todos os peers
		// Aqui seria implementada a lógica de envio para os peers
		// Por enquanto, apenas registra nos logs
		chatUI.AddLogMessage(fmt.Sprintf("[DEBUG] Mensagem enviada para %s: %s", channel, input))
	}
}

// Funções auxiliares para processar comandos

func showHelp(chatUI *ui.UnifiedUI) {
	chatUI.AddMessage("Comandos disponíveis:")
	chatUI.AddMessage("- /nick <novo> - Define seu nickname")
	chatUI.AddMessage("- /join <#canal> - Entra em um canal")
	chatUI.AddMessage("- /part [#canal] - Sai do canal atual ou especificado")
	chatUI.AddMessage("- /msg <usuário|#canal> <mensagem> - Envia mensagem privada")
	chatUI.AddMessage("- /who - Lista usuários conectados na rede")
	chatUI.AddMessage("- /peers - Lista todos os peers conectados")
	chatUI.AddMessage("- /quit ou /exit - Encerra a aplicação")
	chatUI.AddMessage("")
	chatUI.AddMessage("Atalhos de teclado:")
	chatUI.AddMessage("- F1: Exibe esta ajuda")
	chatUI.AddMessage("- Alt+1-9: Alterna entre canais")
	chatUI.AddMessage("- Ctrl+L: Limpa a área de logs")
	chatUI.AddMessage("- Ctrl+P: Alterna o foco entre painéis")
	chatUI.AddMessage("- Ctrl+N: Cria um novo canal")
	chatUI.AddMessage("- Esc: Volta para o campo de entrada")
}

func handleNickCommand(args string, chatUI *ui.UnifiedUI) {
	if args == "" {
		chatUI.AddMessage("Uso: /nick <novo>")
		return
	}
	
	oldNick := nickname
	nickname = args
	chatUI.AddMessage(fmt.Sprintf("Seu nickname foi alterado de %s para %s", oldNick, nickname))
	chatUI.AddLogMessage(fmt.Sprintf("[INFO] Nickname alterado: %s -> %s", oldNick, nickname))
}

func handleJoinCommand(args string, chatUI *ui.UnifiedUI) {
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
	
	// Adiciona o canal
	channels = append(channels, channel)
	chatUI.SetChannels(channels)
	chatUI.SetActiveChannel(channel)
	
	// Adiciona mensagem de entrada
	chatUI.AddMessageToChannel(channel, fmt.Sprintf("[green]* Você entrou no canal %s[white]", channel))
	chatUI.AddLogMessage(fmt.Sprintf("[INFO] Entrou no canal: %s", channel))
}

func handlePartCommand(args string, chatUI *ui.UnifiedUI) {
	// Se não especificar canal, usa o ativo
	channel := chatUI.GetActiveChannel()
	if args != "" {
		channel = args
		// Adiciona # se não estiver presente
		if !strings.HasPrefix(channel, "#") {
			channel = "#" + channel
		}
	}
	
	// Verifica se está no canal
	found := false
	for i, ch := range channels {
		if ch == channel {
			// Remove o canal
			channels = append(channels[:i], channels[i+1:]...)
			found = true
			break
		}
	}
	
	if !found {
		chatUI.AddMessage(fmt.Sprintf("Você não está no canal %s", channel))
		return
	}
	
	chatUI.AddMessage(fmt.Sprintf("Você saiu do canal %s", channel))
	chatUI.AddLogMessage(fmt.Sprintf("[INFO] Saiu do canal: %s", channel))
	
	// Se era o canal ativo, muda para #general
	if channel == activeChannel {
		activeChannel = "#general"
		// Verifica se #general existe
		hasGeneral := false
		for _, ch := range channels {
			if ch == "#general" {
				hasGeneral = true
				break
			}
		}
		
		if !hasGeneral {
			channels = append(channels, "#general")
		}
		
		chatUI.SetActiveChannel("#general")
	}
	
	chatUI.SetChannels(channels)
}

func handleMsgCommand(args string, chatUI *ui.UnifiedUI) {
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
		chatUI.AddLogMessage(fmt.Sprintf("[DEBUG] Mensagem enviada para canal %s: %s", channel, message))
	} else {
		// Mensagem privada para usuário
		targetNick := strings.TrimPrefix(target, "@")
		
		// Adiciona a mensagem localmente
		timestamp := time.Now().Format("[2006-01-02 15:04:05]")
		formattedMsg := fmt.Sprintf("%s [Mensagem privada para %s]: %s", timestamp, targetNick, message)
		chatUI.AddMessage(formattedMsg)
		chatUI.AddLogMessage(fmt.Sprintf("[DEBUG] Mensagem privada enviada para %s: %s", targetNick, message))
	}
}

func handleWhoCommand(chatUI *ui.UnifiedUI, discoveryService *discovery.Discovery) {
	// Lista usuários localmente
	chatUI.AddMessage("Usuários conectados:")
	chatUI.AddMessage(fmt.Sprintf("- %s (você)", nickname))
	
	// Obtém a lista de peers do serviço de descoberta
	peers := discoveryService.GetPeers()
	for _, peer := range peers {
		chatUI.AddMessage(fmt.Sprintf("- Peer: %s", peer.Addr))
	}
}

func handlePeersCommand(chatUI *ui.UnifiedUI, discoveryService *discovery.Discovery) {
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

func handleQuitCommand(chatUI *ui.UnifiedUI) {
	// Exibe mensagem de despedida
	chatUI.AddMessage("Encerrando aplicação...")
	
	// Aguarda um pouco para a mensagem ser exibida
	time.Sleep(500 * time.Millisecond)
	
	// Encerra a aplicação
	os.Exit(0)
}

func monitorPeers(ctx context.Context, discoveryService *discovery.Discovery, chatUI *ui.UnifiedUI) {
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
			chatUI.AddLogMessage(fmt.Sprintf("[INFO] Métricas: Peers ativos: %d, Total descobertos: %d", 
				metrics.ActivePeers, metrics.TotalDiscovered))
		}
	}
}
