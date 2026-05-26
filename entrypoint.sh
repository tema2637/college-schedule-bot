#!/bin/sh
set -e

echo "🚀 College Schedule Bot — Docker (optimized)"
echo "📡 Creating Cloudflare tunnel..."

TUNNEL_LOG=$(mktemp)
cloudflared tunnel --url http://localhost:8080 > "$TUNNEL_LOG" 2>&1 &
TUNNEL_PID=$!

WEBHOOK_URL=""
ATTEMPTS=0
while [ -z "$WEBHOOK_URL" ] && [ $ATTEMPTS -lt 30 ]; do
    sleep 1
    WEBHOOK_URL=$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$TUNNEL_LOG" | head -1)
    ATTEMPTS=$((ATTEMPTS + 1))
done

if [ -z "$WEBHOOK_URL" ]; then
    echo "❌ Failed to get tunnel URL"
    cat "$TUNNEL_LOG"
    exit 1
fi

echo "🔗 Tunnel URL: $WEBHOOK_URL"

# Export webhook URL for the bot
export WEBHOOK_URL="${WEBHOOK_URL}/webhook"

echo "🤖 Starting bot..."
exec /app/bot
