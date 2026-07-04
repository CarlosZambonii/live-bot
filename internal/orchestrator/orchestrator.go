package orchestrator

import (
	"log"
	"strings"

	"github.com/CarlosZambonii/backseat/internal/brain"
	"github.com/CarlosZambonii/backseat/internal/capture"
	"github.com/CarlosZambonii/backseat/internal/stt"
	"github.com/CarlosZambonii/backseat/internal/voice"
)

type Orchestrator struct {
	STT   *stt.Client
	Brain *brain.Client
	Voice *voice.Client

	ChunkSeconds int
	Vision       bool // liga/desliga o screenshot junto da fala
}

func (o *Orchestrator) Run() {
	log.Printf("[loop] ouvindo... (visão: %v; Ctrl+C para sair)", o.Vision)
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
		if len(text) < 6 || !strings.ContainsAny(strings.ToLower(text), "abcdefghijklmnopqrstuvwxyzáéíóúãõç") {
			continue
		}
		log.Printf("[você] %s", text)

		var reply string
		if o.Vision {
			shot, err := capture.Screenshot()
			if err != nil {
				log.Printf("[visão] %v (seguindo sem imagem)", err)
				reply, err = o.Brain.Think(text)
			} else {
				reply, err = o.Brain.ThinkWithVision(text, shot)
				capture.Cleanup(shot)
			}
			if err != nil {
				log.Printf("[brain] %v", err)
				continue
			}
		} else {
			reply, err = o.Brain.Think(text)
			if err != nil {
				log.Printf("[brain] %v", err)
				continue
			}
		}
		log.Printf("[backseat] %s", reply)

		if err := o.Voice.Speak(reply); err != nil {
			log.Printf("[voice] %v", err)
		}
	}
}
