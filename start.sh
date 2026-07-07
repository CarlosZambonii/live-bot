#!/bin/bash
# Sobe tudo que o Backseat precisa, em ordem.
set -e
cd ~/backseat

echo "→ Postgres..."
docker compose -f deploy/docker-compose.yml up -d

echo "→ VAD sidecar..."
(cd ~/backseat/sidecar && python3 vad_server.py &)

echo "→ Voicebox (novo terminal)..."
# abre o voicebox num terminal separado
if command -v cosmic-term &>/dev/null; then
  cosmic-term -- bash -c "cd ~/voicebox && just dev-web" &
elif command -v gnome-terminal &>/dev/null; then
  gnome-terminal -- bash -c "cd ~/voicebox && just dev-web" &
else
  echo "  (suba o voicebox manualmente: cd ~/voicebox && just dev-web)"
fi

echo "→ aguardando serviços (15s)..."
sleep 15

echo "→ Backseat!"
export $(grep -v '^#' .env | xargs)
go run ./cmd/backseat
