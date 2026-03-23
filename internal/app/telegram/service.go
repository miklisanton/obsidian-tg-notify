package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/app/ports"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type Service struct {
	chats          ports.ChatRepository
	rules          ports.ReminderRepository
	tasks          ports.TaskRepository
	allowedChats   map[int64]struct{}
	defaultVaultID int64
	defaultTZ      string
}

func NewService(chats ports.ChatRepository, rules ports.ReminderRepository, tasks ports.TaskRepository, allowed []int64, defaultVaultID int64, defaultTZ string) *Service {
	allowedChats := make(map[int64]struct{}, len(allowed))
	for _, chatID := range allowed {
		allowedChats[chatID] = struct{}{}
	}
	return &Service{chats: chats, rules: rules, tasks: tasks, allowedChats: allowedChats, defaultVaultID: defaultVaultID, defaultTZ: defaultTZ}
}

func (s *Service) Handle(ctx context.Context, chatID int64, text string) (string, error) {
	if _, ok := s.allowedChats[chatID]; !ok {
		return "chat not allowed", nil
	}
	if err := s.chats.Ensure(ctx, chatID); err != nil {
		return "", err
	}

	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) == 0 {
		return helpText(), nil
	}

	command := normalizeCommand(parts[0])
	args := parts[1:]

	switch command {
	case "/start", "/help":
		return helpText(), nil
	case "/rules", "/list":
		return s.listRules(ctx, chatID)
	case "/due":
		return s.handleDue(ctx, args)
	case "/add", "/new":
		return s.handleAdd(ctx, chatID, args)
	case "/disable", "/off":
		if len(args) != 1 {
			return "usage: /disable RULE_ID", nil
		}
		if err := s.rules.Disable(ctx, args[0], chatID); err != nil {
			return "", err
		}
		return "disabled " + args[0], nil
	default:
		return "unknown command\n\n" + helpText(), nil
	}
}

func (s *Service) handleDue(ctx context.Context, args []string) (string, error) {
	scope := task.DueScopeAll
	if len(args) > 0 {
		switch args[0] {
		case "all":
			scope = task.DueScopeAll
			args = args[1:]
		case "daily":
			scope = task.DueScopeDaily
			args = args[1:]
		case "weekly":
			scope = task.DueScopeWeekly
			args = args[1:]
		}
	}
	from, to, err := parseDueRange(args, s.defaultTZ)
	if err != nil {
		return "", err
	}
	items, err := s.tasks.ListDueTasksInRange(ctx, task.DueRangeFilter{VaultID: s.defaultVaultID, Scope: scope, From: from, To: to})
	if err != nil {
		return "", err
	}
	return formatDueTasks(scope, from, to, items), nil
}

func (s *Service) handleAdd(ctx context.Context, chatID int64, args []string) (string, error) {
	if len(args) == 0 {
		return addUsage(), nil
	}

	var (
		rule reminder.Rule
		err  error
	)

	switch args[0] {
	case "new-task":
		if len(args) != 1 {
			return "usage: /add new-task", nil
		}
		rule = s.newRule(chatID, reminder.RuleKindNewTask, s.defaultTZ, reminder.Schedule{}, reminder.NewTaskConfig{})
	case "due-today":
		rule, err = s.ruleWithDailyTime(chatID, reminder.RuleKindDueTasks, reminder.DueTasksConfig{Window: task.DueWindowToday}, args[1:])
	case "daily-prompt":
		rule, err = s.ruleWithDailyTime(chatID, reminder.RuleKindPromptDailyGoals, reminder.PromptDailyGoalsConfig{}, args[1:])
	case "weekly-prompt":
		rule, err = s.ruleWithSlots(chatID, reminder.RuleKindPromptWeeklyGoals, reminder.PromptWeeklyGoalsConfig{}, args[1:])
	case "weekly-review":
		rule, err = s.ruleWithSlots(chatID, reminder.RuleKindReviewWeeklyUnfinished, reminder.ReviewWeeklyUnfinishedConfig{}, args[1:])
	default:
		return addUsage(), nil
	}
	if err != nil {
		return "", err
	}
	return s.insert(ctx, rule)
}

