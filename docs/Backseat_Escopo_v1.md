
---

## Dívidas técnicas registradas

- **Silero VAD (adiado em 2026-07-05):** ouvido atual é RMS + heurísticas (timestamp de mute + similaridade textual, ~95% dos ecos). Silero como sidecar Python/ONNX mata falsos positivos e distingue fala de música na raiz. Fazer junto do upgrade RTX / repensada do pipeline pra streaming.
- **Classificador de espontâneas polui histórico do brain:** chamadas de classificação entram no history do Client. Isolar em chamada stateless na Fase 5+.
- **Anti-eco definitivo é físico:** cabo de áudio virtual (Fase 9) — a voz dela nunca tocar o ar do quarto. Heurísticas atuais são ponte até lá.
