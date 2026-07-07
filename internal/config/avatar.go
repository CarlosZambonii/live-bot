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
			state := map[string]any{"mood": m.Get(), "speaking": m.IsSpeaking(), "dancing": m.IsDancing(), "anim": m.Anim(), "mouth": m.Mouth(), "said": m.LastSaid(), "object": m.Object(), "skin": m.Skin()}
			b, _ := json.Marshal(state)
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				log.Println("[avatar] desconectado")
				return
			}
		}
	})
}


// DanceAPI liga/desliga a dança via POST /dance?on=true|false
func DanceAPI(mux *http.ServeMux, m *mood.State) {
	mux.HandleFunc("/dance", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		on := r.URL.Query().Get("on") != "false"
		m.SetDancing(on)
		w.Write([]byte("ok"))
	})
}

// AnimAPI dispara uma animação por nome: POST /anim?name=Clapping
func AnimAPI(mux *http.ServeMux, m *mood.State) {
	mux.HandleFunc("/anim", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "name obrigatório", 400)
			return
		}
		m.SetAnim(name)
		w.Write([]byte("ok"))
	})
}

// ObjectAPI força um objeto de cena: POST /object?name=cafe (ou none)
func ObjectAPI(mux *http.ServeMux, m *mood.State) {
	mux.HandleFunc("/object", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		name := r.URL.Query().Get("name")
		if name == "none" {
			name = ""
		}
		m.SetObject(name)
		w.Write([]byte("ok"))
	})
}

// MoodAPI força o humor da Dora: POST /mood?set=animada (pra teste)
func MoodAPI(mux *http.ServeMux, m *mood.State) {
	mux.HandleFunc("/mood", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		set := r.URL.Query().Get("set")
		if set != "" {
			m.Set(set)
			m.SetAnim(animForMoodAvatar(set))
		}
		w.Write([]byte("ok"))
	})
}

func animForMoodAvatar(m string) string {
	switch m {
	case "surpresa":
		return "Surprised"
	case "animada":
		return "Clapping"
	case "provocada":
		return "Angry"
	case "entediada":
		return "Sleepy"
	case "zoeira":
		return "Blush"
	}
	return ""
}

// SkinAPI troca a skin: POST /skin?name=avatar
func SkinAPI(mux *http.ServeMux, m *mood.State) {
	mux.HandleFunc("/skin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if n := r.URL.Query().Get("name"); n != "" {
			m.SetSkin(n)
		}
		w.Write([]byte("ok"))
	})
}