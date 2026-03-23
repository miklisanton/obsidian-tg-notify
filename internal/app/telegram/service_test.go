package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type chatRepoStub struct{}

func (chatRepoStub) Ensure(context.Context, int64) error { return nil }

type taskRepoStub struct {
	due []task.Snapshot
}

func (taskRepoStub) ListFileTasks(context.Context, int64, string) ([]task.Snapshot, error) {
	return nil, nil
}
func (taskRepoStub) ReplaceFile(context.Context, task.DocumentSnapshot, []task.Snapshot) error {
	return nil
}
func (taskRepoStub) ListDueTasks(context.Context, task.DueFilter) ([]task.Snapshot, error) {
	return nil, nil
}
func (t taskRepoStub) ListDueTasksInRange(context.Context, task.DueRangeFilter) ([]task.Snapshot, error) {
	return t.due, nil
}
func (taskRepoStub) GetDocument(context.Context, int64, string) (task.DocumentSnapshot, bool, error) {
	return task.DocumentSnapshot{}, false, nil
}
func (taskRepoStub) GetDailyDocument(context.Context, int64, task.Date) (task.DocumentSnapshot, bool, error) {
	return task.DocumentSnapshot{}, false, nil
}
func (taskRepoStub) ListWeeklyDocuments(context.Context, int64, int, int) ([]task.DocumentSnapshot, error) {
	return nil, nil
}
func (taskRepoStub) ListCurrentWeekUnfinished(context.Context, int64, int, int) ([]task.Snapshot, error) {
	return nil, nil
}

type ruleRepoStub struct {
	inserted []reminder.Rule
	rules    []reminder.Rule
	disabled []string
}

func (r *ruleRepoStub) ListActive(context.Context) ([]reminder.Rule, error) { return r.rules, nil }
func (r *ruleRepoStub) ListByChat(context.Context, int64) ([]reminder.Rule, error) {
	return r.rules, nil
}
func (r *ruleRepoStub) Insert(_ context.Context, rule reminder.Rule) error {
	r.inserted = append(r.inserted, rule)
	return nil
}
func (r *ruleRepoStub) Disable(_ context.Context, ruleID string, _ int64) error {
	r.disabled = append(r.disabled, ruleID)
	return nil
}

func TestHelpTextOnStart(t *testing.T) {
	t.Parallel()

	service := NewService(chatRepoStub{}, &ruleRepoStub{}, taskRepoStub{}, []int64{1}, 1, "UTC+3")
	text, err := service.Handle(context.Background(), 1, "/start")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if text == "" || text[:9] != "commands:" {
		t.Fatalf("bad help text: %q", text)
	}
}

func TestAddWeeklyReviewRule(t *testing.T) {
	t.Parallel()

	repo := &ruleRepoStub{}
	service := NewService(chatRepoStub{}, repo, taskRepoStub{}, []int64{1}, 1, "UTC+3")
	text, err := service.Handle(context.Background(), 1, "/add weekly-review mon 08:00 fri 10:00")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected 1 inserted rule, got %d", len(repo.inserted))
	}
	rule := repo.inserted[0]
	if rule.Kind != reminder.RuleKindReviewWeeklyUnfinished {
		t.Fatalf("bad rule kind: %s", rule.Kind)
	}
	if len(rule.Schedule.Slots) != 2 {
		t.Fatalf("bad slots count: %d", len(rule.Schedule.Slots))
	}
	if text == "" {
		t.Fatal("expected response text")
	}
}

func TestRulesListPretty(t *testing.T) {
	t.Parallel()

	repo := &ruleRepoStub{rules: []reminder.Rule{{
		ID:        "12345678-abcd",
		ChatID:    1,
		VaultID:   1,
		Kind:      reminder.RuleKindPromptDailyGoals,
		Timezone:  "UTC+3",
		Schedule:  reminder.Schedule{Slots: []reminder.ScheduleSlot{{Time: reminder.LocalTime{Hour: 8, Minute: 0}}}},
		Config:    reminder.PromptDailyGoalsConfig{},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}}
	service := NewService(chatRepoStub{}, repo, taskRepoStub{}, []int64{1}, 1, "UTC+3")

	text, err := service.Handle(context.Background(), 1, "/rules")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if text == "" || text[:6] != "rules:" {
		t.Fatalf("bad list text: %q", text)
	}
	if want := "12345678"; !strings.Contains(text, want) {
		t.Fatalf("expected short id %q in %q", want, text)
	}
	if want := "daily prompt"; !strings.Contains(text, want) {
		t.Fatalf("expected description %q in %q", want, text)
	}
}

func TestChatNotAllowed(t *testing.T) {
	t.Parallel()

	service := NewService(chatRepoStub{}, &ruleRepoStub{}, taskRepoStub{}, []int64{2}, 1, "UTC+3")
	text, err := service.Handle(context.Background(), 1, "/rules")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if text != "chat not allowed" {
		t.Fatalf("bad response: %q", text)
	}
}

func TestDueCommand(t *testing.T) {
	t.Parallel()

	service := NewService(chatRepoStub{}, &ruleRepoStub{}, taskRepoStub{due: []task.Snapshot{{Body: "Task A", SourcePath: "Daily/2026-03-23.md", DueDate: ptrDate("2026-03-23")}}}, []int64{1}, 1, "UTC+3")
	text, err := service.Handle(context.Background(), 1, "/due daily 2026-03-23")
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !strings.Contains(text, "due daily 2026-03-23") {
		t.Fatalf("bad due response: %q", text)
	}
	if !strings.Contains(text, "Task A") {
		t.Fatalf("bad due response: %q", text)
	}
}

func TestParseDueRangeDefaultsToLast14Days(t *testing.T) {
	t.Parallel()

	from, to, err := parseDueRange(nil, "UTC+3")
	if err != nil {
		t.Fatalf("parse range: %v", err)
	}
	if from == "" || to == "" {
		t.Fatalf("expected range, got from=%q to=%q", from, to)
	}
	loc, err := config.LoadLocation("UTC+3")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	fromTime := from.Time(loc)
	toTime := to.Time(loc)
	if int(toTime.Sub(fromTime).Hours()/24) != 13 {
		t.Fatalf("expected 14-day inclusive range, got from=%s to=%s", from, to)
	}
}

func ptrDate(raw string) *task.Date {
	date := task.MustParseDate(raw)
	return &date
}
