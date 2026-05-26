package bot

import (
	"fmt"
	"sort"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"college-schedule-bot/internal/storage"
)

var dayNavNames = []string{"", "ПН", "ВТ", "СР", "ЧТ", "ПТ", "СБ"}

// BuildGroupKeyboard — inline выбор группы, 10 штук на страницу, по 2 в ряд
func BuildGroupKeyboard(groups []string, page int) *maxbot.Keyboard {
	const perPage = 10
	totalPages := (len(groups) + perPage - 1) / perPage
	if page < 0 { page = 0 }
	if page >= totalPages && totalPages > 0 { page = totalPages - 1 }

	start := page * perPage
	end := start + perPage
	if end > len(groups) { end = len(groups) }

	kb := maxbot.Keyboard{}
	for i := start; i < end; i += 2 {
		row := kb.AddRow()
		row.AddCallback(groups[i], schemes.DEFAULT, "group_"+groups[i])
		if i+1 < end {
			row.AddCallback(groups[i+1], schemes.DEFAULT, "group_"+groups[i+1])
		}
	}

	// Навигация по страницам
	if totalPages > 1 {
		rowNav := kb.AddRow()
		if page > 0 {
			rowNav.AddCallback("◀️ Назад", schemes.DEFAULT, fmt.Sprintf("page_%d", page-1))
		}
		rowNav.AddCallback(fmt.Sprintf("%d/%d", page+1, totalPages), schemes.DEFAULT, "noop")
		if page < totalPages-1 {
			rowNav.AddCallback("Вперед ▶️", schemes.DEFAULT, fmt.Sprintf("page_%d", page+1))
		}
	}

	return &kb
}

// BuildDayNavigationKeyboard — кнопки ⬅️ ВТ | 🏠 | ЧТ ➡️
func BuildDayNavigationKeyboard(currentDay int, currentWeek int) *maxbot.Keyboard {
	kb := maxbot.Keyboard{}
	row1 := kb.AddRow()

	prevDay := currentDay - 1
	if prevDay < 1 { prevDay = 6 } // Переход СБ -> ПТ и т.д., 1=ПН, 6=СБ
	row1.AddCallback(fmt.Sprintf("⬅️ %s", dayNavNames[prevDay]), schemes.DEFAULT, fmt.Sprintf("daynav_%d_%d", prevDay, currentWeek))

	row1.AddCallback("🏠 Сегодня", schemes.DEFAULT, "daynav_today")

	nextDay := currentDay + 1
	if nextDay > 6 { nextDay = 1 } // СБ -> ПН
	row1.AddCallback(fmt.Sprintf("%s ➡️", dayNavNames[nextDay]), schemes.DEFAULT, fmt.Sprintf("daynav_%d_%d", nextDay, currentWeek))

	return &kb
}

func BuildWelcomeKeyboard() *maxbot.Keyboard {
	kb := maxbot.Keyboard{}
	row := kb.AddRow()
	row.AddCallback("⚙️ Выбрать группу", schemes.POSITIVE, "start_select_group")
	return &kb
}

// BuildWeekNavigationKeyboard — кнопки переключения недель
func BuildWeekNavigationKeyboard(currentWeek int, realCurrentWeek int) *maxbot.Keyboard {
	kb := maxbot.Keyboard{}
	row := kb.AddRow()
	if currentWeek == realCurrentWeek {
		row.AddCallback("➡️ Следующая неделя", schemes.POSITIVE, "week_next")
	} else {
		row.AddCallback("⬅️ Текущая неделя", schemes.POSITIVE, "week_current")
	}
	return &kb
}

// ExtractGroupsFromSchedule извлекает уникальные группы и сортирует их
func ExtractGroupsFromSchedule(schedule []storage.ScheduleLesson) []string {
	groupMap := make(map[string]bool)
	for _, lesson := range schedule {
		if lesson.Group.Name != "" {
			groupMap[lesson.Group.Name] = true
		}
	}
	var groups []string
	for group := range groupMap {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
