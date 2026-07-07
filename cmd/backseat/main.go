package main

import (
	"log"
	"os"
	"time"

	"github.com/CarlosZambonii/backseat/internal/brain"
	"github.com/CarlosZambonii/backseat/internal/chat"
	"github.com/CarlosZambonii/backseat/internal/config"
	"github.com/CarlosZambonii/backseat/internal/mood"
	"github.com/CarlosZambonii/backseat/internal/tools"
	"github.com/CarlosZambonii/backseat/internal/memory"
	"github.com/CarlosZambonii/backseat/internal/orchestrator"
	"github.com/CarlosZambonii/backseat/internal/stt"
	"github.com/CarlosZambonii/backseat/internal/voice"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

const defaultPersona = `Você é a Dora, co-host de IA de uma live. Sua voz é transmitida NA LIVE: o streamer e os viewers do chat te ouvem. Quando alguém do chat te chama ou pergunta algo, você PODE e DEVE responder falando — dirija-se ao viewer pelo nick.
Personalidade: direta, calma e profissional. Sem piada, sem gíria (modo teste).
Regras: UMA frase, máximo 15 palavras. Sem emojis, sem listas. Português brasileiro.`

func main() {
	log.SetFlags(0)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY não definida. Configure no .env e rode: export $(grep -v '^#' .env | xargs)")
	}

	var chatSrc chat.Source
	switch env("CHAT_SOURCE", "fake") {
	case "twitch":
		ch := os.Getenv("TWITCH_CHANNEL")
		if ch == "" {
			log.Fatal("CHAT_SOURCE=twitch mas TWITCH_CHANNEL vazio no .env")
		}
		chatSrc = chat.NewTwitch(ch)
	case "fake":
		chatSrc = chat.NewFake(8 * time.Second)
	}

	var mem *memory.Store
	if m, err := memory.New(env("REDIS_ADDR", "localhost:6379"), env("PG_DSN", "postgres://backseat:backseat@localhost:5432/backseat?sslmode=disable")); err != nil {
		log.Printf("[memória] indisponível (%v) — rodando sem", err)
	} else {
		mem = m
	}

	cfg := config.Default(env("PERSONA", defaultPersona))
	moodState := mood.New()
	searcher := tools.NewSearcher(os.Getenv("TAVILY_API_KEY"))
	br := brain.New(apiKey, env("OPENAI_MODEL", "gpt-4o-mini"), env("PERSONA", defaultPersona))
	br.SetSearcher(searcher)
	go func() {
		if err := cfg.Serve("127.0.0.1:8090", mem, moodState); err != nil {
			log.Printf("[config] server: %v", err)
		}
	}()

	o := &orchestrator.Orchestrator{
		STT:          stt.New(env("VOICEBOX_URL", "http://127.0.0.1:17493"), env("STT_MODEL", "whisper-base")),
		Brain:        br,
		Voice:        voice.New(env("VOICEBOX_URL", "http://127.0.0.1:17493"), env("TTS_ENGINE", "kokoro"), env("TTS_LANGUAGE", "pt"), env("TTS_PROFILE_ID", "")),
		Cfg:          cfg,
		Search:       searcher,
		Mood:         moodState,
		Chat:         chatSrc,
		Memory:       mem,
	}

	log.Println("Backseat — Fase 1: loop de voz")
	o.Run()
}
