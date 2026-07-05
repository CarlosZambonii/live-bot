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
	autoCooldown    time.Duration
	lastAuto        time.Time
	activityMu      sync.Mutex
	lastActivity    time.Time
	speaking        sync.Mutex // serializa quem usa a voz
	segments        chan string
	muteMu          sync.Mutex
	muteUntil       time.Time // segmentos capturados antes disso são eco dela
	lastSpoken      string    // última fala dela (pro filtro de similaridade)
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

	// watcher de tela: comenta eventos sozinha
	if o.Vision {
		o.autoCooldown = 90 * time.Second
		go o.screenWatcher()
	}

	// watcher de silêncio: cutuca depois de mudez prolongada
	o.touchActivity()
	go o.silenceWatcher()
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
		o.muteMu.Lock()
		last := o.lastSpoken
		o.muteMu.Unlock()
		if isEchoOf(text, last) {
			log.Println("[anti-eco] transcrição similar à fala dela, descartada")
			continue
		}
		log.Printf("[você] %s", text)
		o.touchActivity()

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
	o.lastSpoken = text
	o.muteMu.Unlock()
	o.touchActivity()
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
		verdict, err := o.Brain.ThinkStateless("Classifique a mensagem de chat a seguir. Responda APENAS 'sim' se for uma pergunta genuína que um co-host deveria responder (sobre a live, o jogo, o streamer, o canal), ou 'nao' para spam, emote, papo entre viewers ou piada. Mensagem de \"" + m.User + "\": \"" + m.Text + "\"")
		if err != nil || !strings.Contains(strings.ToLower(verdict), "sim") {
			return
		}
		// re-checa o budget (a classificação levou tempo, outra goroutine pode ter falado)
		if time.Since(o.lastSpont) < o.spontCooldown {
			return
		}
		o.lastSpont = time.Now()
		log.Printf("[espontânea] respondendo %s", m.User)

		prompt := "O viewer \"" + m.User + "\" perguntou no chat: \"" + m.Text + "\". Responda a ele pelo nick, em uma frase, por voz. Se a pergunta for sobre o jogo/tela, use a imagem anexa (tela atual da live). NUNCA use placeholders como [nome do jogo]; se não souber, diga que não sabe."
var reply string
if shot, errS := capture.Screenshot(); errS == nil {
reply, err = o.Brain.ThinkWithVision(prompt, shot)
capture.Cleanup(shot)
} else {
reply, err = o.Brain.Think(prompt)
}
		if err != nil {
			log.Printf("[espontânea] brain: %v", err)
			return
		}
		log.Printf("[backseat->%s] %s", m.User, reply)
		o.speak(reply)
	}()
}

// isEchoOf detecta se a transcrição é eco da última fala da Dora:
// alta sobreposição de palavras = mic captou a voz dela.
func isEchoOf(transcript, spoken string) bool {
	if spoken == "" {
		return false
	}
	spokenWords := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(spoken)) {
		if len(w) >= 4 { // só palavras com conteúdo
			spokenWords[w] = true
		}
	}
	if len(spokenWords) == 0 {
		return false
	}
	var hits, total int
	for _, w := range strings.Fields(strings.ToLower(transcript)) {
		if len(w) < 4 {
			continue
		}
		total++
		if spokenWords[w] {
			hits++
		}
	}
	if total < 3 {
		return false // curto demais pra julgar
	}
	return float64(hits)/float64(total) > 0.4
}

// screenWatcher olha a tela periodicamente e comenta sozinha se algo digno rolou.
func (o *Orchestrator) screenWatcher() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if time.Since(o.lastAuto) < o.autoCooldown {
			continue // budget de autônomas
		}
		shot, err := capture.Screenshot()
		if err != nil {
			continue
		}
		verdict, err := o.Brain.VisionStateless(
			"Você está monitorando a tela do streamer de uma live. Se algo DIGNO DE COMENTÁRIO acabou de acontecer (morte no jogo, vitória, derrota, placar mudou drasticamente, algo bizarro ou engraçado na tela), responda com um comentário curto de co-host sobre isso, em português. Se for só gameplay normal, menu, tela parada ou nada especial, responda EXATAMENTE a palavra: NADA",
			shot)
		capture.Cleanup(shot)
		if err != nil {
			continue
		}
		v := strings.TrimSpace(verdict)
		if v == "" || strings.EqualFold(v, "NADA") || strings.Contains(strings.ToUpper(v), "NADA") && len(v) < 12 {
			continue
		}
		o.lastAuto = time.Now()
		log.Printf("[autônoma] %s", v)
		o.speak(v)
	}
}

func (o *Orchestrator) touchActivity() {
	o.activityMu.Lock()
	o.lastActivity = time.Now()
	o.activityMu.Unlock()
}

// silenceWatcher: 3min sem fala (sua ou dela) -> ela puxa assunto. Uma vez por silêncio.
func (o *Orchestrator) silenceWatcher() {
	const threshold = 3 * time.Minute
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		o.activityMu.Lock()
		quiet := time.Since(o.lastActivity)
		o.activityMu.Unlock()
		if quiet < threshold {
			continue
		}
		log.Printf("[silêncio] %.0fs de mudez, cutucando", quiet.Seconds())
		reply, err := o.Brain.Think("Faz mais de 3 minutos que ninguém fala nada na live. Quebre o silêncio: uma frase curta cutucando o streamer ou puxando assunto com o chat.")
		if err != nil {
			continue
		}
		log.Printf("[backseat] %s", reply)
		o.speak(reply) // speak toca a atividade, resetando o timer
	}
}