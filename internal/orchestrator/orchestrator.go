package orchestrator

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/CarlosZambonii/backseat/internal/brain"
	"github.com/CarlosZambonii/backseat/internal/chat"
	"github.com/CarlosZambonii/backseat/internal/capture"
	"github.com/CarlosZambonii/backseat/internal/stt"
	"github.com/CarlosZambonii/backseat/internal/voice"
)

type Orchestrator struct {
	STT    *stt.Client
	Brain  *brain.Client
	Voice  *voice.Client
	Vision bool
	Chat   chat.Source

	VADThreshold float64

	mentionCooldown time.Duration
	lastMention     time.Time
	speaking        sync.Mutex // serializa quem usa a voz
}

func (o *Orchestrator) Run() {
	// chat: consome mensagens e mantém o buffer de contexto atualizado
	if o.Chat != nil {
		buf := chat.NewBuffer(15)
		go func() {
			if err := o.Chat.Start(); err != nil {
				log.Printf("[chat] %v", err)
			}
		}()
		o.mentionCooldown = 45 * time.Second
		go func() {
			for m := range o.Chat.Messages() {
				log.Printf("[chat] %s: %s", m.User, m.Text)
				buf.Add(m)
				o.Brain.SetContext(buf.Context())

				if mentionsDora(m.Text) {
					if time.Since(o.lastMention) < o.mentionCooldown {
						log.Printf("[menção] %s chamou, mas cooldown ativo (%.0fs restantes)", m.User, (o.mentionCooldown - time.Since(o.lastMention)).Seconds())
						continue
					}
					o.lastMention = time.Now()
					go o.answerMention(m)
				}
			}
		}()
	}

	listener := stt.NewListener(o.VADThreshold)
	go func() {
		if err := listener.Start(); err != nil {
			log.Fatalf("[mic] %v", err)
		}
	}()
	log.Printf("[loop] escuta contínua ligada (VAD threshold=%.0f, visão=%v). Ctrl+C para sair.", o.VADThreshold, o.Vision)

	for wavPath := range listener.Segments {
		start := time.Now()

		tSTT := time.Now()
		text, err := o.STT.Transcribe(wavPath)
		log.Printf("[t] stt: %.1fs", time.Since(tSTT).Seconds())
		os.Remove(wavPath)
		if err != nil {
			log.Printf("[stt] %v", err)
			continue
		}
		text = strings.TrimSpace(text)
		if !isRealSpeech(text) {
			continue
		}
		log.Printf("[você] %s", text)

		var reply string
		if o.Vision {
			if shot, err := capture.Screenshot(); err == nil {
				tBrain := time.Now()
				reply, err = o.Brain.ThinkWithVision(text, shot)
				log.Printf("[t] brain+visão: %.1fs", time.Since(tBrain).Seconds())
				capture.Cleanup(shot)
				if err != nil {
					log.Printf("[brain] %v", err)
					continue
				}
			} else {
				log.Printf("[visão] %v (seguindo sem imagem)", err)
				if reply, err = o.Brain.Think(text); err != nil {
					log.Printf("[brain] %v", err)
					continue
				}
			}
		} else {
			if reply, err = o.Brain.Think(text); err != nil {
				log.Printf("[brain] %v", err)
				continue
			}
		}
		log.Printf("[backseat] %s", reply)

		tVoice := time.Now()
		if err := o.Voice.Speak(reply); err != nil {
			log.Printf("[voice] %v", err)
		}
		log.Printf("[t] voz (gerar+tocar): %.1fs", time.Since(tVoice).Seconds())
		log.Printf("[latência] %.1fs (fala->fim da resposta)", time.Since(start).Seconds())

		// anti-eco: descarta o que foi captado durante a fala dela E
		// segura mais um pouco pra pegar segmentos retardatários do VAD
		drain(listener.Segments)
		time.Sleep(1200 * time.Millisecond)
		drain(listener.Segments)
	}
}

func drain(ch chan string) {
	for {
		select {
		case p := <-ch:
			os.Remove(p)
		default:
			return
		}
	}
}

// isRealSpeech filtra alucinações do Whisper: texto curto, sem letras, ou repetitivo.
func isRealSpeech(text string) bool {
	if len(text) < 6 {
		return false
	}
	if !strings.ContainsAny(strings.ToLower(text), "abcdefghijklmnopqrstuvwxyzáéíóúãõç") {
		return false
	}
	// anti "laur de laur de laur": palavra dominante demais
	words := strings.Fields(strings.ToLower(text))
	if len(words) >= 6 {
		count := map[string]int{}
		max := 0
		for _, w := range words {
			count[w]++
			if count[w] > max {
				max = count[w]
			}
		}
		if float64(max)/float64(len(words)) > 0.5 {
			return false
		}
	}
	return true
}


// mentionsDora detecta se a mensagem chama a Dora.
func mentionsDora(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "dora")
}

// answerMention responde por voz a uma menção do chat.
func (o *Orchestrator) answerMention(m chat.Message) {
	prompt := "O viewer \"" + m.User + "\" disse no chat: \"" + m.Text + "\". Responda a ele diretamente pelo nick, por voz, em uma frase."
	reply, err := o.Brain.Think(prompt)
	if err != nil {
		log.Printf("[menção] brain: %v", err)
		return
	}
	log.Printf("[backseat->%s] %s", m.User, reply)

	o.speaking.Lock()
	defer o.speaking.Unlock()
	if err := o.Voice.Speak(reply); err != nil {
		log.Printf("[menção] voz: %v", err)
	}
}