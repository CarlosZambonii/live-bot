
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
