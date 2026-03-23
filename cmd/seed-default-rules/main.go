package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/google/uuid"

	"obsidian-notify/internal/adapter/config"
	pg "obsidian-notify/internal/adapter/postgres"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type ruleSpec struct {
	kind     reminder.RuleKind
	schedule reminder.Schedule
	config   reminder.Config
	label    string
}

func main() {
	ctx := context.Background()
	path := os.Getenv("APP_CONFIG")
	if path == "" {
		path = "config.yaml"
	}

	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := pg.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.DB.Close() }()

	if err := pg.Migrate(db.DB, "migrations"); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	vaultRepo := pg.NewVaultRepository(db)
	chatRepo := pg.NewChatRepository(db)
	ruleRepo := pg.NewReminderRepository(db)

	if err := vaultRepo.EnsureConfigured(ctx, cfg.Vaults); err != nil {
		log.Fatalf("seed vaults: %v", err)
	}

	specs := defaultRuleSpecs()
	inserted := 0
	skipped := 0

	for _, chatID := range cfg.Telegram.AllowedChatIDs {
		if err := chatRepo.Ensure(ctx, chatID); err != nil {
			log.Fatalf("ensure chat %d: %v", chatID, err)
		}

		existing, err := ruleRepo.ListByChat(ctx, chatID)
		if err != nil {
			log.Fatalf("list rules for chat %d: %v", chatID, err)
		}

		for _, vault := range cfg.Vaults {
			for _, spec := range specs {
				rule := reminder.Rule{
					ID:        uuid.NewString(),
					ChatID:    chatID,
					VaultID:   vault.ID,
					Kind:      spec.kind,
					Timezone:  cfg.App.Timezone,
					Schedule:  spec.schedule,
					Config:    spec.config,
					Enabled:   true,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				if hasEquivalent(existing, rule) {
					skipped++
					fmt.Printf("skip chat=%d vault=%d rule=%s\n", chatID, vault.ID, spec.label)
					continue
				}

				if err := ruleRepo.Insert(ctx, rule); err != nil {
					log.Fatalf("insert rule %s for chat %d vault %d: %v", spec.label, chatID, vault.ID, err)
				}
				existing = append(existing, rule)
				inserted++
				fmt.Printf("add chat=%d vault=%d rule=%s\n", chatID, vault.ID, spec.label)
			}
		}
	}

	fmt.Printf("done inserted=%d skipped=%d\n", inserted, skipped)
}

func defaultRuleSpecs() []ruleSpec {
	return []ruleSpec{
		{
			kind:     reminder.RuleKindNewTask,
			schedule: reminder.Schedule{},
			config:   reminder.NewTaskConfig{},
			label:    "new_task",
		},
		{
			kind: reminder.RuleKindPromptDailyGoals,
			schedule: reminder.Schedule{Slots: []reminder.ScheduleSlot{{
				Time: reminder.LocalTime{Hour: 8, Minute: 0},
			}}},
			config: reminder.PromptDailyGoalsConfig{},
			label:  "daily_prompt_08_00",
		},
		{
			kind: reminder.RuleKindPromptWeeklyGoals,
			schedule: reminder.Schedule{Slots: []reminder.ScheduleSlot{{
				Weekdays: []time.Weekday{time.Sunday},
				Time:     reminder.LocalTime{Hour: 20, Minute: 0},
			}}},
			config: reminder.PromptWeeklyGoalsConfig{},
			label:  "weekly_prompt_sun_20_00",
		},
		{
			kind: reminder.RuleKindDueTasks,
			schedule: reminder.Schedule{Slots: []reminder.ScheduleSlot{{
				Time: reminder.LocalTime{Hour: 9, Minute: 0},
			}}},
			config: reminder.DueTasksConfig{Window: task.DueWindowToday},
			label:  "due_today_09_00",
		},
		{
			kind: reminder.RuleKindReviewWeeklyUnfinished,
			schedule: reminder.Schedule{Slots: []reminder.ScheduleSlot{
				{Weekdays: []time.Weekday{time.Monday}, Time: reminder.LocalTime{Hour: 8, Minute: 0}},
				{Weekdays: []time.Weekday{time.Wednesday}, Time: reminder.LocalTime{Hour: 8, Minute: 0}},
				{Weekdays: []time.Weekday{time.Friday}, Time: reminder.LocalTime{Hour: 10, Minute: 0}},
			}},
			config: reminder.ReviewWeeklyUnfinishedConfig{},
			label:  "weekly_review_mon_wed_fri",
		},
	}
}

func hasEquivalent(existing []reminder.Rule, candidate reminder.Rule) bool {
	for _, item := range existing {
		if item.ChatID != candidate.ChatID || item.VaultID != candidate.VaultID || item.Kind != candidate.Kind || item.Timezone != candidate.Timezone || item.Enabled != candidate.Enabled {
			continue
		}
		if !reflect.DeepEqual(item.Schedule, candidate.Schedule) {
			continue
		}
		if sameConfig(item.Config, candidate.Config) {
			return true
		}
	}
	return false
}

func sameConfig(left reminder.Config, right reminder.Config) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return reflect.TypeOf(left) == reflect.TypeOf(right)
	}
	return reflect.TypeOf(left) == reflect.TypeOf(right) && string(leftJSON) == string(rightJSON)
}
