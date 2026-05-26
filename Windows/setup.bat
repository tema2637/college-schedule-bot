@echo off
chcp 65001 >nul
title College Bot — Setup

echo ========================================
echo  College Schedule Bot — Setup
echo ========================================
echo.

:: Check .env
if not exist .env (
    if exist .env.example (
        echo ⚠️  .env не найден. Создаю из .env.example...
        copy .env.example .env >nul
        echo ✏️  Отредактируй .env и запусти setup.bat снова
        pause
        exit /b 1
    ) else (
        echo ❌ .env и .env.example не найдены
        pause
        exit /b 1
    )
)

echo 📝 Читаю .env...

:: Read .env (simple parser)
for /f "tokens=1,* delims==" %%a in (.env) do (
    if "%%a"=="BOT_TOKEN" set BOT_TOKEN=%%b
    if "%%a"=="SEMESTER_START" set SEMESTER_START=%%b
    if "%%a"=="WEBHOOK_PORT" set WEBHOOK_PORT=%%b
    if "%%a"=="WEBHOOK_PATH" set WEBHOOK_PATH=%%b
    if "%%a"=="SCHEDULE_FILE" set SCHEDULE_FILE=%%b
    if "%%a"=="USERS_FILE" set USERS_FILE=%%b
    if "%%a"=="MESSAGES_FILE" set MESSAGES_FILE=%%b
    if "%%a"=="ADMINS_FILE" set ADMINS_FILE=%%b
    if "%%a"=="TOOLS_DIR" set TOOLS_DIR=%%b
)

if "%BOT_TOKEN%"=="" (
    echo ❌ BOT_TOKEN не установлен в .env
    pause
    exit /b 1
)
if "%BOT_TOKEN%"=="тут_твой_токен" (
    echo ❌ Замени BOT_TOKEN в .env на реальный токен
    pause
    exit /b 1
)

if "%SEMESTER_START%"=="" set SEMESTER_START=11.01.2026
if "%WEBHOOK_PORT%"=="" set WEBHOOK_PORT=:8080
if "%WEBHOOK_PATH%"=="" set WEBHOOK_PATH=/webhook
if "%SCHEDULE_FILE%"=="" set SCHEDULE_FILE=schedule.json
if "%USERS_FILE%"=="" set USERS_FILE=users.json
if "%MESSAGES_FILE%"=="" set MESSAGES_FILE=messages.json
if "%ADMINS_FILE%"=="" set ADMINS_FILE=admins.json
if "%TOOLS_DIR%"=="" set TOOLS_DIR=tools

:: Generate config.json
echo ⚙️  Генерация config.json...
powershell -Command ^
    "$cfg = @{ " ^
    "  api_token = '%BOT_TOKEN%'; " ^
    "  semester_start_date = '%SEMESTER_START%'; " ^
    "  files = @{ " ^
    "    schedule = '%SCHEDULE_FILE%'; " ^
    "    users = '%USERS_FILE%'; " ^
    "    messages = '%MESSAGES_FILE%'; " ^
    "    admins = '%ADMINS_FILE%' " ^
    "  }; " ^
    "  tools_dir = '%TOOLS_DIR%'; " ^
    "  webhook_url = ''; " ^
    "  webhook_port = '%WEBHOOK_PORT%'; " ^
    "  webhook_path = '%WEBHOOK_PATH%' " ^
    "}; " ^
    "$cfg | ConvertTo-Json -Depth 10 | Set-Content config.json"

echo ✅ config.json создан
echo.
echo ========================================
echo  ✅ Готово! Запусти start.bat
echo ========================================
pause
