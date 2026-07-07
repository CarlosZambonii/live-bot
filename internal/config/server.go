package config

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/CarlosZambonii/backseat/internal/memory"
	"github.com/CarlosZambonii/backseat/internal/mood"
)

//go:embed static
var staticFS embed.FS

type dto struct {
	Persona         string  `json:"persona"`
	VADThreshold    float64 `json:"vad_threshold"`
	MentionCooldown int     `json:"mention_cooldown_s"`
	SpontCooldown   int     `json:"spont_cooldown_s"`
	AutoCooldown    int     `json:"auto_cooldown_s"`
	Vision          bool    `json:"vision"`
	SpontChance     float64 `json:"spont_chance"`
}

func toDTO(c Config) dto {
	return dto{
		Persona:         c.Persona,
		VADThreshold:    c.VADThreshold,
		MentionCooldown: int(c.MentionCooldown / time.Second),
		SpontCooldown:   int(c.SpontCooldown / time.Second),
		AutoCooldown:    int(c.AutoCooldown / time.Second),
		Vision:          c.Vision,
		SpontChance:     c.SpontChance,
	}
}

// Serve sobe a API de config + painel + rotas de personas. Bloqueante.
func (c *Config) Serve(addr string, store *memory.Store, m *mood.State) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			json.NewEncoder(w).Encode(toDTO(c.Snapshot()))
		case http.MethodPut:
			var in dto
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			c.Update(func(cfg *Config) {
				if in.Persona != "" {
					cfg.Persona = in.Persona
				}
				if in.VADThreshold > 0 {
					cfg.VADThreshold = in.VADThreshold
				}
				if in.MentionCooldown > 0 {
					cfg.MentionCooldown = time.Duration(in.MentionCooldown) * time.Second
				}
				if in.SpontCooldown > 0 {
					cfg.SpontCooldown = time.Duration(in.SpontCooldown) * time.Second
				}
				if in.AutoCooldown > 0 {
					cfg.AutoCooldown = time.Duration(in.AutoCooldown) * time.Second
				}
				if in.SpontChance > 0 {
					cfg.SpontChance = in.SpontChance
				}
				cfg.Vision = in.Vision
			})
			log.Println("[config] atualizada via API")
			json.NewEncoder(w).Encode(toDTO(c.Snapshot()))
		default:
			http.Error(w, "método não suportado", http.StatusMethodNotAllowed)
		}
	})

	if store != nil {
		PersonaAPI(mux, store, func(prompt string) {
			c.Update(func(cfg *Config) { cfg.Persona = prompt })
		})
	}

	if m != nil {
		AvatarWS(mux, m)
		DanceAPI(mux, m)
		AnimAPI(mux, m)
		ObjectAPI(mux, m)
		MoodAPI(mux, m)
	}

	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("[config] painel em http://%s — API em /config", addr)
	return http.ListenAndServe(addr, mux)
}
