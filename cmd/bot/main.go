package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"college-schedule-bot/internal/bot"
	"college-schedule-bot/internal/config"
	"college-schedule-bot/internal/scheduler"
	"college-schedule-bot/internal/storage"
	"college-schedule-bot/internal/templates"
	"college-schedule-bot/internal/tools"
	"college-schedule-bot/internal/webhook"
)

// Основная точка входа в приложение бота
func main() {
	log.Println("Zapusk kolledzh-bota...")

	// Переходим в директорию с бинарником (чтобы файлы искались относительно него)
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		if err := os.Chdir(exeDir); err != nil {
			log.Printf("[MAIN] не удалось сменить директорию на %s: %v", exeDir, err)
		} else {
			log.Printf("[MAIN] рабочая директория: %s", exeDir)
		}
	}

	// Загрузка конфигурации
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("Oshibka zagruzki konfiguryatsii: %v", err)
	}

	// Инициализация хранилища данных
	storageManager, err := storage.NewManager(cfg.Files)
	if err != nil {
		log.Fatalf("Oshibka initsializatsii khranilishcha: %v", err)
	}

	// Инициализация движка планировщика
	scheduleEngine := scheduler.NewEngine(cfg.SemesterStartDate)

	// Инициализация шаблонизатора
	templateRenderer := templates.NewRenderer(cfg.Files.Messages)

	// Инициализация Python-обёртки для конвертеров
	pyRunner := tools.NewPythonRunner(cfg.ToolsDir, ".")

	// Инициализация клиента Max Bot API
	api, err := maxbot.New(cfg.APIToken)
	if err != nil {
		log.Fatalf("Ошибка создания клиента Max Bot API: %v", err)
	}

	// Создание обработчика бота
	botHandler := bot.NewHandler(api, storageManager, scheduleEngine, templateRenderer, cfg, pyRunner)

	// Настройка graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Запуск бота в отдельной горутине
	go func() {
		log.Println("Бот успешно запущен и готов к работе")
		botHandler.Start(ctx)
	}()

	// --- WEBHOOK MODE ---
	if cfg.WebhookURL != "" {
		log.Printf("[MAIN] Режим webhook: %s", cfg.WebhookURL)

		// Определяем адрес для监听
		addr := cfg.WebhookPort
		if addr == "" {
			addr = ":8080"
		}

		// Создаём webhook-сервер
		webhookServer := webhook.NewServer(addr, botHandler.HandleUpdate)

		// Регистрируем подписку в Max API
		if err := webhook.Subscribe(api, cfg.WebhookURL, cfg.WebhookFilter); err != nil {
			log.Printf("[MAIN] Предупреждение: не удалось зарегистрировать webhook: %v", err)
		}

		// Запускаем сервер
		if err := webhookServer.Start(ctx); err != nil {
			log.Fatalf("[MAIN] Ошибка webhook-сервера: %v", err)
		}
	} else {
		log.Println("[MAIN] Режим long polling (устаревает 11.05.2026)")
	}

	// Ожидание сигнала завершения
	<-ctx.Done()

	log.Println("Получен сигнал завершения, начинаем graceful shutdown...")

	// Создание контекста с таймаутом для завершения
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Остановка бота
	if err := botHandler.Stop(shutdownCtx); err != nil {
		log.Printf("Ошибка при остановке бота: %v", err)
	} else {
		log.Println("Бот успешно остановлен")
	}

	// Сохранение данных
	if err := storageManager.SaveAll(); err != nil {
		log.Printf("Ошибка сохранения данных: %v", err)
	}

	log.Println("Graceful shutdown завершён")
}
