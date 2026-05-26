# Windows — College Schedule Bot

## Состав папки
- `start.bat` — скрипт запуска с автотуннелем
- `college-schedule-bot.exe` — бинарник бота (если есть)
- `config.json` — конфиг (бот обновит `webhook_url` сам)
- `schedule.json` — файл расписания
- `changes.json` — файл изменений
- `messages.json` — шаблоны сообщений
- `users.json` — пользователи
- `tools/` — Python-скрипты для парсинга

## Требования
- [cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) (скачать exe, положить в PATH или в эту же папку)

## Запуск

### 1. Подготовка
Убедись, что `cloudflared.exe` доступен из командной строки:
```cmd
cloudflared --version
```
Если нет — скачай и положи exe в эту же папку или добавь в PATH.

### 2. Запуск
Просто открой `start.bat` двойным кликом (или из cmd):

```cmd
start.bat
```

Скрипт сделает всё сам:
```
[1/5] Останавливаю старые процессы
[2/5] Запускаю новый туннель Cloudflare
[3/5] Ожидаю URL туннеля...
[4/5] Обновляю config.json
[5/5] Запускаю бота
```

### 3. Проверка
Бот запущен в фоне. Окно можно закрыть — процессы останутся работать.

Если нужно остановить — вручную:
```cmd
taskkill /f /im cloudflared.exe
taskkill /f /im college-schedule-bot.exe
```

### 4. Перезапуск с новым туннелем
Просто запусти `start.bat` снова — он сам убьёт старые процессы и создаст новый туннель.
