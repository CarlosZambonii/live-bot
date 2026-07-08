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
	"github.com/CarlosZambonii/backseat/internal/tools"
	"github.com/CarlosZambonii/backseat/internal/config"
	"github.com/CarlosZambonii/backseat/internal/memory"
	"github.com/CarlosZambonii/backseat/internal/mood"
	"github.com/CarlosZambonii/backseat/internal/capture"
	"github.com/CarlosZambonii/backseat/internal/stt"
	"github.com/CarlosZambonii/backseat/internal/voice"
)

type Orchestrator struct {
	STT    *stt.Client
	Brain  *brain.Client
	Voice  *voice.Client
	Cfg    *config.Config
	Chat   chat.Source
	Search *tools.Searcher
	Mood   *mood.State
	VAD    *stt.VADClient
	Memory *memory.Store

	lastMention time.Time
	lastSpont   time.Time
	lastAuto    time.Time
	turnCount   int
	activityMu      sync.Mutex
	lastActivity    time.Time
	speaking        sync.Mutex // serializa quem usa a voz
	segments        chan string
	muteMu          sync.Mutex
	muteUntil       time.Time // segmentos capturados antes disso são eco dela
	lastSpoken      string    // última fala dela (pro filtro de similaridade)
	speechHigh      chan string // fila de fala prioritária (eventos, menções)
	speechLow       chan string // fila de fala secundária (espontâneas, autônomas)
}

