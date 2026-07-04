# Backseat

Co-host de IA pra live: ve a tela, ouve voce, le o chat e participa por voz.

## Stack
- Orquestrador: Go (loop tempo real, trigger engine)
- Cerebro + visao: OpenAI (gpt-4o-mini)
- Voz + ouvido: Voicebox local (Kokoro/CPU -> Chatterbox/GPU)

## Rodar
    cp .env.example .env   # preencha OPENAI_API_KEY
    go run ./cmd/backseat

## Infra (Fase 5)
    docker compose -f deploy/docker-compose.yml up -d
