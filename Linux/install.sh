#!/bin/bash
# College Schedule Bot — Установка зависимостей (Linux)
set -e

echo "========================================"
echo " College Schedule Bot — Установка"
echo " Linux"
echo "========================================"
echo ""

# 1. cloudflared
echo "[1/3] Cloudflare Tunnel (cloudflared)..."
if command -v cloudflared &>/dev/null; then
    echo "  ✅ Уже установлен: $(cloudflared --version | head -1)"
else
    echo "  ⬇️ Скачиваю cloudflared..."
    curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
        -o /usr/local/bin/cloudflared
    chmod +x /usr/local/bin/cloudflared
    echo "  ✅ cloudflared установлен в /usr/local/bin/cloudflared"
fi

# 2. Python 3
echo "[2/3] Python 3..."
if command -v python3 &>/dev/null; then
    echo "  ✅ $(python3 --version)"
else
    echo "  ⬇️ Устанавливаю Python 3..."
    if command -v apt &>/dev/null; then
        apt update -qq && apt install -y -qq python3 python3-pip
    elif command -v yum &>/dev/null; then
        yum install -y python3 python3-pip
    elif command -v apk &>/dev/null; then
        apk add python3 py3-pip
    else
        echo "  ❌ Менеджер пакетов не найден. Установи Python 3 вручную."
        exit 1
    fi
    echo "  ✅ Python 3 установлен"
fi

# 3. Python-библиотеки
echo "[3/3] Python-библиотеки..."
pip3 install openpyxl 2>&1 | grep -q "already satisfied" && echo "  ✅ openpyxl уже установлен" || {
    pip3 install openpyxl && echo "  ✅ openpyxl установлен"
}

echo ""
echo "========================================"
echo "  ✅ Готово! Все зависимости установлены."
echo ""
echo "  Запусти: bash start.sh"
echo "========================================"
