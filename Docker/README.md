# Docker — College Schedule Bot

Самодостаточная папка: содержит исходники, конфиги и скрипты для сборки и запуска в Docker.

## Быстрый старт

```bash
cd Docker
docker compose up -d
```

## Пошагово

### 1. Установка Docker (если нет)
```bash
bash install.sh
```

### 2. Сборка и запуск
```bash
cd Docker
docker compose up -d
```

Сборка займёт ~1-2 минуты (скачивание Go, компиляция, alpine).

### 3. Проверка
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

### 4. Остановка
```bash
docker compose down
```

### 5. Перезапуск с новым туннелем
```bash
docker compose restart
```
Каждый рестарт контейнера создаёт новый туннель и регистрирует webhook.

## Сборка без compose

```bash
docker build -t college-bot .
docker run -d --restart unless-stopped -p 8080:8080 --name college-bot college-bot
```

## Структура папки

```
Docker/
├── Dockerfile           # сборка (мультистейдж)
├── entrypoint.sh        # автотуннель при старте
├── docker-compose.yml   # docker compose up -d
├── install.sh           # установка Docker
├── README.md
├── cmd/bot/main.go      # исходники Go
├── internal/            # Go-пакеты
├── go.mod / go.sum      # зависимости
├── *.json               # конфиги
└── tools/               # Python-скрипты
```
