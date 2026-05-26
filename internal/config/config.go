package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config содержит все настройки бота (большая часть захардкожена)
type Config struct {
	// APIToken — токен для доступа к API бота (из BOT_TOKEN env)
	APIToken string

	// AdminIDs — список ID администраторов
	AdminIDs []int64
}

// Load загружает конфигурацию: токен из env, остальное — константы
func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN не задан")
	}

	// Админы захардкожены
	adminStrs := strings.Split(os.Getenv("ADMIN_IDS"), ",")
	if len(adminStrs) == 0 || (len(adminStrs) == 1 && adminStrs[0] == "") {
		adminStrs = []string{"173390202", "8385088944"}
	}
	var adminIDs []int64
	for _, s := range adminStrs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("неверный admin_id '%s': %w", s, err)
		}
		adminIDs = append(adminIDs, id)
	}

	return &Config{
		APIToken: token,
		AdminIDs: adminIDs,
	}, nil
}

// IsAdmin проверяет, является ли пользователь администратором
func (c *Config) IsAdmin(userID int64) bool {
	for _, adminID := range c.AdminIDs {
		if adminID == userID {
			return true
		}
	}
	return false
}

// Хардкоженные константы
const (
	SemesterStartDate = "11.01.2026"

	SchedulePath = "schedule.json"
	UsersPath    = "users.json"
	ChangesPath  = "changes.json"

	ToolsDir = "tools"

	WebhookPort = ":8080"
	WebhookPath = "/webhook"
)
