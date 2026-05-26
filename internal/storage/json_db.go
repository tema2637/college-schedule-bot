package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"college-schedule-bot/internal/config"
)

// User представляет данные пользователя в системе
type User struct {
	// FirstName - имя пользователя
	FirstName string `json:"first_name"`
	
	// LastName - фамилия пользователя
	LastName string `json:"last_name"`
	
	// GroupName - название группы, которую выбрал пользователь
	GroupName string `json:"group_name"`
	
	// RegistrationDate - дата регистрации в формате RFC3339
	RegistrationDate string `json:"registration_date"`
}

// ScheduleLesson представляет один урок в расписании
type ScheduleLesson struct {
	// LessonTitle - название предмета
	LessonTitle string `json:"LessonTitle"`
	
	// TeacherName - имя преподавателя
	TeacherName string `json:"TeacherName"`
	
	// Cabinet - номер кабинета
	Cabinet string `json:"Cabinet"`
	
	// Weeks - массив номеров недель, когда проводится занятие
	Weeks []int `json:"Weeks"`
	
	// Group - информация о группе
	Group struct {
		Name string `json:"Name"`
	} `json:"Group"`
	
	// TimeSlot - информация о временном слоте
	TimeSlot struct {
		NumberSlot int    `json:"NumberSlot"`
		DayOfWeek  int    `json:"DayOfWeek"`
		StartTime  string `json:"StartTime"`
		EndTime    string `json:"EndTime"`
	} `json:"TimeSlot"`
}

// Messages содержит шаблоны сообщений на русском языке
type Messages struct {
	// Start - приветственное сообщение
	Start string `json:"start"`
	
	// SelectGroup - сообщение для выбора группы
	SelectGroup string `json:"select_group"`
	
	// ScheduleFormat - шаблон форматирования расписания
	ScheduleFormat string `json:"schedule_format"`
	
	// Error - сообщение об ошибке
	Error string `json:"error"`
}

// Manager управляет всеми JSON-файлами данных
type Manager struct {
	// filePaths - пути к файлам из конфигурации
	filePaths config.FilePaths
	
	// users - кэш данных пользователей
	users map[int64]User
	
	// admins - кэш ID администраторов
	admins []int64
	
	// schedule - кэш расписания
	schedule []ScheduleLesson
	
	// messages - кэш шаблонов сообщений
	messages Messages
	
	// mutex для потокобезопасности
	mu sync.RWMutex
}

// NewManager создает новый менеджер хранилища
func NewManager(filePaths config.FilePaths) (*Manager, error) {
	manager := &Manager{
		filePaths: filePaths,
		users:     make(map[int64]User),
	}

	// Загрузка всех данных при инициализации
	if err := manager.LoadAll(); err != nil {
		return nil, fmt.Errorf("ошибка загрузки данных: %w", err)
	}

	return manager, nil
}

// LoadAll загружает все данные из JSON-файлов
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Загрузка пользователей
	if err := m.loadUsers(); err != nil {
		return fmt.Errorf("ошибка загрузки пользователей: %w", err)
	}

	// Загрузка администраторов
	if err := m.loadAdmins(); err != nil {
		return fmt.Errorf("ошибка загрузки администраторов: %w", err)
	}

	// Загрузка расписания
	if err := m.loadSchedule(); err != nil {
		return fmt.Errorf("ошибка загрузки расписания: %w", err)
	}

	// Загрузка сообщений
	if err := m.loadMessages(); err != nil {
		return fmt.Errorf("ошибка загрузки сообщений: %w", err)
	}

	return nil
}

// loadUsers загружает данные пользователей из JSON
func (m *Manager) loadUsers() error {
	data, err := os.ReadFile(m.filePaths.Users)
	if err != nil {
		if os.IsNotExist(err) {
			// Если файла нет, создаем пустой
			m.users = make(map[int64]User)
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &m.users)
}

// loadSchedule загружает расписание из JSON
func (m *Manager) loadSchedule() error {
	data, err := os.ReadFile(m.filePaths.Schedule)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.schedule)
}

// loadAdmins загружает список администраторов из JSON
func (m *Manager) loadAdmins() error {
	if m.filePaths.Admins == "" {
		m.admins = []int64{}
		return nil
	}
	data, err := os.ReadFile(m.filePaths.Admins)
	if err != nil {
		if os.IsNotExist(err) {
			m.admins = []int64{}
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.admins)
}

// saveAdmins сохраняет список администраторов в JSON
func (m *Manager) saveAdmins() error {
	if m.filePaths.Admins == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.admins, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePaths.Admins, data, 0644)
}

// IsAdmin проверяет, является ли пользователь администратором
func (m *Manager) IsAdmin(userID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.admins {
		if id == userID {
			return true
		}
	}
	return false
}

// AddAdmin добавляет пользователя в администраторы
func (m *Manager) AddAdmin(userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.admins {
		if id == userID {
			return nil // уже админ
		}
	}
	m.admins = append(m.admins, userID)
	return m.saveAdmins()
}

// RemoveAdmin удаляет пользователя из администраторов
func (m *Manager) RemoveAdmin(userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, id := range m.admins {
		if id == userID {
			m.admins = append(m.admins[:i], m.admins[i+1:]...)
			return m.saveAdmins()
		}
	}
	return nil
}

// GetAdmins возвращает копию списка администраторов
func (m *Manager) GetAdmins() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copy := make([]int64, len(m.admins))
	copy = append(copy, m.admins...)
	return copy
}

// loadMessages загружает шаблоны сообщений из JSON
func (m *Manager) loadMessages() error {
	data, err := os.ReadFile(m.filePaths.Messages)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.messages)
}

// SaveAll сохраняет все измененные данные в JSON-файлы
func (m *Manager) SaveAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Сохранение пользователей
	if err := m.saveUsers(); err != nil {
		return fmt.Errorf("ошибка сохранения пользователей: %w", err)
	}

	return nil
}

// saveUsers сохраняет данные пользователей в JSON с форматированием
func (m *Manager) saveUsers() error {
	data, err := json.MarshalIndent(m.users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePaths.Users, data, 0644)
}

// GetUser возвращает данные пользователя по ID
func (m *Manager) GetUser(userID int64) (User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	user, exists := m.users[userID]
	return user, exists
}

// SetUser сохраняет данные пользователя
func (m *Manager) SetUser(userID int64, user User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.users[userID] = user
	return m.saveUsers()
}

// GetAllUsers возвращает всех пользователей для broadcast
func (m *Manager) GetAllUsers() map[int64]User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Создаем копию для предотвращения гонок
	usersCopy := make(map[int64]User)
	for id, user := range m.users {
		usersCopy[id] = user
	}
	
	return usersCopy
}

// GetSchedule возвращает все расписание
func (m *Manager) GetSchedule() []ScheduleLesson {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Создаем копию для предотвращения гонок
	scheduleCopy := make([]ScheduleLesson, len(m.schedule))
	copy(scheduleCopy, m.schedule)
	
	return scheduleCopy
}

// GetMessages возвращает шаблоны сообщений
func (m *Manager) GetMessages() Messages {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.messages
}
