package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config содержит все настройки бота из JSON
// Загружается из файла config.json
type Config struct {
	// APIToken - токен для доступа к API бота
	APIToken string `json:"api_token"`
	
	// SemesterStartDate - дата начала семестра в формате DD.MM.YYYY
	SemesterStartDate string `json:"semester_start_date"`
	
	// Files - пути к файлам данных
	Files FilePaths `json:"files"`

	// ToolsDir - директория со вспомогательными Python-скриптами
	ToolsDir string `json:"tools_dir"`

	// WebhookURL - URL для приёма webhook от Max API (если пусто — используется long polling)
	WebhookURL string `json:"webhook_url,omitempty"`

	// WebhookPort - порт для webhook-сервера (по умолчанию 8080)
	WebhookPort string `json:"webhook_port,omitempty"`

	// WebhookPath - путь для webhook endpoint (по умолчанию /webhook)
	WebhookPath string `json:"webhook_path,omitempty"`

	// WebhookFilter - фильтр событий для webhook (опционально)
	WebhookFilter string `json:"webhook_filter,omitempty"`
}

// FilePaths содержит пути к JSON-файлам данных
type FilePaths struct {
	// Schedule - путь к файлу расписания
	Schedule string `json:"schedule"`
	
	// Users - путь к файлу пользователей
	Users string `json:"users"`
	
	// Admins - путь к файлу администраторов
	Admins string `json:"admins"`
	
	// Messages - путь к файлу шаблонов сообщений
	Messages string `json:"messages"`
}

// Load загружает конфигурацию из указанного JSON-файла
// Возвращает ошибку если файл не найден или имеет неверный формат
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации: %w", err)
	}
	
	return &config, nil
}


