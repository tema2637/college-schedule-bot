#!/bin/bash
# Автозапуск бота с автоматической сменой туннеля
set -e

DIR="/root/college-schedule-bot"
CONFIG="$DIR/config.json"

echo "🚀 Запуск колледж-бота с автотуннелем..."

# 1. Убиваем старые процессы
pkill -f "cloudflared tunnel" || true
pkill -f "$DIR/\./bot" || true
sleep 1

# 2. Запускаем cloudflared, захватываем URL
echo "📡 Создание туннеля Cloudflare..."
TUNNEL_LOG=$(mktemp)
cloudflared tunnel --url http://localhost:8080 > "$TUNNEL_LOG" 2>&1 &
TUNNEL_PID=$!

ATTEMPTS=0
TUNNEL_URL=""
while [ -z "$TUNNEL_URL" ] && [ $ATTEMPTS -lt 30 ]; do
    sleep 1
    TUNNEL_URL=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$TUNNEL_LOG" | head -1)
    ATTEMPTS=$((ATTEMPTS + 1))
done

if [ -z "$TUNNEL_URL" ]; then
    echo "❌ Не удалось получить URL туннеля"
    kill $TUNNEL_PID 2>/dev/null || true
    exit 1
fi

WEBHOOK_URL="${TUNNEL_URL}/webhook"
echo "🔗 Новый URL: $WEBHOOK_URL"

# 3. Обновляем config.json
echo "📝 Обновление config.json..."
python3 -c "
import json
cfg = json.load(open('$CONFIG'))
cfg['webhook_url'] = '$WEBHOOK_URL'
with open('$CONFIG', 'w') as f:
    json.dump(cfg, f, indent=2)
"

# 4. Запускаем бота (сам зарегистрирует webhook)
echo "🤖 Старт бота..."
cd "$DIR"
nohup ./bot > bot.log 2>&1 &
BOT_PID=$!
echo "Бот PID: $BOT_PID"

echo "✅ Запущено: cloudflared PID=$TUNNEL_PID, bot PID=$BOT_PID"
echo "📡 Webhook: $WEBHOOK_URL"
