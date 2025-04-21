//go:build ignore
// +build ignore

package ui

import (
    "context"
    "embed"
    "io/fs"
    "log"
    "net/http"

    "github.com/gorilla/websocket"
    "github.com/peder1981/p2p-irc/internal/network"
)

//go:embed static/*
var staticFS embed.FS

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

// RunWebUI inicia o servidor HTTP e WebSocket para a interface web.
func RunWebUI(ctx context.Context, netSvc *network.Network, httpAddr string) error {
    // Serve arquivos estáticos
    subFS, err := fs.Sub(staticFS, "static")
    if err != nil {
        return err
    }
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(subFS))))
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.FileServer(http.FS(subFS)).ServeHTTP(w, r)
    })

    // WebSocket
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Println("upgrade WS:", err)
            return
        }
        defer conn.Close()

        // envia lista inicial de peers
        peers := netSvc.Host.Peerstore().Peers()
        conn.WriteJSON(map[string]interface{}{"type": "peers", "data": peers})

        // recebe mensagens do P2P
        msgs, _ := netSvc.Receive(ctx)
        go func() {
            for m := range msgs {
                conn.WriteJSON(map[string]string{"type": "message", "data": m})
            }
        }()

        // recebe do cliente
        for {
            var msg map[string]string
            if err := conn.ReadJSON(&msg); err != nil {
                break
            }
            if msg["type"] == "message" {
                netSvc.Publish(ctx, msg["data"])
            }
        }
    })

    log.Println("Web UI listening on", httpAddr)
    return http.ListenAndServe(httpAddr, nil)
}
