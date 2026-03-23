package syncer

import (
	"context"
	"testing"
	"time"

	"obsidian-notify/internal/domain/notification"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type reminderRepoStub struct {
	rules []reminder.Rule
}

func (r reminderRepoStub) ListActive(context.Context) ([]reminder.Rule, error) { return r.rules, nil }
func (r reminderRepoStub) ListByChat(context.Context, int64) ([]reminder.Rule, error) {
	return nil, nil
}
func (r reminderRepoStub) Insert(context.Context, reminder.Rule) error  { return nil }
func (r reminderRepoStub) Disable(context.Context, string, int64) error { return nil }

type notificationRepoStub struct {
	recorded []notification.Intent
}

func (r *notificationRepoStub) WasSent(context.Context, string) (bool, error) { return false, nil }
func (r *notificationRepoStub) RecordSent(_ context.Context, intent notification.Intent, _ time.Time) error {
	r.recorded = append(r.recorded, intent)
	return nil
}

type senderStub struct {
	texts []string
}

func (s *senderStub) Send(_ context.Context, _ int64, text string) error {
	s.texts = append(s.texts, text)
	return nil
}

func TestNotifyNewTasksIgnoresSameFingerprint(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	previous := []task.Snapshot{{Fingerprint: "same", LineNumber: 1, Body: "Task A"}}
	current := []task.Snapshot{{Fingerprint: "same", LineNumber: 1, Body: "Task A"}}

	if err := service.notifyNewTasks(context.Background(), 1, previous, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(sender.texts))
	}
}

func TestNotifyNewTasksSendsOnNewFingerprint(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	previous := []task.Snapshot{{Fingerprint: "old", LineNumber: 1, Body: "Task A"}}
	current := []task.Snapshot{{Fingerprint: "new", LineNumber: 1, Body: "Task A edited"}}

	if err := service.notifyNewTasks(context.Background(), 1, previous, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(sender.texts))
	}
}

func TestNotifyNewTasksSuppressesSimilarMovedEdit(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	previous := []task.Snapshot{{Fingerprint: "old", LineNumber: 1, Body: "Task A"}}
	current := []task.Snapshot{{Fingerprint: "new", LineNumber: 2, Body: "Task A edited", SourcePath: "Daily/2026-03-21.md"}}

	if err := service.notifyNewTasks(context.Background(), 1, previous, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(sender.texts))
	}
}

func TestNotifyNewTasksSendsWhenSamePositionButDifferentText(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	previous := []task.Snapshot{{Fingerprint: "old", LineNumber: 1, Section: "TODO", Body: "Buy milk"}}
	current := []task.Snapshot{{Fingerprint: "new", LineNumber: 1, Section: "TODO", Body: "Prepare tax report", SourcePath: "Daily/2026-03-21.md"}}

	if err := service.notifyNewTasks(context.Background(), 1, previous, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sender.texts))
	}
}

func TestNotifyNewTasksBatchesByFileSync(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	current := []task.Snapshot{{Fingerprint: "new-111111111111", SourcePath: "Daily/2026-03-21.md", LineNumber: 1, Body: "Task A"}, {Fingerprint: "new-222222222222", SourcePath: "Daily/2026-03-21.md", LineNumber: 2, Body: "Task B"}}

	if err := service.notifyNewTasks(context.Background(), 1, nil, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sender.texts))
	}
	if sender.texts[0] != "2 new task(s) in Daily/2026-03-21.md:\n- Task A\n- Task B" {
		t.Fatalf("bad notification text: %q", sender.texts[0])
	}
}

func TestNotifyNewTasksWithInsertedTaskAbove(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	previous := []task.Snapshot{
		{Fingerprint: "old-a", SourcePath: "Daily/2026-03-21.md", Section: "TODO", LineNumber: 1, Body: "Existing task"},
		{Fingerprint: "old-b", SourcePath: "Daily/2026-03-21.md", Section: "TODO", LineNumber: 2, Body: "Another task"},
	}
	current := []task.Snapshot{
		{Fingerprint: "new-a", SourcePath: "Daily/2026-03-21.md", Section: "TODO", LineNumber: 1, Body: "Brand new task"},
		{Fingerprint: "curr-a", SourcePath: "Daily/2026-03-21.md", Section: "TODO", LineNumber: 2, Body: "Existing task"},
		{Fingerprint: "curr-b", SourcePath: "Daily/2026-03-21.md", Section: "TODO", LineNumber: 3, Body: "Another task"},
	}

	if err := service.notifyNewTasks(context.Background(), 1, previous, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sender.texts))
	}
	if sender.texts[0] != "1 new task(s) in Daily/2026-03-21.md:\n- Brand new task" {
		t.Fatalf("bad notification text: %q", sender.texts[0])
	}
}

func TestNotifyNewTasksSkipsCompleted(t *testing.T) {
	t.Parallel()

	notifications := &notificationRepoStub{}
	sender := &senderStub{}
	service := &Service{
		rules: reminderRepoStub{rules: []reminder.Rule{{
			ID:      "rule-1",
			ChatID:  1,
			VaultID: 1,
			Kind:    reminder.RuleKindNewTask,
			Enabled: true,
		}}},
		notifications: notifications,
		sender:        sender,
	}

	current := []task.Snapshot{{Fingerprint: "done", LineNumber: 1, Body: "Task A", Completed: true}}

	if err := service.notifyNewTasks(context.Background(), 1, nil, current); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.texts) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(sender.texts))
	}
}
