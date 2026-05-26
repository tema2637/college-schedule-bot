package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"college-schedule-bot/internal/config"
	"college-schedule-bot/internal/scheduler"
	"college-schedule-bot/internal/storage"
	"college-schedule-bot/internal/templates"
	"college-schedule-bot/internal/tools"
)

type Handler struct {
	api            *maxbot.Api
	storage        *storage.Manager
	engine         *scheduler.Engine
	renderer       *templates.Renderer
	config         *config.Config
	pyRunner       *tools.PythonRunner
	state          map[int64]string
	broadcast      map[int64]string
	lastMsgID      map[int64]string
	lastDayNav     map[int64]int
	lastGroupPage  map[int64]int
	lastActionTime map[int64]time.Time
	lastWeekNav    map[int64]int
	lastCorrMsgID  map[int64]string
}

func NewHandler(
	api *maxbot.Api,
	storage *storage.Manager,
	engine *scheduler.Engine,
	renderer *templates.Renderer,
	config *config.Config,
	pyRunner *tools.PythonRunner,
) *Handler {
	return &Handler{
		api:            api,
		storage:        storage,
		engine:         engine,
		renderer:       renderer,
		config:         config,
		pyRunner:       pyRunner,
		state:          make(map[int64]string),
		broadcast:      make(map[int64]string),
		lastMsgID:      make(map[int64]string),
		lastDayNav:     make(map[int64]int),
		lastGroupPage:  make(map[int64]int),
		lastActionTime: make(map[int64]time.Time),
		lastWeekNav:    make(map[int64]int),
		lastCorrMsgID:  make(map[int64]string),
	}
}

func (h *Handler) Start(ctx context.Context) {
	go func() {
		for err := range h.api.GetErrors() {
			log.Printf("[API ERROR] %v", err)
		}
	}()
	upd := h.api.GetUpdates(ctx)
	log.Println("[BOT] Бот запущен, ожидаю обновления...")
	for u := range upd {
		go h.handleUpdate(u)
	}
	log.Println("[BOT] Канал обновлений закрыт")
}

func (h *Handler) Stop(ctx context.Context) error { return nil }

// HandleUpdate — публичный метод для webhook (вызывает приватный handleUpdate)
func (h *Handler) HandleUpdate(update schemes.UpdateInterface) {
	h.handleUpdate(update)
}

func (h *Handler) handleUpdate(update schemes.UpdateInterface) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] %v", r)
		}
	}()

	// Анти-спам — игнорируем слишком частые действия
	userID := extractUserID(update)
	if userID != 0 {
		if lastTime, ok := h.lastActionTime[userID]; ok {
			if time.Since(lastTime) < 500*time.Millisecond {
				return
			}
		}
		h.lastActionTime[userID] = time.Now()
	}

	switch u := update.(type) {
	case *schemes.MessageCreatedUpdate:
		h.handleMessage(u)
	case *schemes.MessageCallbackUpdate:
		h.handleCallback(u)
	case *schemes.BotStartedUpdate:
		h.handleBotStarted(u)
	}
}

func (h *Handler) handleMessage(u *schemes.MessageCreatedUpdate) {
	msg := u.Message
	chatID := msg.Recipient.ChatId
	userID := msg.Sender.UserId
	text := strings.TrimSpace(msg.Body.Text)

	log.Printf("[MSG] userID=%d chatID=%d text=%q", userID, chatID, text)

	// Обрабатываем вложенные файлы (xlsx → расписание.xlsx)
	if len(msg.Body.RawAttachments) > 0 && len(text) == 0 {
		h.handleFileAttachment(chatID, userID, msg.Body.RawAttachments)
		return
	}

	// Перед любой командой удаляем старое сервисное сообщение
	if strings.HasPrefix(text, "/") {
		if msgID, ok := h.lastMsgID[userID]; ok {
			h.api.Messages.DeleteMessage(context.Background(), msgID)
			delete(h.lastMsgID, userID)
		}
	}

	switch text {
	case "/start":
		h.handleStart(chatID, userID)
	case "/today", "/сегодня":
		h.showScheduleForDay(chatID, userID, h.engine.GetCurrentDayOfWeek(), h.engine.GetCurrentWeek(), true)
	case "/tomorrow", "/завтра":
		tomorrow := h.engine.GetCurrentDayOfWeek() + 1
		week := h.engine.GetCurrentWeek()
		if tomorrow > 6 { // До СБ
			tomorrow = 1
			week++
		}
		h.showScheduleForDay(chatID, userID, tomorrow, week, true)
	case "/week", "/неделя":
		h.showScheduleForWeek(chatID, userID, h.engine.GetCurrentWeek(), true)
	case "/nweek", "/следующая":
		h.showScheduleForWeek(chatID, userID, h.engine.GetCurrentWeek()+1, true)
	case "/broadcast", "/рассылка":
		h.handleBroadcastCommand(chatID, userID)
	case "/parse_xlsx", "/парсить", "/замены":
		h.handleParseXlsx(chatID, userID)
	case "/update", "/update_all", "/обновить", "/всё":
		h.handleUpdateAll(chatID, userID)
	case "/change_group", "/сменить":
		h.handleChangeGroup(chatID, userID)
	case "/настройки", "/settings":
		h.handleSettings(chatID, userID)
	case "/корректировка", "/changes":
		h.handleUserChanges(chatID, userID)
	case "/myid", "/мойид":
		h.handleMyID(chatID, userID)
	case "/togglemyid", "/myidtoggle":
		h.handleToggleMyID(chatID, userID)
	case "/ahelp", "/админ":
		h.handleAdminHelp(chatID, userID)
	case "/addadmin", "/добавитьадмина":
		h.handleAddAdmin(chatID, userID, text)
	case "/addsuperadmin", "/добавитьсупера":
		h.handleAddSuperAdmin(chatID, userID, text)
	case "/removeadmin", "/удалитьадмина":
		h.handleRemoveAdmin(chatID, userID, text)
	case "/admins", "/админы":
		h.handleListAdmins(chatID, userID)
	default:
		// Удаляем старое "неизвестное" или любое другое сервисное сообщение перед отправкой нового
		if msgID, ok := h.lastMsgID[userID]; ok {
			h.api.Messages.DeleteMessage(context.Background(), msgID)
		}

		if st, ok := h.state[userID]; ok {
			switch st {
			case "awaiting_broadcast":
				h.handleBroadcastMessage(chatID, userID, text)
			default:
				text, _ := h.renderer.Render("unknown_command", nil)
				h.reply(chatID, text)
			}
			return
		}
		if _, exists := h.storage.GetUser(userID); !exists {
			h.handleStart(chatID, userID)
		} else {
			text, _ := h.renderer.Render("unknown_command", nil)
			h.reply(chatID, text)
		}
	}
}

