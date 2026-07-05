
---

## Dívidas técnicas registradas

- **Silero VAD (adiado em 2026-07-05):** ouvido atual é RMS + heurísticas (timestamp de mute + similaridade textual, ~95% dos ecos). Silero como sidecar Python/ONNX mata falsos positivos e distingue fala de música na raiz. Fazer junto do upgrade RTX / repensada do pipeline pra streaming.
- **Classificador de espontâneas polui histórico do brain:** chamadas de classificação entram no history do Client. Isolar em chamada stateless na Fase 5+.
- **Anti-eco definitivo é físico:** cabo de áudio virtual (Fase 9) — a voz dela nunca tocar o ar do quarto. Heurísticas atuais são ponte até lá.
- **Speaker verification (anotado 2026-07-05):** VAD não distingue QUEM fala — pessoas no ambiente viram entrada do streamer. Solução: voice fingerprint (SpeechBrain ECAPA/Resemblyzer, CPU-friendly) como filtro pós-VAD: só a voz cadastrada do streamer passa. Bônus: mata o eco da Dora em definitivo. Fazer junto/depois do Silero.
