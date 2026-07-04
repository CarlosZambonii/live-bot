package main

import (
	"log"
	"os"

	"github.com/CarlosZambonii/backseat/internal/brain"
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

const defaultPersona = `Você é o Backseat, co-host de IA de uma live de games em português brasileiro.
Personalidade: zoeiro opressor, sarcástico, mas parceiro. Comenta a gameplay, zoa quando o streamer erra, elogia (com deboche) quando acerta.
Regras: responda com UMA frase só, máximo 15 palavras. Zoeira rápida e certeira, não discurso. Sem emojis, sem listas. Português brasileiro com gíria.`

func main() {
	log.SetFlags(0)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY não definida. Configure no .env e rode: export $(grep -v '^#' .env | xargs)")
	}

	o := &orchestrator.Orchestrator{
		STT:          stt.New(env("VOICEBOX_URL", "http://127.0.0.1:17493"), env("STT_MODEL", "whisper-base")),
		Brain:        brain.New(apiKey, env("OPENAI_MODEL", "gpt-4o-mini"), env("PERSONA", defaultPersona)),
		Voice:        voice.New(env("VOICEBOX_URL", "http://127.0.0.1:17493"), env("TTS_ENGINE", "kokoro"), env("TTS_LANGUAGE", "pt"), env("TTS_PROFILE_ID", "")),
		VADThreshold: 500,
		Vision:       true,
	}

	log.Println("Backseat — Fase 1: loop de voz")
	o.Run()
}
