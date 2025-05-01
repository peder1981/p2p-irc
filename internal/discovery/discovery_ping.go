// Este arquivo contém a lógica para ping de peers no serviço de descoberta

package discovery

import (
	"context"
	"log"
	"sync"
	"time"
)

// pingPeerWrapper realiza um ping em um peer específico para verificar se ainda está ativo
func (d *Discovery) pingPeerWrapper(ctx context.Context, addr string, wg *sync.WaitGroup) {
	defer wg.Done()

	select {
	case <-ctx.Done():
		return
	default:
	}

	conn, ok := d.getPeerConn(addr)
	if !ok {
		log.Printf("[PING] Conexão não encontrada para peer %s, removendo da lista", addr)
		d.RemovePeer(addr)
		return
	}

	// PING: registra último Pong recebido
	conn.mu.RLock()
	lastPing := conn.LastPing
	conn.mu.RUnlock()

	// Cria e envia mensagem de ping
	pingMsg := Message{
		Type:      TypePing,
		Sender:    d.instanceID,
		Timestamp: time.Now(),
	}

	if err := conn.SendMessage(pingMsg); err != nil {
		log.Printf("[PING] Falha ao pingar peer %s: %v. Desconectando", addr, err)
		d.DisconnectPeer(addr)
		return
	}

	// Aguarda PONG ou timeout de 3s
	select {
	case <-ctx.Done():
		return
	case <-time.After(3 * time.Second):
		conn2, ok2 := d.getPeerConn(addr)
		if !ok2 {
			log.Printf("[PING] Conexão perdida para peer %s", addr)
			return
		}
		conn2.mu.RLock()
		updatedPing := conn2.LastPing
		conn2.mu.RUnlock()
		if updatedPing.After(lastPing) {
			log.Printf("[PING] Peer %s respondeu ao ping em %v", addr, time.Since(lastPing))
		} else {
			log.Printf("[PING] Timeout ao pingar peer %s, desconectando", addr)
			d.DisconnectPeer(addr)
		}
	}
}

// startPeerMonitor inicia o monitoramento periódico de peers para verificar conectividade
func (d *Discovery) startPeerMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[MONITOR] Monitoramento de peers encerrado")
				return
			case <-ticker.C:
				// Verifica se temos peers conectados antes de logar
				d.connMu.RLock()
				peerCount := len(d.connections)
				d.connMu.RUnlock()
				
				if peerCount > 0 {
					log.Printf("[MONITOR] Verificando %d peers conectados", peerCount)
				}
				
				// Como checkPeers não está definido, vamos implementar uma lógica simples aqui
				var wg sync.WaitGroup
				d.connMu.RLock()
				for addr := range d.connections {
					wg.Add(1)
					go d.pingPeerWrapper(ctx, addr, &wg)
				}
				d.connMu.RUnlock()
				wg.Wait()
			}
		}
	}()
}
