# Linux — College Schedule Bot

## Состав папки
- `start.sh` — скрипт запуска с автотуннелем
- `bot` — скомпилированный бинарник (Go)
- `config.json` — конфиг (бот обновит `webhook_url` сам)
- `schedule.json` — файл расписания
- `changes.json` — файл изменений
- `messages.json` — шаблоны сообщений
- `users.json` — пользователи
- `tools/` — Python-скрипты для парсинга

## Требования
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) — установить:
  ```bash
  # Linux amd64:
  curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /usr/local/bin/cloudflared
  chmod +x /usr/local/bin/cloudflared
  ```
- Python 3 (для парсинга xlsx)

## Запуск

### 1. Подготовка
Убедись, что cloudflared установлен:
```bash
cloudflared --version
```

### 2. Запуск
```bash
cd /root/college-schedule-distro/Linux
bash start.sh
```

Скрипт сделает:
```
[1/5] Останавливаю старые процессы
[2/5] Запускаю новый туннель Cloudflare
[3/5] Ожидаю URL туннеля...
[4/5] Обновляю config.json
[5/5] Запускаю бота
```

### 3. Остановка
```bash
pkill -f "cloudflared tunnel"
pkill -f "./bot"
```

### 4. Перезапуск с новым туннелем
Просто запусти `start.sh` снова — он сам убьёт старые процессы и создаст новый туннель.

### 5. Автозапуск (опционально — systemd)
```bash
cp /root/college-schedule-distro/college-bot.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now college-bot
```
