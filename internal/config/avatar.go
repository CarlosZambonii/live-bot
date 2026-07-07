package config

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/CarlosZambonii/backseat/internal/mood"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// AvatarWS transmite o estado (humor + speaking) pro avatar, 10x por segundo.
func AvatarWS(mux *http.ServeMux, m *mood.State) {
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		log.Println("[avatar] conectado")
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			state := map[string]any{"mood": m.Get(), "speaking": m.IsSpeaking()}
			b, _ := json.Marshal(state)
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				log.Println("[avatar] desconectado")
				return
			}
		}
	})
}
