package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// UpdatePayload представляет входящий webhook от Max API
type UpdatePayload struct {
	UpdateType string          `json:"update_type"`
	Timestamp  int             `json:"timestamp"`
	Data       json.RawMessage `json:"-"`
}

// Server запускает HTTP-сервер для приёма webhook
type Server struct {
	handler func(schemes.UpdateInterface)
	addr    string
	server  *http.Server
}

// NewServer создаёт новый webhook-сервер
func NewServer(addr string, handler func(schemes.UpdateInterface)) *Server {
	if addr == "" {
		addr = ":8080"
	}
	s := &Server{
		handler: handler,
		addr:    addr,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", s.handleWebhook)
	mux.HandleFunc("/health", s.handleHealth)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  10 * time.Second,
		IdleTimeout:   60 * time.Second,
	}
	return s
}

// Start запускает HTTP-сервер
func (s *Server) Start(ctx context.Context) error {
	log.Printf("[WEBHOOK] Сервер запущен на %s", s.addr)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webhook server error: %w", err)
	}
	return nil
}

// handleWebhook обрабатывает входящие POST-запросы от Max API
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WEBHOOK] Ошибка чтения тела: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим update_type для определения конкретного типа
	var base struct {
		UpdateType schemes.UpdateType `json:"update_type"`
	}
	if err := json.Unmarshal(body, &base); err != nil {
		log.Printf("[WEBHOOK] Ошибка парсинга update_type: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	update, err := parseUpdate(base.UpdateType, body)
	if err != nil {
		log.Printf("[WEBHOOK] Ошибка парсинга обновления %s: %v", base.UpdateType, err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if update != nil {
		go s.handler(update)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleHealth — healthcheck endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// parseUpdate парсит конкретный тип обновления на основе update_type
func parseUpdate(updateType schemes.UpdateType, data []byte) (schemes.UpdateInterface, error) {
	switch updateType {
	case schemes.TypeMessageCreated:
		var u schemes.MessageCreatedUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeMessageCallback:
		var u schemes.MessageCallbackUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeMessageEdited:
		var u schemes.MessageEditedUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeMessageRemoved:
		var u schemes.MessageRemovedUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeBotStarted:
		var u schemes.BotStartedUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeBotAdded:
		var u schemes.BotAddedToChatUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeBotRemoved:
		var u schemes.BotRemovedFromChatUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeUserAdded:
		var u schemes.UserAddedToChatUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeUserRemoved:
		var u schemes.UserRemovedFromChatUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	case schemes.TypeChatTitleChanged:
		var u schemes.ChatTitleChangedUpdate
		if err := json.Unmarshal(data, &u); err != nil {
			return nil, err
		}
		return &u, nil
	default:
		log.Printf("[WEBHOOK] Неизвестный тип обновления: %s", updateType)
		return nil, nil
	}
}

// SubscriptionRequest запрос на подписку через webhook
type SubscriptionRequest struct {
	URL    string `json:"url"`
	Filter string `json:"filter,omitempty"`
}

// SubscriptionResponse ответ на создание подписки
type SubscriptionResponse struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

// Subscribe регистрирует webhook в Max API
func Subscribe(api *maxbot.Api, webhookURL string, filter string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Параметры подписки
	var updateTypes []string
	if filter != "" {
		updateTypes = strings.Split(filter, ",")
	}

	// Регистрируем webhook через клиент API
	_, err := api.Subscriptions.Subscribe(ctx, webhookURL, updateTypes, "")
	if err != nil {
		return fmt.Errorf("ошибка регистрации webhook: %w", err)
	}

	log.Printf("[WEBHOOK] Подписка на %s зарегистрирована", webhookURL)
	return nil
}
