package ui

import (
	"fmt"
	"strings"
)

// IRCCommandHandler define a assinatura para handlers de comandos IRC
// Recebe a GUI, o input do usuário e um fallback para handler padrão
// Retorna true se o comando foi tratado, false caso contrário
//
// Handler genérico de comando IRC
// Aceita ChannelManager (UI) e PeerDiscovery (descoberta)
type PeerDiscovery interface {
	GetKnownPeers() []string
}

type IRCGenericCommandHandler func(cm ChannelManager, discovery PeerDiscovery, input string, fallback func(string)) bool

// Comandos IRC suportados
var ircCommandRegistry = map[string]IRCGenericCommandHandler{
	"/peers": handlePeersCommand,
	"/who":   handleWhoCommand,
	"/info":  handleInfoCommand,
}

// Handler para o comando /peers
func handlePeersCommand(cm ChannelManager, discovery PeerDiscovery, input string, fallback func(string)) bool {
	if !strings.HasPrefix(input, "/peers") {
		return false
	}
	peers := discovery.GetKnownPeers()
	var msg string
	if len(peers) == 0 {
		msg = "[SISTEMA] Nenhum peer conhecido no momento."
	} else {
		msg = "[SISTEMA] Peers conhecidos:\n  - " + strings.Join(peers, "\n  - ")
	}
	cm.AddMessageToChannel(cm.GetActiveChannel(), msg)
	return true
}

// Handler para o comando /who (listar peers do canal atual)
func handleWhoCommand(cm ChannelManager, discovery PeerDiscovery, input string, fallback func(string)) bool {
	if !strings.HasPrefix(input, "/who") {
		return false
	}
	activeChannel := cm.GetActiveChannel()
	if activeChannel == "" {
		cm.AddMessage("Nenhum canal ativo.")
		return true
	}
	// Como PeerDiscovery não tem GetPeersInChannel, simulamos filtrando peers conhecidos que contenham o nome do canal (mock/testes)
	peers := []string{}
	for _, peer := range discovery.GetKnownPeers() {
		if strings.Contains(peer, activeChannel) {
			peers = append(peers, peer)
		}
	}
	if len(peers) == 0 {
		cm.AddMessageToChannel(activeChannel, "Nenhum peer encontrado no canal.")
		return true
	}
	msg := fmt.Sprintf("Peers no canal %s: %s", activeChannel, strings.Join(peers, ", "))
	cm.AddMessageToChannel(activeChannel, msg)
	return true
}

// Handler para o comando /info (informações gerais da sessão)
func handleInfoCommand(cm ChannelManager, discovery PeerDiscovery, input string, fallback func(string)) bool {
	if !strings.HasPrefix(input, "/info") {
		return false
	}
	nPeers := len(discovery.GetKnownPeers())
	msg := fmt.Sprintf("[SISTEMA] Sessão IRC P2P\nPeers conhecidos: %d\nCanal ativo: %s", nPeers, cm.GetActiveChannel())
	cm.AddMessageToChannel(cm.GetActiveChannel(), msg)
	return true
}

// Função central para processar comandos IRC
// Adaptador para chamada dos handlers genéricos a partir de uma GUI
func HandleIRCCommand(cm ChannelManager, discovery PeerDiscovery, input string, fallback func(string)) bool {
	for prefix, handler := range ircCommandRegistry {
		if strings.HasPrefix(input, prefix) {
			return handler(cm, discovery, input, fallback)
		}
	}
	return false
}
