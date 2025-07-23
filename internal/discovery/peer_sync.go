package discovery

import (
	"fmt"
	"sync"
	"time"
)

// SyncPeers sincroniza os canais e peers entre todas as conexões
// Esta função é crucial para garantir que todos os peers saibam quais canais
// cada um está participando
func (d *Discovery) SyncPeers() {
	// Obtém todos os canais em que estamos
	d.channelMu.RLock()
	channels := make([]string, 0, len(d.channels))
	for channel := range d.channels {
		channels = append(channels, channel)
	}
	d.channelMu.RUnlock()
	
	// Obtém todas as conexões ativas
	d.connMu.RLock()
	connections := make(map[string]*PeerConnection)
	for addr, conn := range d.connections {
		connections[addr] = conn
	}
	d.connMu.RUnlock()
	
	// Log da sincronização

    //     len(channels), len(connections))

	// Se não temos canais ou conexões, não há o que sincronizar
	if len(channels) == 0 || len(connections) == 0 {
		return
	}

	// Para cada peer, envia seu instanceID
	identifyMsg := Message{
		Type:      TypeIdentify,
		Sender:    d.instanceID,
		Content:   d.instanceID,
		Timestamp: time.Now(),
	}

	// Envia a mensagem IDENTIFY para todos os peers
	for addr, peerConn := range connections {
		if err := peerConn.SendMessage(identifyMsg); err != nil {
			fmt.Printf("[ERRO] Falha ao enviar IDENTIFY para peer %s: %v\n", addr, err)
		} else {

		}
	}

	// Para cada canal, envia uma mensagem JOIN para todos os peers
	for _, channel := range channels {
		joinMsg := Message{
			Type:      TypeJoinChannel,
			Sender:    d.instanceID,
			Channel:   channel,
			Timestamp: time.Now(),
		}

		// Envia para todos os peers
		for addr, peerConn := range connections {
			// Verifica se o peer já conhece este canal (para evitar loops)
			peerConn.mu.RLock()
			alreadyInChannel := peerConn.channels[channel]
			peerConn.mu.RUnlock()
			
			if !alreadyInChannel {
				// Adiciona localmente e envia JOIN apenas se o peer não conhecer o canal
				peerConn.mu.Lock()
				peerConn.channels[channel] = true
				peerConn.mu.Unlock()
				
				if err := peerConn.SendMessage(joinMsg); err != nil {
					fmt.Printf("[ERRO] Falha ao enviar JOIN para peer %s (canal %s): %v\n", 
						addr, channel, err)
				}
			}
		}
	}

	// Força uma verificação de conectividade após a sincronização
	go func() {
		time.Sleep(1 * time.Second)
		var wg sync.WaitGroup
		for addr := range connections {
			wg.Add(1)
			go d.pingPeerWrapper(d.ctx, addr, &wg)
		}
		wg.Wait()
	}()
}

// EnhancedBroadcastMessage é uma versão melhorada do BroadcastMessage
// que garante que a mensagem seja entregue a todos os peers no canal
func (d *Discovery) EnhancedBroadcastMessage(msg Message) {
	d.connMu.RLock()
	defer d.connMu.RUnlock()



	// Conta quantos peers receberam a mensagem
	sentCount := 0
	failCount := 0

	for addr, peerConn := range d.connections {
		// Para mensagens de chat, verifica se o peer está no canal
		if msg.Type == TypeChatMessage && msg.Channel != "" && !peerConn.IsInChannel(msg.Channel) {

			continue
		}

		// Envia a mensagem
		if err := peerConn.SendMessage(msg); err != nil {
			fmt.Printf("[ERRO] Falha ao enviar mensagem para peer %s: %v\n", addr, err)
			failCount++
		} else {

			sentCount++
		}
	}

	fmt.Printf("[INFO] Mensagem enviada para %d peers (falhas: %d)\n", sentCount, failCount)
}

