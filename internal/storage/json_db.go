package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// User представляет данные пользователя в системе
type User struct {
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	GroupName        string `json:"group_name"`
	RegistrationDate string `json:"registration_date"`
	DailyUpdate      bool   `json:"daily_update,omitempty"`
}

// ScheduleLesson представляет один урок в расписании
type ScheduleLesson struct {
	LessonTitle string `json:"LessonTitle"`
	TeacherName string `json:"TeacherName"`
	Cabinet     string `json:"Cabinet"`
	Weeks       []int  `json:"Weeks"`
	Group       struct {
		Name string `json:"Name"`
	} `json:"Group"`
	TimeSlot struct {
		NumberSlot int    `json:"NumberSlot"`
		DayOfWeek  int    `json:"DayOfWeek"`
		StartTime  string `json:"StartTime"`
		EndTime    string `json:"EndTime"`
	} `json:"TimeSlot"`
}

// AdminData хранит ID админов и суперадминов
type AdminData struct {
	Admins      []int64 `json:"admins"`
	SuperAdmins []int64 `json:"super_admins"`
}

// Manager управляет данными (users.json, schedule.json, admins.json)
type Manager struct {
	schedulePath string
	usersPath    string
	adminsPath   string
	users        map[int64]User
	schedule     []ScheduleLesson
	admins       AdminData
	myIDEnabled  bool
	mu           sync.RWMutex
}

// NewManager создаёт менеджер хранилища
func NewManager(schedulePath, usersPath string) (*Manager, error) {
	m := &Manager{
		schedulePath: schedulePath,
		usersPath:    usersPath,
		adminsPath:   "admins.json",
		users:        make(map[int64]User),
	}
	if err := m.LoadAll(); err != nil {
		return nil, fmt.Errorf("load error: %w", err)
	}
	return m, nil
}

func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadUsers(); err != nil {
		return err
	}
	if err := m.loadSchedule(); err != nil {
		return err
	}
	return m.loadAdmins()
}

// --- users ---

func (m *Manager) loadUsers() error {
	data, err := os.ReadFile(m.usersPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.users = make(map[int64]User)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.users)
}

func (m *Manager) loadSchedule() error {
	data, err := os.ReadFile(m.schedulePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.schedule)
}

func (m *Manager) loadAdmins() error {
	data, err := os.ReadFile(m.adminsPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.admins = AdminData{}
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.admins)
}

func (m *Manager) saveAdmins() error {
	data, err := json.MarshalIndent(m.admins, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.adminsPath, data, 0644)
}

func (m *Manager) SaveAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if err := m.saveUsers(); err != nil {
		return err
	}
	return m.saveAdmins()
}

func (m *Manager) saveUsers() error {
	data, err := json.MarshalIndent(m.users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.usersPath, data, 0644)
}

func (m *Manager) GetUser(userID int64) (User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[userID]
	return u, ok
}

func (m *Manager) SetUser(userID int64, user User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[userID] = user
	return m.saveUsers()
}

func (m *Manager) GetAllUsers() map[int64]User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	usersCopy := make(map[int64]User, len(m.users))
	for id, u := range m.users {
		usersCopy[id] = u
	}
	return usersCopy
}

func (m *Manager) GetSchedule() []ScheduleLesson {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sched := make([]ScheduleLesson, len(m.schedule))
	copy(sched, m.schedule)
	return sched
}

// --- daily update ---

func (m *Manager) SetDailyUpdate(userID int64, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	u.DailyUpdate = enabled
	m.users[userID] = u
	return m.saveUsers()
}

// --- admin / superadmin ---

func (m *Manager) IsAdmin(userID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.admins.Admins {
		if id == userID {
			return true
		}
	}
	for _, id := range m.admins.SuperAdmins {
		if id == userID {
			return true
		}
	}
	return false
}

func (m *Manager) IsSuperAdmin(userID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.admins.SuperAdmins {
		if id == userID {
			return true
		}
	}
	return false
}

func (m *Manager) GetAdmins() AdminData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.admins
}

func (m *Manager) AddAdmin(userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.admins.Admins {
		if id == userID {
			return nil
		}
	}
	m.admins.Admins = append(m.admins.Admins, userID)
	return m.saveAdmins()
}

func (m *Manager) AddSuperAdmin(userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.admins.SuperAdmins {
		if id == userID {
			return nil
		}
	}
	m.admins.SuperAdmins = append(m.admins.SuperAdmins, userID)
	return m.saveAdmins()
}

func (m *Manager) RemoveAdmin(userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var newAdmins []int64
	for _, id := range m.admins.Admins {
		if id != userID {
			newAdmins = append(newAdmins, id)
		}
	}
	m.admins.Admins = newAdmins
	return m.saveAdmins()
}

// --- myID ---

func (m *Manager) IsMyIDEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.myIDEnabled
}

func (m *Manager) ToggleMyID() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.myIDEnabled = !m.myIDEnabled
	return m.myIDEnabled, nil
}

// unused but kept for compatibility
func (m *Manager) GetMessages() map[string]string {
	return nil
}
