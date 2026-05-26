#!/bin/bash
# College Schedule Bot — Setup
# Генерирует config.json из .env
set -e

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

# Проверяем .env
if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        echo "⚠️  .env не найден. Создаю из .env.example..."
        cp .env.example .env
        echo "✏️  Отредактируй .env и запусти setup.sh снова"
        echo "   nano .env"
        exit 1
    else
        echo "❌ .env и .env.example не найдены"
        exit 1
    fi
fi

echo "📝 Читаю .env..."

# Загружаем переменные (без export чтобы не засорять среду)
set -a
source .env
set +a

# Проверяем обязательные поля
if [ "$BOT_TOKEN" = "тут_твой_токен" ] || [ -z "$BOT_TOKEN" ]; then
    echo "❌ BOT_TOKEN не установлен в .env"
    exit 1
fi

if [ -z "$SEMESTER_START" ]; then
    SEMESTER_START="11.01.2026"
    echo "⚠️  SEMESTER_START не указан, ставлю $SEMESTER_START"
fi

# Генерируем config.json
echo "⚙️  Генерация config.json..."
cat > config.json << JSON
{
  "api_token": "${BOT_TOKEN}",
  "semester_start_date": "${SEMESTER_START}",
  "files": {
    "schedule": "${SCHEDULE_FILE:-schedule.json}",
    "users": "${USERS_FILE:-users.json}",
    "messages": "${MESSAGES_FILE:-messages.json}",
    "admins": "${ADMINS_FILE:-admins.json}"
  },
  "tools_dir": "${TOOLS_DIR:-tools}",
  "webhook_url": "",
  "webhook_port": "${WEBHOOK_PORT:-:8080}",
  "webhook_path": "${WEBHOOK_PATH:-/webhook}"
}
JSON

echo "✅ config.json создан"

# Копируем в дистрибутивы
echo "📦 Синхронизация с дистрибутивами..."
for dist in Windows Linux Docker; do
    if [ -d "$dist" ]; then
        cp config.json "$dist/"
    fi
done

echo ""
echo "========================================"
echo "  ✅ Готово! Запусти бота:"
echo ""
echo "  Прямой запуск:  bash start.sh"
echo "  Docker:         cd Docker && docker compose up -d"
echo "========================================"
