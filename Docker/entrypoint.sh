#!/bin/bash
set -e
CONFIG="/app/config.json"
WEBHOOK_URL=""

echo "🚀 College Schedule Bot — Docker"
echo "📡 Создание туннеля Cloudflare..."
TUNNEL_LOG=$(mktemp)
cloudflared tunnel --url http://localhost:8080 > "$TUNNEL_LOG" 2>&1 &
TUNNEL_PID=$!

ATTEMPTS=0
while [ -z "$WEBHOOK_URL" ] && [ $ATTEMPTS -lt 30 ]; do
    sleep 1
    TUNNEL_URL=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$TUNNEL_LOG" | head -1)
    if [ -n "$TUNNEL_URL" ]; then WEBHOOK_URL="${TUNNEL_URL}/webhook"; fi
    ATTEMPTS=$((ATTEMPTS + 1))
done

if [ -z "$WEBHOOK_URL" ]; then
    echo "❌ Не удалось получить URL туннеля"
    cat "$TUNNEL_LOG"; exit 1
fi

echo "🔗 Webhook URL: $WEBHOOK_URL"
python3 -c "
import json
cfg = json.load(open('$CONFIG'))
cfg['webhook_url'] = '$WEBHOOK_URL'
with open('$CONFIG', 'w') as f: json.dump(cfg, f, indent=2)
"
echo "📝 config.json обновлён"
echo "🤖 Запуск бота..."
cd /app
exec ./bot
