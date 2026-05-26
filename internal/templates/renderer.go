package templates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
)

// Renderer управляет шаблонами сообщений на русском языке
type Renderer struct {
	// templates - кэш скомпилированных шаблонов
	templates map[string]*template.Template
}

// NewRenderer создает новый рендерер шаблонов
// Загружает шаблоны из JSON-файла
func NewRenderer(messagesPath string) *Renderer {
	renderer := &Renderer{
		templates: make(map[string]*template.Template),
	}
	
	// Загрузка и парсинг шаблонов при инициализации
	if err := renderer.loadFromJSON(messagesPath); err != nil {
		// При ошибке загрузки создаем пустые шаблоны
		fmt.Printf("Ошибка загрузки шаблонов: %v\n", err)
	}
	
	return renderer
}

// loadFromJSON загружает и парсит шаблоны из JSON-файла
func (r *Renderer) loadFromJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла сообщений: %w", err)
	}
	
	// Временная структура для загрузки JSON
	var rawMessages map[string]string
	if err := json.Unmarshal(data, &rawMessages); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %w", err)
	}
	
	// Парсинг каждого шаблона
	for key, message := range rawMessages {
		tmpl, err := template.New(key).Parse(message)
		if err != nil {
			fmt.Printf("Ошибка парсинга шаблона '%s': %v\n", key, err)
			continue
		}
		r.templates[key] = tmpl
	}
	
	return nil
}

// Render выполняет рендеринг шаблона с заданными данными
// name - имя шаблона (start, select_group, schedule_format, error)
// data - данные для подстановки (Group, LessonTitle, Cabinet, Teacher, TimeStart, TimeEnd, WeekNum)
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

// Reload перезагружает шаблоны из файла
func (r *Renderer) Reload(path string) error {
	r.templates = make(map[string]*template.Template)
	return r.loadFromJSON(path)
}