func (s *Service) listRules(ctx context.Context, chatID int64) (string, error) {
	rules, err := s.rules.ListByChat(ctx, chatID)
	if err != nil {
		return "", err
	}
	if len(rules) == 0 {
		return "no rules\n\n" + addUsage(), nil
	}

	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Enabled != rules[j].Enabled {
			return rules[i].Enabled
		}
		return rules[i].CreatedAt.After(rules[j].CreatedAt)
	})

	lines := make([]string, 0, len(rules)+1)
	lines = append(lines, "rules:")
	for _, rule := range rules {
		state := "on"
		if !rule.Enabled {
			state = "off"
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] %s - %s", shortRuleID(rule.ID), state, describeRule(rule), describeSchedule(rule.Schedule)))
	}
	lines = append(lines, "disable: /disable RULE_ID")
	return strings.Join(lines, "\n"), nil
}

func (s *Service) ruleWithDailyTime(chatID int64, kind reminder.RuleKind, cfg reminder.Config, args []string) (reminder.Rule, error) {
	if len(args) != 1 {
		return reminder.Rule{}, fmt.Errorf("usage: %s HH:MM", kind)
	}
	tm, err := reminder.ParseLocalTime(args[0])
	if err != nil {
		return reminder.Rule{}, err
	}
	return s.newRule(chatID, kind, s.defaultTZ, reminder.Schedule{Slots: []reminder.ScheduleSlot{{Time: tm}}}, cfg), nil
}

func (s *Service) ruleWithSlots(chatID int64, kind reminder.RuleKind, cfg reminder.Config, args []string) (reminder.Rule, error) {
	if len(args) == 0 || len(args)%2 != 0 {
		return reminder.Rule{}, fmt.Errorf("usage: %s day HH:MM [day HH:MM...]", kind)
	}
	slots := make([]reminder.ScheduleSlot, 0, len(args)/2)
	for index := 0; index < len(args); index += 2 {
		weekday, err := reminder.ParseWeekday(args[index])
		if err != nil {
			return reminder.Rule{}, err
		}
		tm, err := reminder.ParseLocalTime(args[index+1])
		if err != nil {
			return reminder.Rule{}, err
		}
		slots = append(slots, reminder.ScheduleSlot{Weekdays: []time.Weekday{weekday}, Time: tm})
	}
	return s.newRule(chatID, kind, s.defaultTZ, reminder.Schedule{Slots: slots}, cfg), nil
}

func (s *Service) insert(ctx context.Context, rule reminder.Rule) (string, error) {
	if err := s.rules.Insert(ctx, rule); err != nil {
		return "", err
	}
	return fmt.Sprintf("added [%s] %s\n%s", shortRuleID(rule.ID), describeRule(rule), describeSchedule(rule.Schedule)), nil
}

