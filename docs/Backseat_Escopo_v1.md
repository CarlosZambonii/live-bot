
---

## Dívidas técnicas registradas

- **Silero VAD (adiado em 2026-07-05):** ouvido atual é RMS + heurísticas (timestamp de mute + similaridade textual, ~95% dos ecos). Silero como sidecar Python/ONNX mata falsos positivos e distingue fala de música na raiz. Fazer junto do upgrade RTX / repensada do pipeline pra streaming.
- **Classificador de espontâneas polui histórico do brain:** chamadas de classificação entram no history do Client. Isolar em chamada stateless na Fase 5+.
- **Anti-eco definitivo é físico:** cabo de áudio virtual (Fase 9) — a voz dela nunca tocar o ar do quarto. Heurísticas atuais são ponte até lá.
- **Speaker verification (anotado 2026-07-05):** VAD não distingue QUEM fala — pessoas no ambiente viram entrada do streamer. Solução: voice fingerprint (SpeechBrain ECAPA/Resemblyzer, CPU-friendly) como filtro pós-VAD: só a voz cadastrada do streamer passa. Bônus: mata o eco da Dora em definitivo. Fazer junto/depois do Silero.

---

## Features futuras — Dora que AGE (não só fala)

Evolução das tools (function calling) — hoje ela tem buscar_web; adicionar:

### Música
- Tocar/pausar/pular no Spotify (via playerctl/MPRIS ou API OAuth)
- Anunciar a faixa que entrou
- playerctl funciona com Spotify, navegador, qualquer player MPRIS

### Volume (Pop!_OS)
- Subir/baixar volume por comando de voz ("Dora, abaixa a música")
- Controlar apps separados: música vs jogo vs voz dela (pactl set-sink-input-volume)

### OBS (produção de live)
- Trocar cena por comando (obs-websocket)
- Ligar/desligar mic, câmera
- Disparar alertas/efeitos na tela

### Arquitetura
- Cada ação = uma tool nova no brain (controlar_musica, ajustar_volume, trocar_cena_obs)
- GPT decide quando chamar, Go executa o comando no sistema
- Tudo fazível sem GPU

### Interação com recompensas — DUAS camadas

**1. Channel Points (grátis, pontos por assistir)** — o principal
- Viewer troca pontos acumulados por ações na Dora
- Itens: "dar água" → bebe, "fazer dançar" → dança, "cutucar" → provocada, "elogiar" → animada, "dormir" → Sleepy
- Fonte técnica: Twitch EventSub "channel.channel_points_custom_reward_redemption.add"
- Barato, alto engajamento, o público "cuida/brinca" com ela = tamagotchi ao vivo

**2. Recompensas pagas (subs, bits, donate)** — reações especiais
- Sub novo → Dora agradece pelo nick + animação comemorativa (Clapping/Jump)
- Bits/cheer → reação proporcional ao valor
- Donate → ela lê/reage à mensagem
- Fonte: EventSub (subs/bits) + integração de doação (StreamElements/StreamlabsX)

Ambos: EventSub → gatilho no orchestrator → SetAnim + fala contextual.
Sistema de animação por gatilho JÁ pronto — falta só ligar o EventSub.

---

## Backlog de ideias — "deixar a Dora mais viva" (filtrado)

### Personalidade viva ✓
- Relacionamento que evolui (mais íntima/atrevida ao longo de semanas, nível de amizade)
- Opinião própria (torce por time, gosta/odeia jogos, gostos que aparecem sozinhos)
- Humor muda com contexto (mau humor se perde muito, hype em win streak, sonolenta de madrugada)

### Voz e áudio ✓
- Cantarolar/reagir a música tocando
- Sussurrar vs gritar (tom conforme o momento)
- Risada/gargalhada real
- Gírias regionais

### Percepção ✓
- Reconhecer o jogo e dar dicas reais SÓ SE PERGUNTADO (busca+visão) — não palpitar sozinha
- Ler melhores comentários do chat em voz alta
- Reagir à webcam (se usar)
- Perceber horário ("boa madrugada, insônia de novo?")

### Gamificação da Dora ✓ (o tamagotchi)
- Stats visíveis: humor, energia, "fome"
- Fases do dia: acorda, cansa, dorme ao longo da live
- Roupas/skins desbloqueáveis por marcos do canal
- Comemora marcos (X seguidores, aniversário dela)

### Apostas com channel points ✓ (grátis)
- Viewers apostam pontos no resultado de algo, Dora conduz a aposta

### CORTADO (não fazer — fica chato):
- Palpitar em decisões sem ser perguntado
- Comentário esportivo/narração
- Existência fora da live (postar sozinha, sentir falta, DM)
- As "insanas" (duas Doras, modo história/lore, reconhecer convidados)
