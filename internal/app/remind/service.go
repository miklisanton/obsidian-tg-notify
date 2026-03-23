package remind

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/app/ports"
	"obsidian-notify/internal/app/syncer"
	"obsidian-notify/internal/domain/notification"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type Clock interface {
	Now() time.Time
	Location(name string) (*time.Location, error)
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

func (RealClock) Location(name string) (*time.Location, error) {
	return config.LoadLocation(name)
}

type GoalsService struct {
	tasks      ports.TaskRepository
	classifier WeeklyAreaReader
}

type WeeklyAreaReader interface {
	ListWeeklyAreas(root string) ([]string, error)
}

func NewGoalsService(tasks ports.TaskRepository, classifier WeeklyAreaReader) *GoalsService {
	return &GoalsService{tasks: tasks, classifier: classifier}
}

type Evaluator struct {
	clock         Clock
	tasks         ports.TaskRepository
	rules         ports.ReminderRepository
	notifications ports.NotificationRepository
	goals         *GoalsService
	sender        ports.TelegramSender
	vaultRootByID map[int64]string
}

func NewEvaluator(clock Clock, tasks ports.TaskRepository, rules ports.ReminderRepository, notifications ports.NotificationRepository, goals *GoalsService) *Evaluator {
	return &Evaluator{clock: clock, tasks: tasks, rules: rules, notifications: notifications, goals: goals, vaultRootByID: make(map[int64]string)}
}

func (e *Evaluator) SetSender(sender ports.TelegramSender) {
	e.sender = sender
}

func (e *Evaluator) SetVaultRoots(vaults map[int64]string) {
	e.vaultRootByID = vaults
}

func (e *Evaluator) RunDue(ctx context.Context) error {
	if e.sender == nil {
		return nil
	}
	rules, err := e.rules.ListActive(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		loc, err := e.clock.Location(rule.Timezone)
		if err != nil {
			return err
		}
		now := e.clock.Now().In(loc)
		if !rule.Schedule.Matches(now) {
			continue
		}
		intents, err := e.evaluateRule(ctx, now, rule, loc)
		if err != nil {
			return err
		}
		for _, intent := range intents {
			if err := syncer.SendIntent(ctx, e.notifications, e.sender, intent, e.clock.Now()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Evaluator) evaluateRule(ctx context.Context, now time.Time, rule reminder.Rule, loc *time.Location) ([]notification.Intent, error) {
	switch cfg := rule.Config.(type) {
	case reminder.NewTaskConfig:
		_ = cfg
		return nil, nil
	case reminder.DueTasksConfig:
		items, err := e.tasks.ListDueTasks(ctx, task.DueFilter{VaultID: rule.VaultID, Now: now, Loc: loc, Window: cfg.Window})
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, nil
		}
		lines := []string{"Due tasks:"}
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("- %s (%s)", item.Body, item.SourcePath))
		}
		return []notification.Intent{{
			DedupeKey: fmt.Sprintf("due:%s:%s:%s", rule.ID, cfg.Window, task.Today(now, loc)),
			ChatID:    rule.ChatID,
			RuleID:    rule.ID,
			Kind:      rule.Kind,
			Text:      strings.Join(lines, "\n"),
		}}, nil
	case reminder.PromptDailyGoalsConfig:
		date := task.Today(now, loc)
		doc, exists, err := e.tasks.GetDailyDocument(ctx, rule.VaultID, date)
		if err != nil {
			return nil, err
		}
		if exists && doc.HasNonBlankTask {
			return nil, nil
		}
		return []notification.Intent{{
			DedupeKey: fmt.Sprintf("daily_prompt:%s:%s", rule.ID, date),
			ChatID:    rule.ChatID,
			RuleID:    rule.ID,
			Kind:      rule.Kind,
			Text:      fmt.Sprintf("Daily goals missing for %s", date),
		}}, nil
	case reminder.PromptWeeklyGoalsConfig:
		year, week := now.ISOWeek()
		missing, err := e.missingWeeklyAreas(ctx, rule.VaultID, year, week)
		if err != nil {
			return nil, err
		}
		if len(missing) == 0 {
			return nil, nil
		}
		return []notification.Intent{{
			DedupeKey: fmt.Sprintf("weekly_prompt:%s:%04d-W%02d:%s", rule.ID, year, week, strings.Join(missing, ",")),
			ChatID:    rule.ChatID,
			RuleID:    rule.ID,
			Kind:      rule.Kind,
			Text:      fmt.Sprintf("Weekly goals missing for: %s", strings.Join(missing, ", ")),
		}}, nil
	case reminder.ReviewWeeklyUnfinishedConfig:
		year, week := now.ISOWeek()
		items, err := e.tasks.ListCurrentWeekUnfinished(ctx, rule.VaultID, year, week)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, nil
		}
		lines := []string{fmt.Sprintf("Weekly unfinished W%02d-%d:", week, year)}
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("- %s (%s)", item.Body, item.SourcePath))
		}
		return []notification.Intent{{
			DedupeKey: fmt.Sprintf("weekly_review:%s:%04d-W%02d", rule.ID, year, week),
			ChatID:    rule.ChatID,
			RuleID:    rule.ID,
			Kind:      rule.Kind,
			Text:      strings.Join(lines, "\n"),
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported config %T", cfg)
	}
}

func (e *Evaluator) missingWeeklyAreas(ctx context.Context, vaultID int64, year int, week int) ([]string, error) {
	docs, err := e.tasks.ListWeeklyDocuments(ctx, vaultID, year, week)
	if err != nil {
		return nil, err
	}
	root := e.vaultRootByID[vaultID]
	if root == "" {
		return nil, nil
	}
	areas, err := e.goals.classifier.ListWeeklyAreas(root)
	if err != nil {
		return nil, err
	}
	present := make(map[string]struct{}, len(docs))
	for _, doc := range docs {
		if doc.WeeklyArea == nil || !doc.HasNonBlankTask {
			continue
		}
		present[*doc.WeeklyArea] = struct{}{}
	}
	var missing []string
	for _, area := range areas {
		if _, ok := present[area]; !ok {
			missing = append(missing, area)
		}
	}
	sort.Strings(missing)
	return missing, nil
}
