package changes

import (
	"fmt"
	"regexp"
	"strings"
)

// ParsedChange представляет одно изменение для одной группы
type ParsedChange struct {
	GroupNum    string
	Subject     string
	Teacher     string
	ChangeType  string // Добавление, Замена, Отмена
	LessonNum   string
	ReplaceInfo string // что отменили (для замены)
	Note        string // примечание (вм. N п, сам/р.)
}

// ParseResult результат парсинга всего сообщения
type ParseResult struct {
	Header  string
	Date    string
	Changes []ParsedChange
	Groups  []string // уникальные группы
}

// Parse распознаёт текст изменений расписания
func Parse(text string) (*ParseResult, error) {
	lines := strings.Split(text, "\n")
	result := &ParseResult{}

	var currentGroup string
	var current ParsedChange
	var pendingSubject string
	var pendingTeacher []string
	state := "idle" // idle, group, subject, teacher, type, lesson

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// Заголовок
		if strings.Contains(line, "ИЗМЕНЕНИЯ СТАБИЛЬНОГО РАСПИСАНИЯ") {
			result.Header = line
			re := regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})`)
			m := re.FindStringSubmatch(line)
			if len(m) > 1 {
				result.Date = m[1]
			}
			continue
		}

		// Новая группа: "1. ИП252" или "1. Р3252"
		if matched, _ := regexp.MatchString(`^\d+\.\s+[А-ЯA-Z]{1,3}\d{2,3}$`, line); matched {
			// Сохраняем предыдущее если есть
			if current.GroupNum != "" {
				result.Changes = append(result.Changes, current)
			}
			parts := strings.SplitN(line, ". ", 2)
			if len(parts) == 2 {
				currentGroup = strings.TrimSpace(parts[1])
				result.Groups = appendUnique(result.Groups, currentGroup)
			}
			current = ParsedChange{GroupNum: currentGroup}
			pendingSubject = ""
			pendingTeacher = nil
			state = "group"
			continue
		}

		// Тип изменения
		if line == "Добавление" || line == "Замена" || line == "Отмена" {
			if state == "teacher" || state == "subject" {
				if pendingSubject != "" {
					current.Subject = pendingSubject
				}
				if len(pendingTeacher) > 0 {
					current.Teacher = strings.Join(pendingTeacher, "/")
				}
			}
			current.ChangeType = line
			state = "type"
			continue
		}

		// "Отменили: ..."
		if strings.HasPrefix(line, "Отменили:") {
			current.ReplaceInfo = strings.TrimPrefix(line, "Отменили:")
			continue
		}

		// "Номер пары: 4 п"
		if strings.HasPrefix(line, "Номер пары:") {
			current.LessonNum = strings.TrimPrefix(line, "Номер пары:")
			current.LessonNum = strings.TrimSpace(strings.ReplaceAll(current.LessonNum, "п", ""))
			// Сохраняем текущее изменение
			if current.GroupNum != "" {
				result.Changes = append(result.Changes, current)
				// Сбрасываем для следующей пары в той же группе
				current = ParsedChange{GroupNum: currentGroup}
				pendingSubject = ""
				pendingTeacher = nil
			}
			state = "lesson"
			continue
		}

		// Примечание (вм. 4 п, сам/р.)
		if strings.Contains(line, "вм.") || strings.Contains(line, "сам/р.") {
			current.Note = line
			continue
		}

		// Если строка короткая и похожа на предмет
		if len(line) < 60 && !strings.Contains(line, "Отменили") && state != "teacher" {
			if pendingSubject == "" {
				pendingSubject = line
				state = "subject"
			} else if current.Teacher == "" && pendingSubject != "" {
				// Возможно это второй предмет или преподаватель короткий
				// Проверим: если есть заглавная и точка — это ФИО
				if isLikelyTeacher(line) {
					pendingTeacher = append(pendingTeacher, line)
					state = "teacher"
				} else {
					pendingSubject = line
				}
			}
			continue
		}

		// Преподаватель (обычно Фамилия И.О. или Фамилия/Фамилия)
		if isLikelyTeacher(line) {
			pendingTeacher = append(pendingTeacher, line)
			state = "teacher"
			continue
		}

		// Fallback — любая нераспознанная строка
		if current.Subject == "" && pendingSubject != "" {
			current.Subject = pendingSubject
		}

		_ = i
	}

	// Сохраняем последнее
	if current.GroupNum != "" && current.LessonNum != "" {
		result.Changes = append(result.Changes, current)
	}

	// Удаляем дубликаты групп
	result.Groups = uniqueStrings(result.Groups)

	if len(result.Changes) == 0 {
		return nil, fmt.Errorf("не удалось распознать изменения в тексте")
	}

	return result, nil
}

// FormatMessage форматирует изменения в красивое сообщение для рассылки
func (r *ParseResult) FormatMessage() string {
	var sb strings.Builder
	if r.Date != "" {
		sb.WriteString(fmt.Sprintf("🗓 Изменения расписания на %s\n\n", r.Date))
	} else {
		sb.WriteString("🗓 Изменения расписания\n\n")
	}

	lastGroup := ""
	for _, ch := range r.Changes {
		if ch.GroupNum != lastGroup {
			if lastGroup != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("👥 Группа %s\n", ch.GroupNum))
			lastGroup = ch.GroupNum
		}

		sb.WriteString(fmt.Sprintf("  📚 %s\n", ch.Subject))
		if ch.Teacher != "" {
			sb.WriteString(fmt.Sprintf("  👤 %s\n", ch.Teacher))
		}
		if ch.ReplaceInfo != "" {
			sb.WriteString(fmt.Sprintf("  ❌ Отменили: %s\n", strings.TrimSpace(ch.ReplaceInfo)))
		}
		typeEmoji := "➕"
		if ch.ChangeType == "Замена" {
			typeEmoji = "🔄"
		} else if ch.ChangeType == "Отмена" {
			typeEmoji = "❌"
		}
		sb.WriteString(fmt.Sprintf("  %s %s, %s пара", typeEmoji, ch.ChangeType, ch.LessonNum))
		if ch.Note != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", ch.Note))
		}
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// isLikelyTeacher определяет похоже ли на ФИО преподавателя
func isLikelyTeacher(s string) bool {
	// Шаблоны: "Иванов И.И.", "Иванов/Петров", "Сапрыкина А.А."
	re := regexp.MustCompile(`^[А-ЯA-Z][а-яa-z]+(\s+[А-ЯA-Z]\.\s*[А-ЯA-Z]\.|/[А-ЯA-Z][а-яa-z]+\s+[А-ЯA-Z]\.[А-ЯA-Z]\.)$`)
	return re.MatchString(s) || regexp.MustCompile(`^[А-ЯA-Z][а-яa-z]+\s*/\s*[А-ЯA-Z][а-яa-z]+$`).MatchString(s)
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
