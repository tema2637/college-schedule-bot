#!/bin/bash
# College Schedule Bot — Установка Docker (если ещё не установлен)
set -e

echo "========================================"
echo " College Schedule Bot — Docker"
echo " Установка Docker Engine + Compose"
echo "========================================"
echo ""

# Проверяем Docker
if command -v docker &>/dev/null; then
    echo "  ✅ Docker уже установлен: $(docker --version)"
else
    echo "  ⬇️ Устанавливаю Docker Engine..."
    curl -fsSL https://get.docker.com | sh
    echo "  ✅ Docker установлен"
fi

# Проверяем Docker Compose
if docker compose version &>/dev/null; then
    echo "  ✅ Docker Compose уже установлен: $(docker compose version)"
elif docker-compose --version &>/dev/null; then
    echo "  ✅ Docker Compose уже установлен (v1)"
else
    echo "  ⬇️ Устанавливаю Docker Compose..."
    curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" \
        -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    echo "  ✅ Docker Compose установлен"
fi

echo ""
echo "========================================"
echo "  ✅ Готово!"
echo ""
echo "  Запусти:"
echo "    cd /root/college-schedule-distro/Docker"
echo "    docker compose up -d"
echo "========================================"