func (h *Handler) handleBotStarted(u *schemes.BotStartedUpdate) {
	h.handleStart(u.ChatId, u.User.UserId)
}

// handleStart — регистрация или сразу показ расписания
func (h *Handler) handleStart(chatID int64, userID int64) {
	if user, exists := h.storage.GetUser(userID); exists {
		// Уже зарегистрирован — показываем расписание на сегодня
		log.Printf("[START] пользователь уже зарегистрирован: %s", user.GroupName)
		h.showScheduleForDay(chatID, userID, h.engine.GetCurrentDayOfWeek(), h.engine.GetCurrentWeek(), true)
		return
	}

	text, _ := h.renderer.Render("welcome_message", map[string]interface{}{
		"Name": "студент",
	})
	msg, err := h.sendMessageWithInlineKeyboard(chatID, text, BuildWelcomeKeyboard())
	if err != nil {
		log.Printf("[START ERROR] %v", err)
		return
	}
	h.lastMsgID[userID] = msg.Body.Mid
}

// showScheduleForDay — показывает/редактирует расписание на конкретный день
func (h *Handler) showScheduleForDay(chatID int64, userID int64, dayOfWeek int, weekNum int, isNew bool) {
	user, exists := h.storage.GetUser(userID)
	if !exists {
		h.handleStart(chatID, userID)
		return
	}

	h.lastWeekNav[userID] = weekNum
	realCurrentWeek := h.engine.GetCurrentWeek()
	schedule := h.storage.GetSchedule()
	lessons := h.engine.Filter(schedule, user.GroupName, weekNum, dayOfWeek)

	now := h.engine.NowMSK()
	dateStr := now.Format("02.01.2006")
	
	// Если мы смотрим текущую неделю, дату считаем от сегодня
	if weekNum == realCurrentWeek {
		if dayOfWeek != h.engine.GetCurrentDayOfWeek() {
			delta := dayOfWeek - h.engine.GetCurrentDayOfWeek()
			dateStr = now.AddDate(0, 0, delta).Format("02.01.2006")
		}
	} else {
		// Если следующая неделя
		deltaWeeks := weekNum - realCurrentWeek
		deltaDays := dayOfWeek - h.engine.GetCurrentDayOfWeek()
		dateStr = now.AddDate(0, 0, deltaWeeks*7+deltaDays).Format("02.01.2006")
	}

	var bodyText string
	if len(lessons) == 0 {
		text, _ := h.renderer.Render("schedule_empty", nil)
		bodyText = text
	} else {
		header, _ := h.renderer.Render("schedule_header", map[string]interface{}{
			"DayName":   scheduler.GetDayName(dayOfWeek),
			"Date":      dateStr,
			"WeekNum":   weekNum,
			"GroupName": user.GroupName,
		})
		bodyText = header + "\n\n"

		// Группировка пар по времени (подгруппы)
		type groupedLesson struct {
			Title     string
			TimeStart string
			TimeEnd   string
			Cabinets  map[string]bool
			Teachers  map[string]bool
			SlotNum   int
		}
		var groups []groupedLesson
		groupMap := make(map[string]*groupedLesson)

		for _, l := range lessons {
			key := fmt.Sprintf("%s-%s", l.TimeSlot.StartTime, l.LessonTitle)
			if g, ok := groupMap[key]; ok {
				if l.Cabinet != "" {
					g.Cabinets[l.Cabinet] = true
				}
				if l.TeacherName != "" {
					g.Teachers[l.TeacherName] = true
				}
			} else {
				nl := groupedLesson{
					Title:     l.LessonTitle,
					TimeStart: l.TimeSlot.StartTime,
					TimeEnd:   l.TimeSlot.EndTime,
					Cabinets:  make(map[string]bool),
					Teachers:  make(map[string]bool),
					SlotNum:   l.TimeSlot.NumberSlot,
				}
				if l.Cabinet != "" {
					nl.Cabinets[l.Cabinet] = true
				}
				if l.TeacherName != "" {
					nl.Teachers[l.TeacherName] = true
				}
				groups = append(groups, nl)
				groupMap[key] = &groups[len(groups)-1]
			}
		}

		// Сортировка groups по SlotNum или времени
		for i := 0; i < len(groups)-1; i++ {
			for j := i + 1; j < len(groups); j++ {
				if groups[i].TimeStart > groups[j].TimeStart {
					groups[i], groups[j] = groups[j], groups[i]
				}
			}
		}

		for i, g := range groups {
			var cabs []string
			for c := range g.Cabinets {
				cabs = append(cabs, c)
			}
			var teachs []string
			for t := range g.Teachers {
				teachs = append(teachs, t)
			}

			cabinetStr := strings.Join(cabs, ", ")
			teacherStr := strings.Join(teachs, ", ")

			lessonText, err := h.renderer.Render("schedule_format", map[string]interface{}{
				"Num":         i + 1, // Или g.SlotNum если нужно реальный номер пары
				"TimeStart":   g.TimeStart,
				"TimeEnd":     g.TimeEnd,
				"LessonTitle": g.Title,
				"Cabinet":     cabinetStr,
				"Teacher":     teacherStr,
			})
			if err != nil {
				log.Printf("[RENDER] %v", err)
				continue
			}
			bodyText += lessonText + "\n\n"
		}
	}

	h.lastDayNav[userID] = dayOfWeek
	kb := BuildDayNavigationKeyboard(dayOfWeek, weekNum)

	if isNew {
		// Отправляем новое сообщение
		msg, err := h.sendMessageWithInlineKeyboard(chatID, bodyText, kb)
		if err != nil {
			log.Printf("[SCHEDULE] ошибка отправки: %v", err)
			return
		}
		h.lastMsgID[userID] = msg.Body.Mid
	} else {
		// Редактируем старое сообщение
		msgID, ok := h.lastMsgID[userID]
		if !ok {
			// Если нет сохранённого — отправляем новое
			msg, err := h.sendMessageWithInlineKeyboard(chatID, bodyText, kb)
			if err != nil {
				log.Printf("[SCHEDULE] ошибка отправки: %v", err)
				return
			}
			h.lastMsgID[userID] = msg.Body.Mid
			return
		}
		err := h.editMessage(chatID, msgID, bodyText, kb)
		if err != nil {
			log.Printf("[EDIT] ошибка редактирования: %v", err)
			// Fallback — новое сообщение
			msg, _ := h.sendMessageWithInlineKeyboard(chatID, bodyText, kb)
			if msg != nil {
				h.lastMsgID[userID] = msg.Body.Mid
			}
		}
	}

	// Если включена авто-корректировка, отправляем изменения
	if user, exists := h.storage.GetUser(userID); exists && user.DailyUpdate {
		// Удаляем предыдущее сообщение с корректировками
		if oldCorrID, ok := h.lastCorrMsgID[userID]; ok {
			h.api.Messages.DeleteMessage(context.Background(), oldCorrID)
			delete(h.lastCorrMsgID, userID)
		}
		// Отправляем свежие корректировки
		corrText := h.buildChangesForGroup(userID, user.GroupName)
		if corrText != "" {
			msg, err := h.sendMessage(chatID, corrText)
			if err == nil && msg != nil {
				h.lastCorrMsgID[userID] = msg.Body.Mid
			}
		}
	}
}

