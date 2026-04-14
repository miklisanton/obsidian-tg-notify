package ports

import (
	"context"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/domain/message"
	"obsidian-notify/internal/domain/notification"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

type VaultRepository interface {
	EnsureConfigured(ctx context.Context, vaults []config.VaultConfig) error
	GetByID(ctx context.Context, id int64) (config.VaultConfig, error)
}

type TaskRepository interface {
	ListFileTasks(ctx context.Context, vaultID int64, sourcePath string) ([]task.Snapshot, error)
	ReplaceFile(ctx context.Context, document task.DocumentSnapshot, snapshots []task.Snapshot) error
	ListDueTasks(ctx context.Context, filter task.DueFilter) ([]task.Snapshot, error)
	ListDueTasksInRange(ctx context.Context, filter task.DueRangeFilter) ([]task.Snapshot, error)
	GetDocument(ctx context.Context, vaultID int64, sourcePath string) (task.DocumentSnapshot, bool, error)
	GetDailyDocument(ctx context.Context, vaultID int64, date task.Date) (task.DocumentSnapshot, bool, error)
	ListWeeklyDocuments(ctx context.Context, vaultID int64, year int, week int) ([]task.DocumentSnapshot, error)
	ListCurrentWeekUnfinished(ctx context.Context, vaultID int64, year int, week int) ([]task.Snapshot, error)
}

type ReminderRepository interface {
	ListActive(ctx context.Context) ([]reminder.Rule, error)
	ListByChat(ctx context.Context, chatID int64) ([]reminder.Rule, error)
	Insert(ctx context.Context, rule reminder.Rule) error
	Disable(ctx context.Context, ruleID string, chatID int64) error
}

type NotificationRepository interface {
	WasSent(ctx context.Context, dedupeKey string) (bool, error)
	RecordSent(ctx context.Context, intent notification.Intent, sentAt time.Time) error
}

type ChatRepository interface {
	Ensure(ctx context.Context, chatID int64) error
}

type MessageRepository interface {
	SaveIncomingText(ctx context.Context, item message.IncomingText) error
	ListIncomingTexts(ctx context.Context, chatID int64, from time.Time, to time.Time) ([]message.IncomingText, error)
}

type TelegramSender interface {
	Send(ctx context.Context, chatID int64, text string) error
}
