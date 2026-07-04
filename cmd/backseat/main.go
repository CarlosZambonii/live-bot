package main

import (
	"fmt"
	"log"
	"os"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(0)

	fmt.Println("Backseat — co-host de IA")
	fmt.Println("--------------------------------")

	pairs := [][2]string{
		{"OPENAI_MODEL", env("OPENAI_MODEL", "gpt-4o-mini")},
		{"VOICEBOX_URL", env("VOICEBOX_URL", "http://127.0.0.1:17493")},
		{"TTS_ENGINE", env("TTS_ENGINE", "kokoro")},
		{"TTS_LANGUAGE", env("TTS_LANGUAGE", "pt")},
		{"STT_MODEL", env("STT_MODEL", "whisper-base")},
		{"TWITCH_CHANNEL", env("TWITCH_CHANNEL", "(vazio)")},
	}
	for _, p := range pairs {
		fmt.Printf("  %-15s %s\n", p[0], p[1])
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("\n[aviso] OPENAI_API_KEY nao definida — configure no .env antes da Fase 1")
	}

	fmt.Println("\nOK. Esqueleto rodando. Proximo: captura + loop STT->LLM->TTS.")
}