func (s *Service) newRule(chatID int64, kind reminder.RuleKind, timezone string, schedule reminder.Schedule, cfg reminder.Config) reminder.Rule {
	now := time.Now()
	return reminder.Rule{
		ID:        uuid.NewString(),
		ChatID:    chatID,
		VaultID:   s.defaultVaultID,
		Kind:      kind,
		Timezone:  timezone,
		Schedule:  schedule,
		Config:    cfg,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func normalizeCommand(raw string) string {
	if index := strings.IndexByte(raw, '@'); index >= 0 {
		return raw[:index]
	}
	return raw
}

func shortRuleID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func describeRule(rule reminder.Rule) string {
	switch cfg := rule.Config.(type) {
	case reminder.NewTaskConfig:
		_ = cfg
		return "new task"
	case reminder.DueTasksConfig:
		return "due " + string(cfg.Window)
	case reminder.PromptDailyGoalsConfig:
		return "daily prompt"
	case reminder.PromptWeeklyGoalsConfig:
		return "weekly prompt"
	case reminder.ReviewWeeklyUnfinishedConfig:
		return "weekly review"
	default:
		return string(rule.Kind)
	}
}

func describeSchedule(schedule reminder.Schedule) string {
	if len(schedule.Slots) == 0 {
		return "instant"
	}
	parts := make([]string, 0, len(schedule.Slots))
	for _, slot := range schedule.Slots {
		if len(slot.Weekdays) == 0 {
			parts = append(parts, "daily "+slot.Time.String())
			continue
		}
		days := make([]string, 0, len(slot.Weekdays))
		for _, weekday := range slot.Weekdays {
			days = append(days, shortWeekday(weekday))
		}
		parts = append(parts, strings.Join(days, ",")+" "+slot.Time.String())
	}
	return strings.Join(parts, " | ")
}

func shortWeekday(day time.Weekday) string {
	switch day {
	case time.Sunday:
		return "sun"
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	case time.Saturday:
		return "sat"
	default:
		return day.String()
	}
}

func helpText() string {
	return strings.Join([]string{
		"commands:",
		"- /rules - list rules",
		"- /due [all|daily|weekly] [YYYY-MM-DD] [YYYY-MM-DD]",
		"- /add new-task",
		"- /add due-today HH:MM",
		"- /add daily-prompt HH:MM",
		"- /add weekly-prompt day HH:MM [day HH:MM...]",
		"- /add weekly-review day HH:MM [day HH:MM...]",
		"- /disable RULE_ID",
	}, "\n")
}

func addUsage() string {
	return strings.Join([]string{
		"add usage:",
		"- /due all        # last 14 days..today",
		"- /due daily 2026-03-23",
		"- /due weekly 2026-03-23 2026-03-30",
		"- /add new-task",
		"- /add due-today 09:00",
		"- /add daily-prompt 08:00",
		"- /add weekly-prompt sun 20:00",
		"- /add weekly-review mon 08:00 wed 08:00 fri 10:00",
	}, "\n")
}

func parseDueRange(args []string, timezone string) (task.Date, task.Date, error) {
	loc, err := config.LoadLocation(timezone)
	if err != nil {
		return "", "", err
	}
	today := task.Today(time.Now(), loc)
	if len(args) == 0 {
		from := task.Date(today.Time(loc).AddDate(0, 0, -13).Format("2006-01-02"))
		return from, today, nil
	}
	if len(args) == 1 {
		date, err := task.ParseDate(args[0])
		if err != nil {
			return "", "", err
		}
		return date, date, nil
	}
	if len(args) == 2 {
		from, err := task.ParseDate(args[0])
		if err != nil {
			return "", "", err
		}
		to, err := task.ParseDate(args[1])
		if err != nil {
			return "", "", err
		}
		if string(from) > string(to) {
			return "", "", fmt.Errorf("bad range: from after to")
		}
		return from, to, nil
	}
	return "", "", fmt.Errorf("usage: /due [all|daily|weekly] [YYYY-MM-DD] [YYYY-MM-DD]")
}

func formatDueTasks(scope task.DueScope, from task.Date, to task.Date, items []task.Snapshot) string {
	headline := fmt.Sprintf("due %s %s", scope, string(from))
	if from != to {
		headline = fmt.Sprintf("due %s %s..%s", scope, string(from), string(to))
	}
	if len(items) == 0 {
		return headline + "\n- none"
	}
	lines := []string{headline}
	for _, item := range items {
		due := ""
		if item.DueDate != nil {
			due = string(*item.DueDate) + " "
		}
		lines = append(lines, fmt.Sprintf("- %s%s (%s)", due, item.Body, item.SourcePath))
	}
	return strings.Join(lines, "\n")
}
