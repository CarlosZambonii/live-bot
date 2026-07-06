package config

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/CarlosZambonii/backseat/internal/memory"
)

// PersonaAPI registra as rotas de personas no mux.
// onActivate é chamado quando uma persona é ativada (pra aplicar na config viva).
func PersonaAPI(mux *http.ServeMux, store *memory.Store, onActivate func(prompt string)) {
	cors := func(w http.ResponseWriter) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	}

	// GET /personas (lista) | POST /personas (cria)
	mux.HandleFunc("/personas", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			list, err := store.ListPersonas()
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(list)
		case http.MethodPost:
			var in memory.Persona
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" || in.Prompt == "" {
				http.Error(w, "name e prompt obrigatórios", 400)
				return
			}
			id, err := store.CreatePersona(in.Name, in.Prompt)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			in.ID = id
			json.NewEncoder(w).Encode(in)
		default:
			http.Error(w, "método não suportado", 405)
		}
	})

	// PUT/DELETE /personas/{id} | POST /personas/{id}/activate
	mux.HandleFunc("/personas/", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/personas/")
		activate := strings.HasSuffix(path, "/activate")
		path = strings.TrimSuffix(path, "/activate")
		id, err := strconv.Atoi(path)
		if err != nil {
			http.Error(w, "id inválido", 400)
			return
		}

		switch {
		case activate && r.Method == http.MethodPost:
			prompt, err := store.ActivatePersona(id)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			onActivate(prompt)
			log.Printf("[persona] ativada id=%d", id)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut:
			var in memory.Persona
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if err := store.UpdatePersona(id, in.Name, in.Prompt); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete:
			if err := store.DeletePersona(id); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "método não suportado", 405)
		}
	})
}
