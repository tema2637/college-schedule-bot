package scheduler

import (
	"fmt"
	"strings"
	"time"

	"college-schedule-bot/internal/storage"
)

var msk = time.FixedZone("MSK", 3*60*60)

type Engine struct {
	semesterStartDate string
	parsedStartDate   time.Time
}

func NewEngine(startDate string) *Engine {
	e := &Engine{semesterStartDate: startDate}
	if err := e.parseStartDate(); err != nil {
		panic(fmt.Sprintf("Неверный формат даты начала семестра: %v", err))
	}
	return e
}

func (e *Engine) parseStartDate() error {
	parsedDate, err := time.Parse("02.01.2006", e.semesterStartDate)
	if err != nil {
		return err
	}
	e.parsedStartDate = parsedDate
	return nil
}

// NowMSK возвращает текущее время по Москве
func (e *Engine) NowMSK() time.Time {
	return time.Now().UTC().Add(3 * time.Hour)
}

// GetCurrentWeek вычисляет текущий учебный номер недели (отсчёт с понедельника 12.01.2026)
func (e *Engine) GetCurrentWeek() int {
	now := e.NowMSK()
	var firstMonday time.Time
	if e.parsedStartDate.Weekday() == time.Sunday {
		firstMonday = e.parsedStartDate.AddDate(0, 0, 1)
	} else {
		daysToMonday := int(time.Monday - e.parsedStartDate.Weekday())
		if daysToMonday > 0 {
			daysToMonday -= 7
		}
		firstMonday = e.parsedStartDate.AddDate(0, 0, daysToMonday)
	}

	daysDiff := int(now.Sub(firstMonday).Hours() / 24)
	if daysDiff < 0 {
		return 1
	}
	return daysDiff/7 + 1
}

// GetCurrentDayOfWeek возвращает текущий день недели по МСК (1=пн, 7=вс)
func (e *Engine) GetCurrentDayOfWeek() int {
	now := e.NowMSK()
	dayOfWeek := int(now.Weekday())
	if dayOfWeek == 0 {
		return 7
	}
	return dayOfWeek
}

// GetDayName возвращает русское название дня недели
func GetDayName(dayOfWeek int) string {
	names := []string{"", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота", "Воскресенье"}
	if dayOfWeek < 1 || dayOfWeek > 7 {
		return ""
	}
	return names[dayOfWeek]
}

// GetShortDayName возвращает короткое русское название дня недели
func GetShortDayName(dayOfWeek int) string {
	names := []string{"", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
	if dayOfWeek < 1 || dayOfWeek > 7 {
		return ""
	}
	return names[dayOfWeek]
}

// Filter фильтрует занятия по группе, неделе и дню недели
func normalizeName(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), " ", "")
}

func (e *Engine) Filter(schedule []storage.ScheduleLesson, groupName string, weekNum int, dayOfWeek int) []storage.ScheduleLesson {
	var filtered []storage.ScheduleLesson
	normalizedGroup := normalizeName(groupName)
	for _, lesson := range schedule {
		if normalizeName(lesson.Group.Name) != normalizedGroup || lesson.TimeSlot.DayOfWeek != dayOfWeek {
			continue
		}
		for _, week := range lesson.Weeks {
			if week == weekNum {
				filtered = append(filtered, lesson)
				break
			}
		}
	}
	return filtered
}
