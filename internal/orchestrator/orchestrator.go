package orchestrator

import (
	"log"
	"strings"

	"github.com/CarlosZambonii/backseat/internal/brain"
	"github.com/CarlosZambonii/backseat/internal/stt"
	"github.com/CarlosZambonii/backseat/internal/voice"
)

type Orchestrator struct {
	STT   *stt.Client
	Brain *brain.Client
	Voice *voice.Client

	ChunkSeconds int // duração de cada janela de escuta
}

// Run: loop ouvir -> transcrever -> pensar -> falar. Ctrl+C pra parar.
func (o *Orchestrator) Run() {
	log.Println("[loop] ouvindo... (fale no mic; Ctrl+C para sair)")
	for {
		wav, err := stt.Listen(o.ChunkSeconds)
		if err != nil {
			log.Printf("[mic] %v", err)
			continue
		}

		text, err := o.STT.Transcribe(wav)
		if err != nil {
			log.Printf("[stt] %v", err)
			continue
		}
		text = strings.TrimSpace(text)
		if len(text) < 6 || !strings.ContainsAny(strings.ToLower(text), "abcdefghijklmnopqrstuvwxyzáéíóúãõç") { // silêncio/ruído -> ignora (VAD de pobre; melhora na Fase 3)
			continue
		}
		log.Printf("[você] %s", text)

		reply, err := o.Brain.Think(text)
		if err != nil {
			log.Printf("[brain] %v", err)
			continue
		}
		log.Printf("[backseat] %s", reply)

		if err := o.Voice.Speak(reply); err != nil {
			log.Printf("[voice] %v", err)
		}
	}
}