// handleCallback обрабатывает inline-кнопки
func (h *Handler) handleCallback(u *schemes.MessageCallbackUpdate) {
	chatID := u.GetChatID()
	userID := u.Callback.User.UserId
	data := u.Callback.Payload

	log.Printf("[CB] userID=%d chatID=%d data=%q", userID, chatID, data)

	switch {
	case data == "start_select_group":
		// Удаляем приветствие
		if msgID, ok := h.lastMsgID[userID]; ok {
			h.api.Messages.DeleteMessage(context.Background(), msgID)
		}
		// Показываем выбор групп
		groups := ExtractGroupsFromSchedule(h.storage.GetSchedule())
		text, _ := h.renderer.Render("select_group", nil)
		msg, _ := h.sendMessageWithInlineKeyboard(chatID, text, BuildGroupKeyboard(groups, 0))
		h.lastMsgID[userID] = msg.Body.Mid
		h.lastGroupPage[userID] = 0

	case len(data) > 6 && data[:6] == "group_":
		groupName := data[6:]
		firstName := u.Callback.User.FirstName
		lastName := u.Callback.User.LastName
		h.handleGroupSelected(chatID, userID, groupName, firstName, lastName)
	case len(data) > 5 && data[:5] == "page_":
		var page int
		fmt.Sscanf(data[5:], "%d", &page)
		h.handleGroupPage(chatID, userID, page)
	case len(data) > 7 && data[:7] == "daynav_":
		if data == "daynav_today" {
			h.showScheduleForDay(chatID, userID, h.engine.GetCurrentDayOfWeek(), h.engine.GetCurrentWeek(), false)
		} else {
			var day, week int
			n, _ := fmt.Sscanf(data[7:], "%d_%d", &day, &week)
			if n == 2 {
				h.showScheduleForDay(chatID, userID, day, week, false)
			} else {
				// Старый формат или ошибка
				fmt.Sscanf(data[7:], "%d", &day)
				h.showScheduleForDay(chatID, userID, day, h.engine.GetCurrentWeek(), false)
			}
		}
	case data == "broadcast_confirm":
		h.handleBroadcastConfirm(chatID, userID)
	case data == "broadcast_cancel":
		delete(h.state, userID)
		delete(h.broadcast, userID)
		h.reply(chatID, "Рассылка отменена.")

	// Коллбэки для всей недели
	case data == "week_next":
		h.showScheduleForWeek(chatID, userID, h.engine.GetCurrentWeek()+1, false)
	case data == "week_current":
		h.showScheduleForWeek(chatID, userID, h.engine.GetCurrentWeek(), false)

	// Настройки
	case data == "settings_change_group":
		groups := ExtractGroupsFromSchedule(h.storage.GetSchedule())
		text, _ := h.renderer.Render("select_group", nil)
		msg, _ := h.sendMessageWithInlineKeyboard(chatID, text, BuildGroupKeyboard(groups, 0))
		h.lastMsgID[userID] = msg.Body.Mid
	case data == "settings_toggle_corr":
		user, ok := h.storage.GetUser(userID)
		if !ok {
			return
		}
		newVal := !user.DailyUpdate
		if err := h.storage.SetDailyUpdate(userID, newVal); err != nil {
			log.Printf("[SETTINGS] ошибка: %v", err)
			return
		}
		// Обновляем сообщение с настройками
		text := "⚙️ *Настройки*\n\nВыберите действие:"
		msg, _ := h.sendMessageWithInlineKeyboard(chatID, text, BuildSettingsKeyboard(newVal))
		if oldID, ok := h.lastMsgID[userID]; ok {
			h.api.Messages.DeleteMessage(context.Background(), oldID)
		}
		h.lastMsgID[userID] = msg.Body.Mid
	}

	_, err := h.api.Messages.AnswerOnCallback(context.Background(), u.Callback.CallbackID, &schemes.CallbackAnswer{
		Notification: "Готово",
	})
	if err != nil {
		log.Printf("[CB ERROR] %v", err)
	}
}

