package orchestrator

import (
	"log"
	"math/rand"
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
	spontCooldown   time.Duration
	lastSpont       time.Time
	speaking        sync.Mutex // serializa quem usa a voz
	segments        chan string
	muteMu          sync.Mutex
	muteUntil       time.Time // segmentos capturados antes disso são eco dela
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
		o.spontCooldown = 150 * time.Second
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
					continue
				}
				o.maybeAnswerSpontaneous(m)
			}
		}()
	}

	listener := stt.NewListener(o.VADThreshold)
	o.segments = listener.Segments
	go func() {
		if err := listener.Start(); err != nil {
			log.Fatalf("[mic] %v", err)
		}
	}()
	log.Printf("[loop] escuta contínua ligada (VAD threshold=%.0f, visão=%v). Ctrl+C para sair.", o.VADThreshold, o.Vision)

	for wavPath := range listener.Segments {
		o.muteMu.Lock()
		muted := time.Now().Before(o.muteUntil)
		o.muteMu.Unlock()
		if muted {
			os.Remove(wavPath)
			log.Println("[anti-eco] segmento do período de fala dela, descartado")
			continue
		}
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
		o.speak(reply)
		log.Printf("[t] voz (gerar+tocar): %.1fs", time.Since(tVoice).Seconds())
		log.Printf("[latência] %.1fs (fala->fim da resposta)", time.Since(start).Seconds())


	}
}

// speak centraliza toda fala da Dora: serializa a boca e drena o eco.
func (o *Orchestrator) speak(text string) {
	o.speaking.Lock()
	defer o.speaking.Unlock()
	if err := o.Voice.Speak(text); err != nil {
		log.Printf("[voice] %v", err)
	}
	// tudo que o VAD fechar até 1.5s após o fim da fala é eco dela
	o.muteMu.Lock()
	o.muteUntil = time.Now().Add(1500 * time.Millisecond)
	o.muteMu.Unlock()
	if o.segments != nil {
		drain(o.segments)
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
	o.speak(reply)
}

// maybeAnswerSpontaneous decide se responde uma mensagem que NÃO menciona a Dora.
// Travas em ordem barata->cara: budget -> dado -> classificador LLM -> resposta.
func (o *Orchestrator) maybeAnswerSpontaneous(m chat.Message) {
	if time.Since(o.lastSpont) < o.spontCooldown {
		return // budget estourado, nem gasta classificação
	}
	if rand.Float64() > 0.6 {
		return // dado: 40% das candidatas morrem aqui, mantém imprevisível
	}
	go func() {
		verdict, err := o.Brain.Think("Classifique a mensagem de chat a seguir. Responda APENAS 'sim' se for uma pergunta genuína que um co-host deveria responder (sobre a live, o jogo, o streamer, o canal), ou 'nao' para spam, emote, papo entre viewers ou piada. Mensagem de \"" + m.User + "\": \"" + m.Text + "\"")
		if err != nil || !strings.Contains(strings.ToLower(verdict), "sim") {
			return
		}
		// re-checa o budget (a classificação levou tempo, outra goroutine pode ter falado)
		if time.Since(o.lastSpont) < o.spontCooldown {
			return
		}
		o.lastSpont = time.Now()
		log.Printf("[espontânea] respondendo %s", m.User)

		reply, err := o.Brain.Think("O viewer \"" + m.User + "\" perguntou no chat: \"" + m.Text + "\". Responda a ele pelo nick, em uma frase, por voz.")
		if err != nil {
			log.Printf("[espontânea] brain: %v", err)
			return
		}
		log.Printf("[backseat->%s] %s", m.User, reply)
		o.speak(reply)
	}()
}