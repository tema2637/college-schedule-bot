@echo off
chcp 65001 >nul
title College Bot — Установка зависимостей

echo ========================================
echo  College Schedule Bot — Установка
echo  Windows
echo ========================================
echo.

:: 1. Проверка/установка cloudflared
echo [1/3] Cloudflare Tunnel (cloudflared)...
where cloudflared >nul 2>&1
if %errorlevel% equ 0 (
    echo   ✅ Уже установлен
) else (
    echo   ⬇️ Скачиваю cloudflared...
    powershell -Command ^
        "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; " ^
        "Invoke-WebRequest -Uri 'https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe' -OutFile '%temp%\cloudflared.exe'" ^
    if exist "%temp%\cloudflared.exe" (
        copy /y "%temp%\cloudflared.exe" cloudflared.exe >nul
        echo   ✅ cloudflared.exe сохранён в текущую папку
    ) else (
        echo   ❌ Ошибка скачивания cloudflared
        echo     Скачай вручную: https://github.com/cloudflare/cloudflared/releases
        echo     Положи cloudflared.exe в эту папку
    )
)

:: 2. Проверка Python
echo [2/3] Python 3...
where python >nul 2>&1
if %errorlevel% equ 0 (
    python --version
    echo   ✅ Python найден
) else (
    echo   ❌ Python 3 не найден
    echo     Скачай: https://www.python.org/downloads/
    echo     При установке отметь "Add Python to PATH"
    echo.
    echo     После установки запусти install.bat снова
    pause
    exit /b 1
)

:: 3. Установка Python-библиотек
echo [3/3] Python-библиотеки (openpyxl)...
pip install openpyxl 2>&1 | findstr /i "already satisfied" >nul
if %errorlevel% equ 0 (
    echo   ✅ openpyxl уже установлен
) else (
    pip install openpyxl
    if %errorlevel% equ 0 (
        echo   ✅ openpyxl установлен
    ) else (
        echo   ⚠️ Не удалось установить openpyxl
        echo     Установи вручную: pip install openpyxl
    )
)

echo.
echo ========================================
echo  ✅ Готово! Все зависимости установлены.
echo.
echo  Запусти start.bat для старта бота.
echo ========================================
echo.
pause