func (h *Handler) handleGroupSelected(chatID int64, userID int64, groupName string, firstName string, lastName string) {
	user := storage.User{
		FirstName:        firstName,
		LastName:         lastName,
		GroupName:        normalizeGroupName(groupName),
		RegistrationDate: time.Now().Format(time.RFC3339),
	}
	if err := h.storage.SetUser(userID, user); err != nil {
		log.Printf("[SAVE] %v", err)
		h.reply(chatID, "Ошибка при регистрации. Попробуйте позже.")
		return
	}
	delete(h.state, userID)

	// Удаляем сообщение выбора группы (чтобы не мусорило)
	msgID, ok := h.lastMsgID[userID]
	if ok {
		h.api.Messages.DeleteMessage(context.Background(), msgID)
	}

	// Сразу показываем расписание на сегодня (isNew=true — новое сообщение)
	h.showScheduleForDay(chatID, userID, h.engine.GetCurrentDayOfWeek(), h.engine.GetCurrentWeek(), true)
}

func (h *Handler) showScheduleForWeek(chatID int64, userID int64, weekNum int, isNew bool) {
	user, exists := h.storage.GetUser(userID)
	if !exists {
		h.handleStart(chatID, userID)
		return
	}

	h.lastWeekNav[userID] = weekNum
	realCurrentWeek := h.engine.GetCurrentWeek()
	schedule := h.storage.GetSchedule()

	var bodyText string
	header, _ := h.renderer.Render("schedule_header", map[string]interface{}{
		"DayName":   "Вся неделя",
		"Date":      "", // Дата не нужна для всей недели
		"WeekNum":   weekNum,
		"GroupName": user.GroupName,
	})
	bodyText = header + "\n\n"

	daysFound := 0
	for day := 1; day <= 6; day++ { // ПН-СБ
		lessons := h.engine.Filter(schedule, user.GroupName, weekNum, day)
		if len(lessons) == 0 {
			continue
		}
		daysFound++
		bodyText += fmt.Sprintf("----------\n")
		bodyText += fmt.Sprintf("%s\n", strings.ToUpper(scheduler.GetDayName(day)))
		bodyText += fmt.Sprintf("----------\n")
		
		// Группировка
		type grp struct {
			Title, Start, End string
			Cabs, Techs map[string]bool
		}
		var gL []grp
		gM := make(map[string]*grp)

		for _, l := range lessons {
			k := l.TimeSlot.StartTime + l.LessonTitle
			if g, o := gM[k]; o {
				if l.Cabinet != "" { g.Cabs[l.Cabinet] = true }
				if l.TeacherName != "" { g.Techs[l.TeacherName] = true }
			} else {
				nl := grp{l.LessonTitle, l.TimeSlot.StartTime, l.TimeSlot.EndTime, make(map[string]bool), make(map[string]bool)}
				if l.Cabinet != "" { nl.Cabs[l.Cabinet] = true }
				if l.TeacherName != "" { nl.Techs[l.TeacherName] = true }
				gL = append(gL, nl)
				gM[k] = &gL[len(gL)-1]
			}
		}

		for i, g := range gL {
			var cs, ts []string
			for c := range g.Cabs { cs = append(cs, c) }
			for t := range g.Techs { ts = append(ts, t) }
			
			cabinetStr := ""
			if len(cs) > 0 {
				cabinetStr = fmt.Sprintf("📍 %s", strings.Join(cs, ", "))
			}
			
			bodyText += fmt.Sprintf("%d. %s\n", i+1, g.Title)
			bodyText += fmt.Sprintf("   %s — %s\n", g.Start, g.End)
			if cabinetStr != "" {
				bodyText += fmt.Sprintf("   %s\n", cabinetStr)
			}
			if len(ts) > 0 {
				bodyText += fmt.Sprintf("   👤 %s\n", strings.Join(ts, ", "))
			}
			bodyText += "\n"
		}
	}

	if daysFound == 0 {
		bodyText += "Занятий на этой неделе нет 🎉"
	}

	kb := BuildWeekNavigationKeyboard(weekNum, realCurrentWeek)

	if isNew {
		msg, err := h.sendMessageWithInlineKeyboard(chatID, bodyText, kb)
		if err == nil {
			h.lastMsgID[userID] = msg.Body.Mid
		}
	} else {
		msgID, ok := h.lastMsgID[userID]
		if ok {
			h.editMessage(chatID, msgID, bodyText, kb)
		}
	}
}

func (h *Handler) handleGroupPage(chatID int64, userID int64, page int) {
	groups := ExtractGroupsFromSchedule(h.storage.GetSchedule())
	if len(groups) == 0 {
		h.reply(chatID, "Список групп недоступен.")
		return
	}

	// Сохраняем текущую страницу
	h.lastGroupPage[userID] = page

	// Редактируем сообщение вместо отправки нового
	msgID, ok := h.lastMsgID[userID]
	if !ok {
		// Если нет ID сообщения, отправляем новое
		text, _ := h.renderer.Render("select_group", nil)
		msg, err := h.sendMessageWithInlineKeyboard(chatID, text, BuildGroupKeyboard(groups, page))
		if err != nil {
			log.Printf("[PAGE] %v", err)
			return
		}
		h.lastMsgID[userID] = msg.Body.Mid
		return
	}

	text, _ := h.renderer.Render("select_group", nil)
	err := h.editMessage(chatID, msgID, text, BuildGroupKeyboard(groups, page))
	if err != nil {
		log.Printf("[PAGE EDIT] %v", err)
		// Если не удалось редактировать, отправляем новое
		msg, err2 := h.sendMessageWithInlineKeyboard(chatID, text, BuildGroupKeyboard(groups, page))
		if err2 != nil {
			log.Printf("[PAGE] %v", err2)
			return
		}
		h.lastMsgID[userID] = msg.Body.Mid
	}
}

func (h *Handler) handleCurrentWeek(chatID int64) {
	weekNum := h.engine.GetCurrentWeek()
	text, _ := h.renderer.Render("current_week", map[string]interface{}{
		"WeekNum": weekNum,
	})
	h.reply(chatID, text)
}

func (h *Handler) handleSettings(chatID int64, userID int64) {
	user, ok := h.storage.GetUser(userID)
	if !ok {
		h.handleStart(chatID, userID)
		return
	}
	text := "⚙️ *Настройки*\n\nВыберите действие:"
	msg, _ := h.sendMessageWithInlineKeyboard(chatID, text, BuildSettingsKeyboard(user.DailyUpdate))
	if oldID, ok := h.lastMsgID[userID]; ok {
		h.api.Messages.DeleteMessage(context.Background(), oldID)
	}
	h.lastMsgID[userID] = msg.Body.Mid
}

