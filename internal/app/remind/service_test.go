package remind

import (
	"context"
	"strings"
	"testing"
	"time"

	"obsidian-notify/internal/domain/message"
	"obsidian-notify/internal/domain/notification"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type remindTaskRepoStub struct {
	doc      task.DocumentSnapshot
	ok       bool
	due      []task.Snapshot
	dueRange []task.Snapshot
}

func (remindTaskRepoStub) ListFileTasks(context.Context, int64, string) ([]task.Snapshot, error) {
	return nil, nil
}

func (remindTaskRepoStub) ReplaceFile(context.Context, task.DocumentSnapshot, []task.Snapshot) error {
	return nil
}

func (r remindTaskRepoStub) ListDueTasks(context.Context, task.DueFilter) ([]task.Snapshot, error) {
	return r.due, nil
}

func (r remindTaskRepoStub) ListDueTasksInRange(context.Context, task.DueRangeFilter) ([]task.Snapshot, error) {
	return r.dueRange, nil
}

func (remindTaskRepoStub) GetDocument(context.Context, int64, string) (task.DocumentSnapshot, bool, error) {
	return task.DocumentSnapshot{}, false, nil
}

func (r remindTaskRepoStub) GetDailyDocument(context.Context, int64, task.Date) (task.DocumentSnapshot, bool, error) {
	return r.doc, r.ok, nil
}

func (remindTaskRepoStub) ListWeeklyDocuments(context.Context, int64, int, int) ([]task.DocumentSnapshot, error) {
	return nil, nil
}

func (remindTaskRepoStub) ListCurrentWeekUnfinished(context.Context, int64, int, int) ([]task.Snapshot, error) {
	return nil, nil
}

type remindMessageRepoStub struct {
	items []message.IncomingText
}

func (remindMessageRepoStub) SaveIncomingText(context.Context, message.IncomingText) error {
	return nil
}

func (r remindMessageRepoStub) ListIncomingTexts(context.Context, int64, time.Time, time.Time) ([]message.IncomingText, error) {
	return r.items, nil
}

type remindRuleRepoStub struct{}

func (remindRuleRepoStub) ListActive(context.Context) ([]reminder.Rule, error) { return nil, nil }
func (remindRuleRepoStub) ListByChat(context.Context, int64) ([]reminder.Rule, error) {
	return nil, nil
}
func (remindRuleRepoStub) Insert(context.Context, reminder.Rule) error  { return nil }
func (remindRuleRepoStub) Disable(context.Context, string, int64) error { return nil }

type remindNotificationRepoStub struct{}

func (remindNotificationRepoStub) WasSent(context.Context, string) (bool, error) { return false, nil }
func (remindNotificationRepoStub) RecordSent(context.Context, notification.Intent, time.Time) error {
	return nil
}

func TestEvaluateDailySummaryPromptIncludesMessages(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+3", 3*3600)
	items := []message.IncomingText{
		{ChatID: 1, TelegramMessageID: 1, Text: "shipped parser fix", SentAt: time.Date(2026, 4, 14, 6, 15, 0, 0, time.UTC)},
		{ChatID: 1, TelegramMessageID: 2, Text: "need summary reminder", SentAt: time.Date(2026, 4, 14, 16, 40, 0, 0, time.UTC)},
	}
	evaluator := NewEvaluator(RealClock{}, remindTaskRepoStub{}, remindMessageRepoStub{items: items}, remindRuleRepoStub{}, remindNotificationRepoStub{}, nil)
	rule := reminder.Rule{ID: "rule-1", ChatID: 1, VaultID: 1, Kind: reminder.RuleKindPromptDailySummary, Timezone: "UTC+3", Config: reminder.PromptDailySummaryConfig{}}
	now := time.Date(2026, 4, 14, 21, 0, 0, 0, loc)

	intents, err := evaluator.evaluateRule(context.Background(), now, rule, loc)
	if err != nil {
		t.Fatalf("evaluate rule: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(intents))
	}
	text := intents[0].Text
	for _, want := range []string{"Today's summary missing for 2026-04-14", "09:15 shipped parser fix", "19:40 need summary reminder"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in %q", want, text)
		}
	}
}

func TestEvaluateDailySummaryPromptSkipsWhenSummaryExists(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+3", 3*3600)
	date := task.MustParseDate("2026-04-14")
	evaluator := NewEvaluator(RealClock{}, remindTaskRepoStub{doc: task.DocumentSnapshot{DailyDate: &date, HasDailySummary: true}, ok: true}, remindMessageRepoStub{}, remindRuleRepoStub{}, remindNotificationRepoStub{}, nil)
	rule := reminder.Rule{ID: "rule-1", ChatID: 1, VaultID: 1, Kind: reminder.RuleKindPromptDailySummary, Timezone: "UTC+3", Config: reminder.PromptDailySummaryConfig{}}
	now := time.Date(2026, 4, 14, 21, 0, 0, 0, loc)

	intents, err := evaluator.evaluateRule(context.Background(), now, rule, loc)
	if err != nil {
		t.Fatalf("evaluate rule: %v", err)
	}
	if len(intents) != 0 {
		t.Fatalf("expected no intents, got %d", len(intents))
	}
}

func TestEvaluateDueTasksUsesRangeSemanticsForToday(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("UTC+3", 3*3600)
	today := task.MustParseDate("2026-04-14")
	evaluator := NewEvaluator(RealClock{}, remindTaskRepoStub{dueRange: []task.Snapshot{{Body: "Daily note task", SourcePath: "Daily/2026-04-14.md", DailyDate: &today}}}, remindMessageRepoStub{}, remindRuleRepoStub{}, remindNotificationRepoStub{}, nil)
	rule := reminder.Rule{ID: "rule-1", ChatID: 1, VaultID: 1, Kind: reminder.RuleKindDueTasks, Timezone: "UTC+3", Config: reminder.DueTasksConfig{Window: task.DueWindowToday}}
	now := time.Date(2026, 4, 14, 9, 0, 0, 0, loc)

	intents, err := evaluator.evaluateRule(context.Background(), now, rule, loc)
	if err != nil {
		t.Fatalf("evaluate rule: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(intents))
	}
	if !strings.Contains(intents[0].Text, "Daily note task") {
		t.Fatalf("expected due task in %q", intents[0].Text)
	}
}