// DebugConnections imprime informações detalhadas sobre as conexões atuais
// Útil para depurar problemas de comunicação entre peers
func (d *Discovery) DebugConnections() {
	d.connMu.RLock()
	defer d.connMu.RUnlock()




	for _, peerConn := range d.connections {

		
		// Lista os canais em que o peer está
		peerConn.mu.RLock()
		channels := make([]string, 0, len(peerConn.channels))
		for channel := range peerConn.channels {
			channels = append(channels, channel)
		}
		peerConn.mu.RUnlock()



	}

	// Lista os canais em que estamos
	d.channelMu.RLock()
	channels := make([]string, 0, len(d.channels))
	for channel := range d.channels {
		channels = append(channels, channel)
	}
	d.channelMu.RUnlock()



}

// FixChannelSync corrige a sincronização de canais entre peers
// Esta função deve ser chamada quando houver problemas de comunicação
func (d *Discovery) FixChannelSync() {
	fmt.Printf("[INFO] Iniciando correção de sincronização de canais...\n")

	// 1. Força a sincronização de canais
	d.SyncPeers()

	// 2. Verifica e corrige as conexões
	d.connMu.RLock()
	connections := make(map[string]*PeerConnection)
	for addr, conn := range d.connections {
		connections[addr] = conn
	}
	d.connMu.RUnlock()

	// 3. Para cada conexão, verifica se está funcionando corretamente
	for addr, peerConn := range connections {
		// Envia um PING para verificar se a conexão está ativa
		pingMsg := Message{
			Type:      TypePing,
			Sender:    d.instanceID,
			Timestamp: time.Now(),
		}

		if err := peerConn.SendMessage(pingMsg); err != nil {
			fmt.Printf("[ERRO] Conexão com peer %s parece estar com problemas: %v\n", addr, err)
			
			// Remove a conexão problemática
			d.connMu.Lock()
			delete(d.connections, addr)
			d.connMu.Unlock()

			// Tenta reconectar
			go func(addr string) {
				fmt.Printf("[INFO] Tentando reconectar ao peer %s\n", addr)
				if err := d.ConnectToPeer(addr); err != nil {
					fmt.Printf("[ERRO] Falha ao reconectar ao peer %s: %v\n", addr, err)
				} else {
					fmt.Printf("[INFO] Reconectado com sucesso ao peer %s\n", addr)
				}
			}(addr)
		}
	}

	fmt.Printf("[INFO] Correção de sincronização concluída\n")
}

// handlePeerMessage processa mensagens recebidas de peers
func (d *Discovery) handlePeerMessage(addr string, peerConn *PeerConnection, msg Message) {
    switch msg.Type {
    case TypeJoinChannel:
        if msg.Channel != "" {
            peerConn.mu.Lock()
            peerConn.channels[msg.Channel] = true
            fmt.Printf("[SYNC] Peer %s entrou no canal %s\n", addr, msg.Channel)
            peerConn.mu.Unlock()
        }
    case TypePartChannel:
        if msg.Channel != "" {
            peerConn.mu.Lock()
            delete(peerConn.channels, msg.Channel)
            fmt.Printf("[SYNC] Peer %s saiu do canal %s\n", addr, msg.Channel)
            peerConn.mu.Unlock()
        }
    case TypePing:
        // Responde a pings
        pongMsg := Message{
            Type:      TypePong,
            Sender:    d.instanceID,
            Timestamp: time.Now(),
        }
        if err := peerConn.SendMessage(pongMsg); err != nil {
            fmt.Printf("[ERRO] Falha ao responder PING de %s: %v\n", addr, err)
        }
        peerConn.LastPing = time.Now()
    case TypePong:
        // Atualiza o timestamp do último ping
        peerConn.LastPing = time.Now()

    case TypeChatMessage:
        // Propaga a mensagem para outros peers no mesmo canal
        d.propagateMessage(msg, addr)
        
        // Se temos um handler de mensagens, chama ele
        if d.messageHandler != nil {
            d.messageHandler(msg)
        }
    }
}