func (h *Handler) handleChangeGroup(chatID int64, userID int64) {
	// Удаляем старое расписание, чтобы не мусорить
	if msgID, ok := h.lastMsgID[userID]; ok {
		h.api.Messages.DeleteMessage(context.Background(), msgID)
	}

	groups := ExtractGroupsFromSchedule(h.storage.GetSchedule())
	if len(groups) == 0 {
		h.reply(chatID, "Список групп недоступен.")
		return
	}
	h.state[userID] = "selecting_group"
	h.lastGroupPage[userID] = 0
	text, _ := h.renderer.Render("select_group", nil)
	msg, err := h.sendMessageWithInlineKeyboard(chatID, text, BuildGroupKeyboard(groups, 0))
	if err != nil {
		log.Printf("[CHANGE GROUP] %v", err)
		return
	}
	h.lastMsgID[userID] = msg.Body.Mid
}

func (h *Handler) handleBroadcastCommand(chatID int64, userID int64) {
	if !h.storage.IsAdmin(userID) {
		text, _ := h.renderer.Render("no_rights", nil)
		h.reply(chatID, text)
		log.Printf("[BROADCAST] отказано userID=%d, chatID=%d (не админ)", userID, chatID)
		return
	}
	h.state[userID] = "awaiting_broadcast"
	text, _ := h.renderer.Render("broadcast_prompt", nil)
	msg, err := h.sendMessage(chatID, text)
	if err != nil {
		log.Printf("[BROADCAST CMD] ошибка отправки prompt: %v", err)
		return
	}
	if msg != nil {
		h.lastMsgID[userID] = msg.Body.Mid
	}
	log.Printf("[BROADCAST] ожидание текста от userID=%d", userID)
}

func (h *Handler) handleBroadcastMessage(chatID int64, userID int64, text string) {
	if !h.storage.IsAdmin(userID) {
		text, _ := h.renderer.Render("no_rights", nil)
		h.reply(chatID, text)
		return
	}
	h.state[userID] = "confirm_broadcast"
	h.broadcast[userID] = text
	preview, _ := h.renderer.Render("broadcast_preview", map[string]interface{}{
		"Text": text,
	})
	kb := &maxbot.Keyboard{}
	row := kb.AddRow()
	row.AddCallback("Подтвердить ✅", schemes.POSITIVE, "broadcast_confirm")
	row.AddCallback("Отмена ❌", schemes.NEGATIVE, "broadcast_cancel")
	msg, err := h.sendMessageWithInlineKeyboard(chatID, preview, kb)
	if err != nil {
		log.Printf("[BROADCAST PREVIEW] ошибка: %v", err)
		return
	}
	if msg != nil {
		h.lastMsgID[userID] = msg.Body.Mid
	}
	log.Printf("[BROADCAST] показан превью для userID=%d, chatID=%d", userID, chatID)
}

func (h *Handler) handleBroadcastConfirm(chatID int64, userID int64) {
	if !h.storage.IsAdmin(userID) {
		return
	}
	text, ok := h.broadcast[userID]
	if !ok {
		h.reply(chatID, "Ошибка: сообщение не найдено")
		return
	}

	users := h.storage.GetAllUsers()
	total := len(users)

	// Уведомляем о начале
	progressText := fmt.Sprintf("⏳ Рассылка начата...\nВсего получателей: %d", total)
	h.reply(chatID, progressText)

	sent, failed := 0, 0
	for uid := range users {
		if err := h.sendToUser(uid, text); err != nil {
			log.Printf("[BROADCAST] ошибка для %d: %v", uid, err)
			failed++
		} else {
			sent++
		}
	}
	delete(h.state, userID)
	delete(h.broadcast, userID)
	result, _ := h.renderer.Render("broadcast_done", map[string]interface{}{
		"Sent":   sent,
		"Failed": failed,
	})
	h.reply(chatID, result)
	log.Printf("[BROADCAST] завершена: userID=%d, получателей=%d, отправлено=%d, ошибок=%d", userID, total, sent, failed)
}

func (h *Handler) IsAdmin(userID int64) bool {
	return h.storage.IsAdmin(userID)
}


// ======== ПАРСИНГ XLSX → CHANGES.JSON → РАССЫЛКА ========

// ChangeEntry — одна запись из changes.json
// changeType: removed / added / status_change
type ChangeEntry struct {
	Date         string `json:"date"`
	DayOfWeek    string `json:"day_of_week"`
	Group        string `json:"group"`
	LessonNumber *int   `json:"lesson_number"`
	Type         string `json:"type"`
	Subject      string `json:"subject"`
	Teacher      string `json:"teacher"`
	Note         string `json:"note"`
}

