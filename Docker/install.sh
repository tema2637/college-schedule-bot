#!/bin/bash
echo "=== College Schedule Bot — Docker ==="
echo ""
if command -v docker &>/dev/null; then
    echo "✅ Docker: $(docker --version)"
else
    echo "⬇️ Устанавливаю Docker..."
    curl -fsSL https://get.docker.com | sh
fi
docker compose version &>/dev/null && echo "✅ Compose: $(docker compose version)" || {
    echo "⬇️ Устанавливаю Compose..."
    curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
}
echo ""
echo "Запуск: docker compose up -d"
