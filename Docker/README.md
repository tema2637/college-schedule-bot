# Docker — College Schedule Bot

## Состав папки
- `Dockerfile` — сборка образа (мультистейдж: Go build → Alpine)
- `entrypoint.sh` — скрипт запуска внутри контейнера
- `docker-compose.yml` — для удобного запуска
- `config.json` — конфиг (обновится автоматически)
- `schedule.json` — файл расписания
- `changes.json` — файл изменений
- `messages.json` — шаблоны сообщений
- `users.json` — пользователи
- `tools/` — Python-скрипты для парсинга

## Требования
- Docker Engine (>= 20.x)
- Docker Compose (>= 2.x)

## Запуск

### 1. Сборка и запуск
```bash
cd /root/college-schedule-distro/Docker
docker compose up -d
```

Docker compose сделает:
- сборку образа (скачает golang, зависимости, скомпилирует бота)
- при старте контейнера: новый туннель → обновление конфига → запуск бота

### 2. Проверка
```bash
docker compose logs -f
```

Должен увидеть:
```
🚀 College Schedule Bot — Docker
📡 Создание туннеля Cloudflare...
🔗 Webhook URL: https://xxx.trycloudflare.com/webhook
📝 config.json обновлён
🤖 Запуск бота...
```

### 3. Остановка
```bash
docker compose down
```

### 4. Перезапуск с новым туннелем
```bash
docker compose restart
```
Каждый рестарт контейнера создаёт новый туннель и регистрирует webhook.

### 5. Обновление бота (пересборка)
Если изменился код:
```bash
docker compose build --no-cache
docker compose up -d
```