func (h *Handler) handleParseXlsx(chatID int64, userID int64) {
	if !h.storage.IsAdmin(userID) {
		text, _ := h.renderer.Render("no_rights", nil)
		h.reply(chatID, text)
		return
	}

	xlsxPath := "расписание.xlsx"
	jsonPath := "changes.json"

	h.reply(chatID, "⏳ Парсинг и рассылка запущены в фоне…")

	go func() {
		if err := h.pyRunner.ParseCorrections(xlsxPath, jsonPath); err != nil {
			log.Printf("[PARSE_XLSX] ошибка: %v", err)
			h.reply(chatID, fmt.Sprintf("❌ Ошибка: %v", err))
			return
		}

		data, err := os.ReadFile(jsonPath)
		if err != nil {
			h.reply(chatID, "✅ changes.json создан, но не удалось прочитать")
			return
		}

		var entries []ChangeEntry
		if err := json.Unmarshal(data, &entries); err != nil {
			h.reply(chatID, "✅ changes.json создан, ошибка разбора")
			return
		}

		if len(entries) == 0 {
			h.reply(chatID, "⚠️ Изменений не найдено")
			return
		}

		// Группируем по датам
		type dayGroup struct {
			groups map[string][]ChangeEntry
		}
		days := make(map[string]*dayGroup)
		for _, e := range entries {
			if days[e.Date] == nil {
				days[e.Date] = &dayGroup{groups: make(map[string][]ChangeEntry)}
			}
			days[e.Date].groups[e.Group] = append(days[e.Date].groups[e.Group], e)
		}

		// Отправка — каждому пользователю только его группа
		users := h.storage.GetAllUsers()
		sent, failed := 0, 0
		sentByGroup := make(map[string]int)

		for uid, u := range users {
			userGroup := normalizeGroupName(u.GroupName)
			var userMessages []string

			for date := range days {
				dg := days[date]
				changes, hasGroup := dg.groups[userGroup]
				if !hasGroup {
					for g, ch := range dg.groups {
						if normalizeGroupName(g) == userGroup {
							changes = ch
							hasGroup = true
							break
						}
					}
				}
				if !hasGroup {
					continue
				}

				dayName := ""
				for _, e := range entries {
					if e.Date == date && e.DayOfWeek != "" {
						dayName = e.DayOfWeek
						break
					}
				}

				var sb strings.Builder
				if dayName != "" {
					sb.WriteString(fmt.Sprintf("📋 Изменения расписания на %s (%s)\n\n", date, capitalize(dayName)))
				} else {
					sb.WriteString(fmt.Sprintf("📋 Изменения расписания на %s\n\n", date))
				}

				sort.Slice(changes, func(i, j int) bool {
					a := changes[i].LessonNumber
					b := changes[j].LessonNumber
					if a == nil {
						return false
					}
					if b == nil {
						return true
					}
					return *a < *b
				})

				for _, ch := range changes {
					ln := ""
					if ch.LessonNumber != nil {
						ln = fmt.Sprintf("%d п.", *ch.LessonNumber)
					}

					switch ch.Type {
					case "removed":
						subj := ch.Subject
						if ch.Teacher != "" {
							subj += fmt.Sprintf(" (%s)", ch.Teacher)
						}
						if ch.Note == "снять" || ch.Note == "Снять" {
							sb.WriteString(fmt.Sprintf("  ❌ Снято: %s  %s\n", subj, ln))
						} else {
							sb.WriteString(fmt.Sprintf("  ❌ Отменено: %s  %s\n", subj, ln))
						}
						if ch.Note != "" && ch.Note != "снять" && ch.Note != "Снять" && ch.Note != "Замена" {
							sb.WriteString(fmt.Sprintf("     (%s)\n", ch.Note))
						}

					case "added":
						subj := ch.Subject
						teacher := ""
						if ch.Teacher != "" {
							teacher = ch.Teacher
						}
						sb.WriteString(fmt.Sprintf("  ➕ Добавлено: %s  %s\n", subj, ln))
						if teacher != "" {
							sb.WriteString(fmt.Sprintf("     Преподаватель: %s\n", teacher))
						}
						if ch.Note != "" && ch.Note != "Добавление" {
							sb.WriteString(fmt.Sprintf("     (%s)\n", ch.Note))
						}

					case "status_change":
						sb.WriteString(fmt.Sprintf("  ℹ️ %s\n", ch.Subject))
					}
				}
				userMessages = append(userMessages, sb.String())
			}

			if len(userMessages) == 0 {
				continue
			}

			for _, text := range userMessages {
				if err := h.sendToUser(uid, text); err != nil {
					log.Printf("[PARSE_XLSX] ошибка отправки user=%d: %v", uid, err)
					failed++
				} else {
					sent++
					sentByGroup[userGroup]++
				}
			}
		}

		// Итог
		var result strings.Builder
		result.WriteString(fmt.Sprintf("✅ Рассылка изменений выполнена!\n\n📋 Всего изменений: %d\n", len(entries)))
		for g, count := range sentByGroup {
			result.WriteString(fmt.Sprintf("\n👥 %s: отправлено %d", g, count))
		}
		result.WriteString(fmt.Sprintf("\n\n📊 Отправлено: %d, Ошибок: %d", sent, failed))

		h.reply(chatID, result.String())
	}()
}

// normalizeGroupName удаляет все пробелы из названия группы для сравнения
func normalizeGroupName(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), " ", "")
}

// matchingUserIDs возвращает ID пользователей из списка, чья группа совпадает с одной из targetGroups
func matchingUserIDs(users map[int64]storage.User, targetGroups []string) []int64 {
	var ids []int64
	groupSet := make(map[string]bool)
	for _, g := range targetGroups {
		groupSet[normalizeGroupName(g)] = true
	}
	for uid, u := range users {
		if groupSet[normalizeGroupName(u.GroupName)] {
			ids = append(ids, uid)
		}
	}
	return ids
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return strings.ToUpper(string(runes[:1])) + string(runes[1:])
}

// ======== ПОЛНОЕ ОБНОВЛЕНИЕ (РАСПИСАНИЕ + КОРРЕКТИРОВКИ) ========

func (h *Handler) handleUpdateAll(chatID int64, userID int64) {
	if !h.storage.IsAdmin(userID) {
		text, _ := h.renderer.Render("no_rights", nil)
		h.reply(chatID, text)
		return
	}

	report, err := h.pyRunner.FullPipeline(
		"расписание.xlsx",           // входной xlsx (автопоиск расписание/.xlsx/.xls)
		"Расписание-2.csv",          // промежуточный CSV
		h.config.Files.Schedule,      // schedule.json
		"changes.json",              // changes.json
	)
	if err != nil {
		log.Printf("[UPDATE_ALL] ошибка: %v", err)
		h.reply(chatID, fmt.Sprintf("❌ Ошибка:\n%s\n\n%v", report, err))
		return
	}

	// Перезагружаем данные в памяти
	if err := h.storage.LoadAll(); err != nil {
		log.Printf("[UPDATE_ALL] ошибка перезагрузки: %v", err)
	}

	h.reply(chatID, report)
	log.Printf("[UPDATE_ALL] полное обновление завершено admin=%d", userID)
}