func (o *Orchestrator) Run() {
	// fila de fala com prioridade: alta (eventos/menções) antes de baixa (espontâneas)
	o.speechHigh = make(chan string, 8)
	o.speechLow = make(chan string, 8)
	go o.speechWorker()

	// acumula tempo de convívio (nível de relação cresce com as horas juntos)
	if o.Memory != nil {
		go func() {
			tick := time.NewTicker(5 * time.Minute)
			defer tick.Stop()
			for range tick.C {
				total := o.Memory.AddMinutes(5)
				log.Printf("[relação] convívio: %d min acumulados", total)
				// energia decai ~0.08 a cada 5min (Dora cansa ao longo da sessão)
				if o.Mood != nil {
					o.Mood.SetEnergy(o.Mood.Energy() - 0.08)
					log.Printf("[energia] %.2f (%s)", o.Mood.Energy(), o.Mood.EnergyPhase())
				}
			}
		}()
	}

	// chat: consome mensagens e mantém o buffer de contexto atualizado
	if o.Chat != nil {
		buf := chat.NewBuffer(15)
		go func() {
			if err := o.Chat.Start(); err != nil {
				log.Printf("[chat] %v", err)
			}
		}()
		go func() {
			for m := range o.Chat.Messages() {
				log.Printf("[chat] %s: %s", m.User, m.Text)
				if o.Memory != nil {
					if regular := o.Memory.SeeViewer(m.User); regular {
						o.Brain.SetContext(buf.Context() + "\n(Obs: " + m.User + " é um viewer regular, já apareceu em lives anteriores.)")
					}
				}
				buf.Add(m)
				o.Brain.SetContext(buf.Context())

				if mentionsDora(m.Text) {
					cd := o.Cfg.Snapshot().MentionCooldown
					if time.Since(o.lastMention) < cd {
						log.Printf("[menção] %s chamou, mas cooldown ativo (%.0fs restantes)", m.User, (cd - time.Since(o.lastMention)).Seconds())
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

	if o.Memory != nil {
		mem := ""
		if facts := o.Memory.Facts(20); len(facts) > 0 {
			mem += "O que você já sabe sobre o streamer e lives passadas:\n- " + strings.Join(facts, "\n- ")
			log.Printf("[memória] %d fatos carregados", len(facts))
		}
		if obs := o.Memory.Observations(15); len(obs) > 0 {
			mem += "\n\nO que você conhece sobre o jeito do streamer e a relação de vocês:\n- " + strings.Join(obs, "\n- ")
			log.Printf("[relação] %d observações carregadas", len(obs))
		}
		relMin := o.Memory.RelMinutes()
		mem += "\n\nNível de intimidade de vocês: " + relLevelDesc(relMin)
		log.Printf("[relação] %d min de convívio", relMin)
		if o.Mood != nil {
			mem += "\n\nSua energia agora está: " + o.Mood.EnergyPhase() + ". Se estiver cansada ou sonolenta, demonstre isso no jeito de responder (mais lenta, bocejos, menos animada)."
		}
		if mem != "" {
			o.Brain.SetMemory(mem)
		}
		go o.consolidator()
	}

	listener := stt.NewListener(o.Cfg.Snapshot().VADThreshold)
	o.segments = listener.Segments

	// watcher de tela: comenta eventos sozinha
	go o.screenWatcher()

	// watcher de silêncio: cutuca depois de mudez prolongada
	o.touchActivity()
	go o.silenceWatcher()
	go func() {
		if err := listener.Start(); err != nil {
			log.Fatalf("[mic] %v", err)
		}
	}()
	log.Printf("[loop] escuta contínua ligada (config viva em :8090). Ctrl+C para sair.")

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

		if o.VAD != nil && !o.VAD.HasSpeech(wavPath) {
			os.Remove(wavPath)
			log.Println("[vad] segmento sem fala humana, descartado")
			continue
		}
		tSTT := time.Now()
		text, err := o.STT.Transcribe(wavPath)
		log.Printf("[t] stt: %.1fs", time.Since(tSTT).Seconds())
		os.Remove(wavPath)
		if err != nil {
			log.Printf("[stt] %v", err)
			continue
		}
		text = strings.TrimSpace(text)
		if !isRealSpeech(text) || isLooping(text) {
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
		o.Brain.SetPersona(o.Cfg.Snapshot().Persona)
		if o.Memory != nil {
			o.Memory.AppendSession("streamer", text)
		}

		// busca proativa: se a fala pede fato atual, busca antes e injeta no contexto
		if o.Search != nil && o.Search.Enabled() && needsSearch(text) {
			if o.Mood != nil { o.Mood.SetAnim("Thinking") }
			if res, err := o.Search.Search(text); err == nil {
				log.Printf("[busca] %s", text)
				text = text + "\n\n[Resultado de busca web atual, use para responder]:\n" + res
			}
		}

		var reply string
		// comandos de ação (volume/música) vão pro Think com tools, sem visão
		if isActionCommand(text) {
			if r, err := o.Brain.Think(text); err == nil {
				reply = r
			} else {
				log.Printf("[brain] %v", err)
				continue
			}
		} else if o.Cfg.Snapshot().Vision {
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
		log.Printf("[backseat] %s", stripMood(reply))

		tVoice := time.Now()
		o.speak(reply)
		log.Printf("[t] voz (gerar+tocar): %.1fs", time.Since(tVoice).Seconds())
		log.Printf("[latência] %.1fs (fala->fim da resposta)", time.Since(start).Seconds())

		o.turnCount++
		if o.Memory != nil && o.turnCount%6 == 0 {
			go o.updateNarrative()
		}


	}
}

// speak centraliza toda fala da Dora: serializa a boca e drena o eco.
// speechWorker consome a fila de fala, sempre priorizando a alta.
func (o *Orchestrator) speechWorker() {
	for {
		select {
		case t := <-o.speechHigh:
			o.speakNow(t)
		default:
			select {
			case t := <-o.speechHigh:
				o.speakNow(t)
			case t := <-o.speechLow:
				o.speakNow(t)
			}
		}
	}
}

// speak enfileira uma fala de prioridade normal/baixa.
func (o *Orchestrator) speak(text string) {
	o.speechLow <- text
}

// speakPriority enfileira uma fala prioritária (eventos, menções).
func (o *Orchestrator) speakPriority(text string) {
	o.speechHigh <- text
}

func (o *Orchestrator) speakNow(text string) {
	o.speaking.Lock()
	defer o.speaking.Unlock()
	// (execução real da fala; a ordenação por prioridade acontece em speak())
	// extrai o humor do prefixo [humor] e limpa o texto (fonte única pra todas as rotas)
	if o.Mood != nil {
		m, clean := brain.ExtractMood(text)
		if m != "neutra" || strings.HasPrefix(strings.TrimSpace(text), "[") {
			o.Mood.Set(m)
			if a := animForMood(m); a != "" {
				o.Mood.SetAnim(a)
			}
		}
		text = clean
		o.Mood.SetSpeaking(true)
		o.Mood.SetLastSaid(text)
		defer o.Mood.SetSpeaking(false)
	}
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
	log.Printf("[backseat->%s] %s", m.User, stripMood(reply))
	if o.Mood != nil {
		gestos := []string{"Goodbye", "LookAround"}
		o.Mood.SetAnim(gestos[time.Now().UnixNano()%2])
	}
	o.speakPriority(reply)
}

// maybeAnswerSpontaneous decide se responde uma mensagem que NÃO menciona a Dora.
// Travas em ordem barata->cara: budget -> dado -> classificador LLM -> resposta.
func (o *Orchestrator) maybeAnswerSpontaneous(m chat.Message) {
	snap := o.Cfg.Snapshot()
	if time.Since(o.lastSpont) < snap.SpontCooldown {
		return // budget estourado, nem gasta classificação
	}
	if rand.Float64() > snap.SpontChance {
		return // dado: 40% das candidatas morrem aqui, mantém imprevisível
	}
	go func() {
		verdict, err := o.Brain.ThinkStateless("Classifique a mensagem de chat a seguir. Responda APENAS 'sim' se for uma pergunta genuína que um co-host deveria responder (sobre a live, o jogo, o streamer, o canal), ou 'nao' para spam, emote, papo entre viewers ou piada. Mensagem de \"" + m.User + "\": \"" + m.Text + "\"")
		if err != nil || !strings.Contains(strings.ToLower(verdict), "sim") {
			return
		}
		// re-checa o budget (a classificação levou tempo, outra goroutine pode ter falado)
		if time.Since(o.lastSpont) < o.Cfg.Snapshot().SpontCooldown {
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
		log.Printf("[backseat->%s] %s", m.User, stripMood(reply))
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

// isLooping detecta alucinação repetitiva do Whisper: vocabulário minúsculo pra texto longo.
func isLooping(text string) bool {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 15 {
		return false
	}
	uniq := map[string]bool{}
	for _, w := range words {
		uniq[w] = true
	}
	return float64(len(uniq))/float64(len(words)) < 0.2 // menos de 20% de palavras únicas = loop
}

// screenWatcher olha a tela periodicamente e comenta sozinha se algo digno rolou.
func (o *Orchestrator) screenWatcher() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		snap := o.Cfg.Snapshot()
		if !snap.Vision {
			continue
		}
		if time.Since(o.lastAuto) < snap.AutoCooldown {
			continue // budget de autônomas
		}
		shot, err := capture.Screenshot()
		if err != nil {
			continue
		}
		verdict, err := o.Brain.VisionStateless(
			"Você monitora a tela do streamer. Se algo digno aconteceu, responda no formato: TIPO|comentário. TIPO é uma palavra: MORTE, VITORIA, PERIGO, ENGRACADO ou OUTRO. Exemplo: MORTE|morreu de novo, hein campeão. Se for gameplay normal/menu/nada, responda só: NADA",
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
		// separa TIPO|comentário
		tipo, comentario := "OUTRO", v
		if i := strings.Index(v, "|"); i > 0 {
			tipo = strings.ToUpper(strings.TrimSpace(v[:i]))
			comentario = strings.TrimSpace(v[i+1:])
		}
		v = comentario
		if o.Mood != nil {
			switch tipo {
			case "MORTE":
				o.Mood.SetAnim("Sad")
			case "VITORIA":
				o.Mood.SetAnim("Clapping")
			case "PERIGO":
				o.Mood.SetAnim("Surprised")
			case "ENGRACADO":
				o.Mood.SetAnim("Jump")
			default:
				o.Mood.SetAnim("LookAround")
			}
		}
		log.Printf("[autônoma:%s] %s", tipo, v)
		o.speakPriority(v)
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
		if o.Mood != nil { o.Mood.SetAnim("LookAround") }
		reply, err := o.Brain.Think("Faz mais de 3 minutos que ninguém fala nada na live. Quebre o silêncio: uma frase curta cutucando o streamer ou puxando assunto com o chat.")
		if err != nil {
			continue
		}
		log.Printf("[backseat] %s", stripMood(reply))
		o.speak(reply) // speak toca a atividade, resetando o timer
	}
}

func (o *Orchestrator) consolidator() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		transcript := o.Memory.SessionTranscript()
		if len(transcript) < 200 {
			continue
		}
		out, err := o.Brain.ThinkStateless("Abaixo está a transcrição recente de uma live. Extraia até 3 fatos DURADOUROS que valem lembrar em lives futuras (preferências do streamer, eventos marcantes, piadas internas, nomes citados). Um por linha, frases curtas. Se nada valer a pena, responda NADA.\n\n" + transcript)
		if err != nil {
			continue
		}
		out = strings.TrimSpace(out)
		if strings.EqualFold(out, "NADA") {
			continue
		}
		n := 0
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
			if len(line) > 10 {
				if o.Memory.AddFact(line) == nil {
					n++
				}
			}
		}
		if n > 0 {
			log.Printf("[memória] %d fatos consolidados", n)
		}
		// extrai observações de relação (padrões, jeito, vínculo)
		obs, err2 := o.Brain.ThinkStateless("Com base na conversa abaixo, extraia até 2 observações sobre o COMPORTAMENTO e JEITO do streamer (como ele reage, humor, manias, como trata a Dora, padrões). Não fatos objetivos, mas traços de personalidade e relação. Uma por linha, frase curta. Se nada relevante, responda NADA.\n\n" + transcript)
		if err2 == nil {
			obs = strings.TrimSpace(obs)
			if !strings.EqualFold(obs, "NADA") {
				for _, line := range strings.Split(obs, "\n") {
					line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
					if len(line) > 10 {
						o.Memory.AddObservation(line)
					}
				}
				log.Printf("[relação] observações consolidadas")
			}
		}
		// extrai/cria um bordão interno da sessão
		bordao, err3 := o.Brain.ThinkStateless("Com base na conversa abaixo, crie UM bordão curto ou piada interna que a co-host Dora poderia repetir nas próximas lives com este streamer (algo memorável que rolou). Máximo 8 palavras. Se nada rende, responda NADA.\n\n" + transcript)
		if err3 == nil {
			bordao = strings.TrimSpace(bordao)
			if !strings.EqualFold(bordao, "NADA") && len(bordao) > 5 && len(bordao) < 80 {
				o.Memory.AddObservation("Bordão interno pra reusar: " + bordao)
				log.Printf("[bordão] %s", bordao)
			}
		}
		o.Memory.ClearSession()
	}
}

// needsSearch detecta se a fala precisa de fato atual da web.
func needsSearch(text string) bool {
	t := strings.ToLower(text)
	kw := []string{"patch", "versão", "versao", "atual", "último", "ultimo", "última", "ultima",
		"adicionad", "lançad", "lancad", "novo campeão", "novo campeao", "notícia", "noticia",
		"que dia", "que horas são", "hoje é", "quem ganhou", "resultado", "quando sai", "quando lança"}
	for _, k := range kw {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}

// updateNarrative destila o transcript recente num resumo do "agora" da live.
func (o *Orchestrator) updateNarrative() {
	transcript := o.Memory.SessionTranscript()
	if len(transcript) < 150 {
		return
	}
	out, err := o.Brain.ThinkStateless("Resuma em 2 frases curtas o que está acontecendo AGORA nesta live, com base na conversa recente abaixo. Foque no momento atual: o que o streamer está fazendo, o clima, assuntos em andamento. Escreva como uma nota de contexto, não como diálogo.\n\n" + transcript)
	if err != nil {
		return
	}
	o.Brain.SetNarrative(strings.TrimSpace(out))
	log.Printf("[narrativa] atualizada")
}

// animForMood devolve a animação VRMA que combina com o humor (ou "" pra nenhuma).
func animForMood(m string) string {
	switch m {
	case "surpresa":
		return "Surprised"
	case "animada":
		return "Clapping"
	case "provocada":
		return "Angry"
	case "entediada":
		return "Sleepy"
	case "zoeira":
		return "Blush"
	default:
		return ""
	}
}

// stripMood remove o prefixo [humor] pra log limpo.
func stripMood(s string) string {
	_, clean := brain.ExtractMood(s)
	return clean
}

// relLevelDesc traduz minutos de convívio numa instrução de tom pra Dora.
func relLevelDesc(min int) string {
	switch {
	case min < 60:
		return "Vocês ainda se conhecem pouco. Seja simpática mas um pouco mais formal e reservada, como quem está conhecendo alguém."
	case min < 300:
		return "Vocês já têm alguma convivência. Pode ser mais descontraída e brincalhona, com intimidade moderada."
	case min < 900:
		return "Vocês são próximos agora. Pode usar apelidos carinhosos, brincadeiras internas e ser bem à vontade."
	default:
		return "Vocês são muito próximos, quase cúmplices. Seja atrevida, debochada no bom sentido, com total intimidade e piadas particulares de vocês."
	}
}

// HandleReward processa uma recompensa resgatada pelo chat (channel points).
// Cada tipo aciona uma reação: energia, humor, animação e uma fala curta.
func (o *Orchestrator) HandleReward(tipo, user string) {
	log.Printf("[recompensa] %s resgatou: %s", user, tipo)
	if o.Mood == nil {
		return
	}
	var fala string
	switch tipo {
	case "agua":
		o.Mood.SetEnergy(o.Mood.Energy() + 0.4) // recupera energia
		o.Mood.SetForce("animada")
		o.Mood.SetAnim("Clapping")
		fala = user + " me deu água! Ahh, revigorada! Valeu demais!"
	case "cutucar":
		o.Mood.SetForce("provocada")
		o.Mood.SetAnim("Angry")
		fala = "Ei, " + user + ", parou de me cutucar! Kkk"
	case "dancar":
		o.Mood.SetForce("animada")
		o.Mood.SetAnim("Clapping")
		fala = "Bora que o " + user + " pediu dança!"
	case "dormir":
		o.Mood.SetEnergy(0.1)
		o.Mood.SetForce("entediada")
		o.Mood.SetAnim("Sleepy")
		fala = "Hmm... o " + user + " quer que eu tire uma soneca... *boceja*"
	case "elogiar":
		o.Mood.SetForce("animada")
		o.Mood.SetAnim("Blush")
		fala = "Awn, obrigada " + user + "! Fiquei toda boba agora."
	default:
		fala = "Valeu pela recompensa, " + user + "!"
	}
	o.speakPriority(fala)
}

// isActionCommand detecta pedidos de ação (volume, música) que precisam das tools.
func isActionCommand(text string) bool {
	t := strings.ToLower(text)
	kw := []string{"volume", "som", "música", "musica", "pausa", "pause", "toca", "tocar", "próxima", "proxima", "pula", "abaixa", "aumenta", "diminui", "mais alto", "mais baixo", "skin", "aparência", "aparencia", "roupa", "visual", "veste", "muda de"}
	for _, k := range kw {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}