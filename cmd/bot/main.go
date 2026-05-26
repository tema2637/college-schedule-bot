package main

import (
	"context"
	"log"
	"os"
	"os/signal"
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

func main() {
	log.Println("College Schedule Bot — optimized")

	// Конфиг из env + константы
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Хранилище (schedule.json, users.json)
	storageManager, err := storage.NewManager(
		config.SchedulePath,
		config.UsersPath,
	)
	if err != nil {
		log.Fatalf("Storage init error: %v", err)
	}

	// Планировщик
	scheduleEngine := scheduler.NewEngine(config.SemesterStartDate)

	// Рендерер (захардкожен, без файла)
	templateRenderer := templates.NewRenderer()

	// Python-обёртка (опционально, для конвертации xlsx)
	pyRunner := tools.NewPythonRunner(config.ToolsDir, ".")

	// Клиент Max Bot API
	api, err := maxbot.New(cfg.APIToken)
	if err != nil {
		log.Fatalf("Max API client error: %v", err)
	}

	// Обработчик бота
	botHandler := bot.NewHandler(api, storageManager, scheduleEngine, templateRenderer, cfg, pyRunner)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("Bot started and ready")
		botHandler.Start(ctx)
	}()

	// Webhook: URL из env (устанавливается entrypoint.sh) или long polling
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL != "" {
		log.Printf("[MAIN] Webhook mode: %s", webhookURL)

		webhookServer := webhook.NewServer(config.WebhookPort, botHandler.HandleUpdate)

		if err := webhook.Subscribe(api, webhookURL, ""); err != nil {
			log.Printf("[MAIN] Webhook registration warning: %v", err)
		}

		go func() {
			if err := webhookServer.Start(ctx); err != nil {
				log.Fatalf("[MAIN] Webhook server error: %v", err)
			}
		}()
	} else {
		log.Println("[MAIN] Long polling mode")
	}

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := botHandler.Stop(shutdownCtx); err != nil {
		log.Printf("Stop error: %v", err)
	}

	if err := storageManager.SaveAll(); err != nil {
		log.Printf("Save error: %v", err)
	}

	log.Println("Shutdown complete")
}