// handleFileAttachment — загрузка xlsx-файла и сохранение как расписание.xlsx
func (h *Handler) handleFileAttachment(chatID int64, userID int64, rawAttachments []json.RawMessage) {
	if !h.storage.IsAdmin(userID) {
		return
	}

	for _, raw := range rawAttachments {
		var att struct {
			Type     schemes.AttachmentType `json:"type"`
			Filename string                 `json:"filename"`
			Payload  struct {
				Url   string `json:"url"`
				Token string `json:"token"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &att); err != nil {
			log.Printf("[FILE] ошибка разбора attachment: %v", err)
			continue
		}

		if att.Type != schemes.AttachmentFile {
			continue
		}

		ext := strings.ToLower(filepath.Ext(att.Filename))
		if ext != ".xlsx" && ext != ".xls" {
			log.Printf("[FILE] пропущен не-xlsx файл: %s", att.Filename)
			h.reply(chatID, fmt.Sprintf("❌ Ожидался .xlsx файл, получен: %s", att.Filename))
			return
		}

		log.Printf("[FILE] получен xlsx: %s (%s)", att.Filename, att.Payload.Url)

		// Скачиваем файл
		outPath := "расписание.xlsx"
		if err := h.downloadFile(att.Payload.Url, outPath); err != nil {
			log.Printf("[FILE] ошибка скачивания: %v", err)
			h.reply(chatID, fmt.Sprintf("❌ Ошибка скачивания файла: %v", err))
			return
		}

		log.Printf("[FILE] сохранён: %s", outPath)
		h.reply(chatID, fmt.Sprintf("✅ Файл сохранён как расписание.xlsx\n\nМожешь запустить /замены для рассылки."))
		return
	}
}

// ======== /myid ========

func (h *Handler) handleMyID(chatID int64, userID int64) {
	if !h.storage.IsMyIDEnabled() {
		return
	}
	h.reply(chatID, fmt.Sprintf("🆔 Твой ID: `%d`", userID))
}

func (h *Handler) handleToggleMyID(chatID int64, userID int64) {
	if !h.storage.IsSuperAdmin(userID) {
		h.reply(chatID, "⛔ Только super_admin")
		return
	}
	enabled, err := h.storage.ToggleMyID()
	if err != nil {
		h.reply(chatID, fmt.Sprintf("❌ Ошибка: %v", err))
		return
	}
	status := "включена"
	if !enabled {
		status = "выключена"
	}
	h.reply(chatID, fmt.Sprintf("✅ Команда /myid теперь %s", status))
}

func (h *Handler) handleAdminHelp(chatID int64, userID int64) {
	if !h.storage.IsAdmin(userID) && !h.storage.IsSuperAdmin(userID) {
		return
	}

	role := "🔧 Admin"
	if h.storage.IsSuperAdmin(userID) {
		role = "👑 Super Admin"
	}

	myidStatus := "включена"
	if !h.storage.IsMyIDEnabled() {
		myidStatus = "выключена"
	}

	text := fmt.Sprintf(`📋 *Admin Panel*

👤 Роль: %s
📌 /myid: %s

🔧 *Команды админа:*
• /рассылка (/broadcast) — рассылка всем
• /замены (/parse_xlsx) — парсинг xlsx
• /обновить (/update) — обновление расписания
• /админ (/ahelp) — эта справка`, role, myidStatus)

	if h.storage.IsSuperAdmin(userID) {
		text += `

👑 *Super Admin:*
• /добавитьадмина <id> (/addadmin)
• /добавитьсупера <id> (/addsuperadmin)
• /удалитьадмина <id> (/removeadmin)
• /админы (/admins) — список ролей
• /myidtoggle (/togglemyid) — вкл/выкл /myid`
	}

	h.reply(chatID, text)
}

// ======== УПРАВЛЕНИЕ РОЛЯМИ (super_admin) ========

func (h *Handler) handleAddAdmin(chatID int64, userID int64, text string) {
	if !h.storage.IsSuperAdmin(userID) {
		h.reply(chatID, "⛔ Только super_admin может добавлять админов")
		return
	}
	parts := strings.Fields(text)
	if len(parts) < 2 {
		h.reply(chatID, "Использование: /addadmin <user_id>")
		return
	}
	targetID := parseInt(parts[1])
	if targetID == 0 {
		h.reply(chatID, "❌ Неверный ID")
		return
	}
	if err := h.storage.AddAdmin(targetID); err != nil {
		h.reply(chatID, fmt.Sprintf("❌ Ошибка: %v", err))
		return
	}
	h.reply(chatID, fmt.Sprintf("✅ Пользователь %d добавлен в admin", targetID))
}

func (h *Handler) handleAddSuperAdmin(chatID int64, userID int64, text string) {
	if !h.storage.IsSuperAdmin(userID) {
		h.reply(chatID, "⛔ Только super_admin может добавлять super_admin")
		return
	}
	parts := strings.Fields(text)
	if len(parts) < 2 {
		h.reply(chatID, "Использование: /addsuperadmin <user_id>")
		return
	}
	targetID := parseInt(parts[1])
	if targetID == 0 {
		h.reply(chatID, "❌ Неверный ID")
		return
	}
	if err := h.storage.AddSuperAdmin(targetID); err != nil {
		h.reply(chatID, fmt.Sprintf("❌ Ошибка: %v", err))
		return
	}
	h.reply(chatID, fmt.Sprintf("✅ Пользователь %d добавлен в super_admin", targetID))
}

func (h *Handler) handleRemoveAdmin(chatID int64, userID int64, text string) {
	if !h.storage.IsSuperAdmin(userID) {
		h.reply(chatID, "⛔ Только super_admin может удалять роли")
		return
	}
	parts := strings.Fields(text)
	if len(parts) < 2 {
		h.reply(chatID, "Использование: /removeadmin <user_id>")
		return
	}
	targetID := parseInt(parts[1])
	if targetID == 0 {
		h.reply(chatID, "❌ Неверный ID")
		return
	}
	if err := h.storage.RemoveAdmin(targetID); err != nil {
		h.reply(chatID, fmt.Sprintf("❌ Ошибка: %v", err))
		return
	}
	h.reply(chatID, fmt.Sprintf("✅ Пользователь %d удалён из всех ролей", targetID))
}

func (h *Handler) handleListAdmins(chatID int64, userID int64) {
	if !h.storage.IsSuperAdmin(userID) {
		h.reply(chatID, "⛔ Только super_admin может просматривать админов")
		return
	}
	admins := h.storage.GetAdmins()
	reply := "📋 Текущие роли:\n\n"
	reply += "👑 *Super Admin:*\n"
	if len(admins.SuperAdmins) == 0 {
		reply += "  нет\n"
	} else {
		for _, id := range admins.SuperAdmins {
			reply += fmt.Sprintf("  • `%d`\n", id)
		}
	}
	reply += "\n🔧 *Admin:*\n"
	if len(admins.Admins) == 0 {
		reply += "  нет\n"
	} else {
		for _, id := range admins.Admins {
			reply += fmt.Sprintf("  • `%d`\n", id)
		}
	}
	h.reply(chatID, reply)
}

// ======== /корректировка — изменения для своей группы ========

// buildChangesForGroup возвращает текст изменений для группы (пусто если нет)
func (h *Handler) buildChangesForGroup(userID int64, groupName string) string {
	data, err := os.ReadFile("changes.json")
	if err != nil {
		return ""
	}

	var allChanges []ChangeEntry
	if err := json.Unmarshal(data, &allChanges); err != nil {
		return ""
	}

	if len(allChanges) == 0 {
		return ""
	}

	userGroup := normalizeGroupName(groupName)
	var myChanges []ChangeEntry
	for _, ch := range allChanges {
		if normalizeGroupName(ch.Group) == userGroup {
			myChanges = append(myChanges, ch)
		}
	}

	if len(myChanges) == 0 {
		return ""
	}

	type dayInfo struct {
		date    string
		dayName string
		entries []ChangeEntry
	}
	dayMap := make(map[string]*dayInfo)
	var dayKeys []string
	for _, ch := range myChanges {
		if dayMap[ch.Date] == nil {
			dayMap[ch.Date] = &dayInfo{date: ch.Date, dayName: ch.DayOfWeek}
			dayKeys = append(dayKeys, ch.Date)
		}
		dayMap[ch.Date].entries = append(dayMap[ch.Date].entries, ch)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *Изменения для %s*\n\n", groupName))

	for _, date := range dayKeys {
		di := dayMap[date]
		changes := di.entries

		sort.Slice(changes, func(i, j int) bool {
			a := changes[i].LessonNumber
			b := changes[j].LessonNumber
			if a == nil {
				return false
			}
			if b == nil {
				return true
			}
			return *a < *b
		})

		head := date
		if di.dayName != "" {
			head += " (" + capitalize(di.dayName) + ")"
		}
		sb.WriteString(fmt.Sprintf("%s\n", head))

		for _, ch := range changes {
			ln := ""
			if ch.LessonNumber != nil {
				ln = fmt.Sprintf(" %d п.", *ch.LessonNumber)
			}

			switch ch.Type {
			case "removed":
				subj := ch.Subject
				if ch.Teacher != "" {
					subj += fmt.Sprintf(" (%s)", ch.Teacher)
				}
				if ch.Note == "снять" || ch.Note == "Снять" {
					sb.WriteString(fmt.Sprintf("❌ Снято: %s%s\n", subj, ln))
				} else {
					sb.WriteString(fmt.Sprintf("❌ Отменено: %s%s\n", subj, ln))
				}
			case "added":
				subj := ch.Subject
				if ch.Teacher != "" {
					sb.WriteString(fmt.Sprintf("➕ Добавлено: %s%s\n", subj, ln))
					sb.WriteString(fmt.Sprintf("   Преподаватель: %s\n", ch.Teacher))
				} else {
					sb.WriteString(fmt.Sprintf("➕ Добавлено: %s%s\n", subj, ln))
				}
			case "status_change":
				sb.WriteString(fmt.Sprintf("ℹ️ %s\n", ch.Subject))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (h *Handler) handleUserChanges(chatID int64, userID int64) {
	user, exists := h.storage.GetUser(userID)
	if !exists {
		h.handleStart(chatID, userID)
		return
	}

	text := h.buildChangesForGroup(userID, user.GroupName)
	if text == "" {
		h.reply(chatID, "📭 Для твоей группы изменений нет")
		return
	}

	h.reply(chatID, text)
}

// parseInt парсит строку в int64
func parseInt(s string) int64 {
	var id int64
	fmt.Sscanf(s, "%d", &id)
	return id
}

// downloadFile скачивает файл по URL и сохраняет локально
func (h *Handler) downloadFile(url, outPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// reply — отправляет простое текстовое сообщение
func (h *Handler) reply(chatID int64, text string) {
	if _, err := h.sendMessage(chatID, text); err != nil {
		log.Printf("[SEND ERROR] %v", err)
	}
}

// sendMessage — отправка текста
func (h *Handler) sendMessage(chatID int64, text string) (*schemes.Message, error) {
	m := maxbot.NewMessage().SetChat(chatID).SetText(text)
	return h.api.Messages.SendWithResult(context.Background(), m)
}

// sendToUser — отправка по user_id
func (h *Handler) sendToUser(userID int64, text string) error {
	m := maxbot.NewMessage().SetUser(userID).SetText(text)
	return h.api.Messages.Send(context.Background(), m)
}

// sendMessageWithInlineKeyboard — отправка с inline-клавиатурой
func (h *Handler) sendMessageWithInlineKeyboard(chatID int64, text string, keyboard *maxbot.Keyboard) (*schemes.Message, error) {
	m := maxbot.NewMessage().SetChat(chatID).SetText(text).AddKeyboard(keyboard)
	return h.api.Messages.SendWithResult(context.Background(), m)
}

// editMessage — редактирование существующего сообщения
func (h *Handler) editMessage(chatID int64, messageID string, text string, keyboard *maxbot.Keyboard) error {
	m := maxbot.NewMessage().SetChat(chatID).SetText(text).AddKeyboard(keyboard)
	return h.api.Messages.EditMessage(context.Background(), messageID, m)
}

func extractUserID(update schemes.UpdateInterface) int64 {
	switch u := update.(type) {
	case *schemes.MessageCreatedUpdate:
		return u.Message.Sender.UserId
	case *schemes.MessageCallbackUpdate:
		return u.Callback.User.UserId
	case *schemes.BotStartedUpdate:
		return u.User.UserId
	}
	return 0
}
