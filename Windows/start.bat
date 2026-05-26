@echo off
chcp 65001 >nul
title College Schedule Bot

echo ========================================
echo  College Schedule Bot - Auto Start
echo  ^(with tunnel replacement^)
echo ========================================
echo.

cd /d "%~dp0"

:: 1. Kill old processes
echo [1/5] Останавливаю старые процессы...
taskkill /f /im cloudflared.exe 2>nul
taskkill /f /im college-schedule-bot.exe 2>nul
timeout /t 2 /nobreak >nul

:: 2. Start cloudflared tunnel
echo [2/5] Запускаю новый туннель Cloudflare...
set TUNNEL_LOG=%temp%\cloudflared_%random%.log
start /b "" cloudflared tunnel --url http://localhost:8080 > "%TUNNEL_LOG%" 2>&1

:: 3. Wait for tunnel URL
echo [3/5] Ожидаю URL туннеля...
set TUNNEL_URL=
set ATTEMPTS=0
:wait_loop
set /a ATTEMPTS+=1
if %ATTEMPTS% gtr 30 (
    echo [ERROR] Не удалось получить URL туннеля за 30 секунд
    pause
    exit /b 1
)
timeout /t 1 /nobreak >nul
for /f "tokens=*" %%a in ('findstr /r "https://[a-z0-9-]*\.trycloudflare\.com" "%TUNNEL_LOG%" 2^>nul') do set "TUNNEL_URL=%%a"
if "%TUNNEL_URL%"=="" goto wait_loop

echo   URL: %TUNNEL_URL%

:: 4. Update config.json
echo [4/5] Обновляю config.json...
set WEBHOOK_URL=%TUNNEL_URL%/webhook
powershell -Command ^
    "$cfg = Get-Content config.json | ConvertFrom-Json; " ^
    "$cfg.webhook_url = '%WEBHOOK_URL%'; " ^
    "$cfg | ConvertTo-Json -Depth 10 | Set-Content config.json"
echo   Webhook: %WEBHOOK_URL%

:: 5. Start bot
echo [5/5] Запускаю бота...
start /b "" college-schedule-bot.exe

echo.
echo ========================================
echo  ✅ Запущено:
echo     Bot: college-schedule-bot.exe
echo     Tunnel: %TUNNEL_URL%
echo ========================================
echo.
echo Нажми любую клавишу для выхода (бот останется в фоне).
pause >nul
