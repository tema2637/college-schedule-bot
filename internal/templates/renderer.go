package templates

import (
	"bytes"
	"fmt"
	"text/template"
)

// Renderer управляет шаблонами сообщений (захардкожено)
type Renderer struct {
	templates map[string]*template.Template
}

// NewRenderer создаёт рендерер с захардкоженными шаблонами
func NewRenderer() *Renderer {
	r := &Renderer{
		templates: make(map[string]*template.Template),
	}
	r.loadDefaults()
	return r
}

func (r *Renderer) loadDefaults() {
	messages := map[string]string{
		"start":              "Привет! Вы уже зарегистрированы в группе {{.Group}}.\n\nИспользуйте кнопки ниже.",
		"welcome_message":    "👋 Здравствуй, {{.Name}}!\n\nЭто бот твоего расписания. Здесь ты можешь оперативно получать информацию о парах.\n\n👇 Для начала нажми на кнопку ниже, чтобы выбрать свою группу:",
		"welcome_btn":        "⚙️ Выбрать группу",
		"select_group":       "📅 Выберите вашу учебную группу:",
		"group_selected":     "✅ Группа выбрана: {{.Group}}",
		"schedule_header":    "🏫 Группа: {{.GroupName}}\n📅 {{.DayName}} | Неделя: {{.WeekNum}}",
		"schedule_empty":     "Занятий нет 🎉 \nОтдыхай!",
		"schedule_format":    "Пара {{.Num}} {{.TimeStart}} — {{.TimeEnd}}\n📖 {{.LessonTitle}}\n📍 {{.Cabinet}}\n👤 {{.Teacher}}",
		"error":              "Произошла ошибка 😔\nПопробуйте позже или обратитесь к администратору.",
		"current_week":       "Текущая учебная неделя: {{.WeekNum}}",
		"select_day":         "Выберите день недели:",
		"unknown_command":    "❓ Неизвестная команда.\n\n📖 Доступные команды:\n• /today — Пары на сегодня\n• /tomorrow — Пары на завтра\n• /week — Расписание на неделю\n• /nweek — Следующая неделя\n• /change_group — Сменить группу\n\nЭто сообщение исчезнет при вводе команды.",
		"no_rights":          "⛔ У вас нет прав для этой команды.",
		"broadcast_prompt":   "📢 Введите текст для рассылки.",
		"broadcast_preview":  "📢 Подтвердите рассылку:\n\n{{.Text}}",
		"broadcast_done":     "✅ Рассылка завершена!\nОтправлено: {{.Sent}}\nНе удалось: {{.Failed}}",
		"changes_prompt":     "📋 Отправьте текст изменений расписания в формате колледжа.\n\nЯ автоматически определю затронутые группы.",
	}

	for key, msg := range messages {
		tmpl, err := template.New(key).Parse(msg)
		if err != nil {
			fmt.Printf("Ошибка парсинга шаблона '%s': %v\n", key, err)
			continue
		}
		r.templates[key] = tmpl
	}
}

// Render выполняет рендеринг шаблона с заданными данными
func (r *Renderer) Render(name string, data interface{}) (string, error) {
	tmpl, exists := r.templates[name]
	if !exists {
		return "", fmt.Errorf("шаблон '%s' не найден", name)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("ошибка рендеринга шаблона '%s': %w", name, err)
	}
	return buf.String(), nil
}

// MustRender аналог Render, но паникует при ошибке
func (r *Renderer) MustRender(name string, data interface{}) string {
	result, err := r.Render(name, data)
	if err != nil {
		panic(fmt.Sprintf("Ошибка рендеринга шаблона '%s': %v", name, err))
	}
	return result
}
