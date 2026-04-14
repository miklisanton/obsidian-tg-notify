package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/app/ports"
	"obsidian-notify/internal/domain/message"
	"obsidian-notify/internal/domain/notification"
	"obsidian-notify/internal/domain/reminder"
	"obsidian-notify/internal/domain/task"
)

var _ ports.VaultRepository = (*VaultRepository)(nil)
var _ ports.TaskRepository = (*TaskRepository)(nil)
var _ ports.ReminderRepository = (*ReminderRepository)(nil)
var _ ports.NotificationRepository = (*NotificationRepository)(nil)
var _ ports.ChatRepository = (*ChatRepository)(nil)
var _ ports.MessageRepository = (*MessageRepository)(nil)

type VaultRepository struct{ db *DB }
type TaskRepository struct{ db *DB }
type ReminderRepository struct{ db *DB }
type NotificationRepository struct{ db *DB }
type ChatRepository struct{ db *DB }
type MessageRepository struct{ db *DB }

func NewVaultRepository(db *DB) *VaultRepository       { return &VaultRepository{db: db} }
func NewTaskRepository(db *DB) *TaskRepository         { return &TaskRepository{db: db} }
func NewReminderRepository(db *DB) *ReminderRepository { return &ReminderRepository{db: db} }
func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}
func NewChatRepository(db *DB) *ChatRepository       { return &ChatRepository{db: db} }
func NewMessageRepository(db *DB) *MessageRepository { return &MessageRepository{db: db} }

func (r *VaultRepository) EnsureConfigured(ctx context.Context, vaults []config.VaultConfig) error {
	for _, vault := range vaults {
		_, err := r.db.ExecContext(ctx, `
			insert into vaults (id, name, root_path, timezone)
			values ($1, $2, $3, $4)
			on conflict (id) do update set name = excluded.name, root_path = excluded.root_path
		`, vault.ID, vault.Name, vault.RootPath, vault.Timezone)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *VaultRepository) GetByID(ctx context.Context, id int64) (config.VaultConfig, error) {
	var vault config.VaultConfig
	err := r.db.GetContext(ctx, &vault, `select id, name, root_path from vaults where id = $1`, id)
	return vault, err
}

func (r *TaskRepository) ListFileTasks(ctx context.Context, vaultID int64, sourcePath string) ([]task.Snapshot, error) {
	rows := []taskRow{}
	err := r.db.SelectContext(ctx, &rows, `
		select vault_id, source_path, source_kind, line_number, section, fingerprint, body, completed, due_date, done_date, daily_date, iso_week_year, iso_week_number, weekly_area, seen_at, updated_at
		from task_snapshots where vault_id = $1 and source_path = $2
	`, vaultID, sourcePath)
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func (r *TaskRepository) ReplaceFile(ctx context.Context, document task.DocumentSnapshot, snapshots []task.Snapshot) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `delete from task_snapshots where vault_id = $1 and source_path = $2`, document.VaultID, document.SourcePath); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from document_snapshots where vault_id = $1 and source_path = $2`, document.VaultID, document.SourcePath); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		insert into document_snapshots (vault_id, source_path, source_kind, daily_date, iso_week_year, iso_week_number, weekly_area, has_non_blank_task, has_daily_summary, synced_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, document.VaultID, document.SourcePath, document.SourceKind, nullDate(document.DailyDate), document.ISOWeekYear, document.ISOWeekNumber, document.WeeklyArea, document.HasNonBlankTask, document.HasDailySummary, document.SyncedAt); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if _, err := tx.ExecContext(ctx, `
			insert into task_snapshots (vault_id, source_path, source_kind, line_number, section, fingerprint, body, completed, due_date, done_date, daily_date, iso_week_year, iso_week_number, weekly_area, seen_at, updated_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, snapshot.VaultID, snapshot.SourcePath, snapshot.SourceKind, snapshot.LineNumber, snapshot.Section, snapshot.Fingerprint, snapshot.Body, snapshot.Completed, nullDate(snapshot.DueDate), nullDate(snapshot.DoneDate), nullDate(snapshot.DailyDate), snapshot.ISOWeekYear, snapshot.ISOWeekNumber, snapshot.WeeklyArea, snapshot.SeenAt, snapshot.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *TaskRepository) ListDueTasks(ctx context.Context, filter task.DueFilter) ([]task.Snapshot, error) {
	var target string
	today := task.Today(filter.Now, filter.Loc)
	switch filter.Window {
	case task.DueWindowToday:
		target = string(today)
	case task.DueWindowTomorrow:
		target = string(task.Today(filter.Now.Add(24*time.Hour), filter.Loc))
	case task.DueWindowOverdue:
		rows := []taskRow{}
		err := r.db.SelectContext(ctx, &rows, `
			select vault_id, source_path, source_kind, line_number, section, fingerprint, body, completed, due_date, done_date, daily_date, iso_week_year, iso_week_number, weekly_area, seen_at, updated_at
			from task_snapshots where vault_id = $1 and completed = false and due_date < $2
			order by due_date asc, source_path asc, line_number asc
		`, filter.VaultID, string(today))
		if err != nil {
			return nil, err
		}
		return mapTasks(rows), nil
	default:
		return nil, fmt.Errorf("bad due window %q", filter.Window)
	}
	rows := []taskRow{}
	err := r.db.SelectContext(ctx, &rows, `
		select vault_id, source_path, source_kind, line_number, section, fingerprint, body, completed, due_date, done_date, daily_date, iso_week_year, iso_week_number, weekly_area, seen_at, updated_at
		from task_snapshots where vault_id = $1 and completed = false and due_date = $2
		order by source_path asc, line_number asc
	`, filter.VaultID, target)
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func (r *TaskRepository) ListDueTasksInRange(ctx context.Context, filter task.DueRangeFilter) ([]task.Snapshot, error) {
	query := `
		select vault_id, source_path, source_kind, line_number, section, fingerprint, body, completed,
		       coalesce(
		           due_date,
		           case when source_kind = 'daily' then daily_date end,
		           case
		               when source_kind = 'weekly' and iso_week_year is not null and iso_week_number is not null
		               then (to_date(iso_week_year::text || '-' || lpad(iso_week_number::text, 2, '0') || '-1', 'IYYY-IW-ID') + interval '6 days')::date
		           end
		       ) as due_date,
		       done_date, daily_date, iso_week_year, iso_week_number, weekly_area, seen_at, updated_at
		from task_snapshots
		where vault_id = $1 and completed = false and coalesce(
		           due_date,
		           case when source_kind = 'daily' then daily_date end,
		           case
		               when source_kind = 'weekly' and iso_week_year is not null and iso_week_number is not null
		               then (to_date(iso_week_year::text || '-' || lpad(iso_week_number::text, 2, '0') || '-1', 'IYYY-IW-ID') + interval '6 days')::date
		           end
		       ) is not null
		  and coalesce(
		           due_date,
		           case when source_kind = 'daily' then daily_date end,
		           case
		               when source_kind = 'weekly' and iso_week_year is not null and iso_week_number is not null
		               then (to_date(iso_week_year::text || '-' || lpad(iso_week_number::text, 2, '0') || '-1', 'IYYY-IW-ID') + interval '6 days')::date
		           end
		       ) >= $2
		  and coalesce(
		           due_date,
		           case when source_kind = 'daily' then daily_date end,
		           case
		               when source_kind = 'weekly' and iso_week_year is not null and iso_week_number is not null
		               then (to_date(iso_week_year::text || '-' || lpad(iso_week_number::text, 2, '0') || '-1', 'IYYY-IW-ID') + interval '6 days')::date
		           end
		       ) <= $3
	`
	args := []any{filter.VaultID, string(filter.From), string(filter.To)}
	switch filter.Scope {
	case task.DueScopeAll:
		query += ` order by due_date asc, source_path asc, line_number asc`
	case task.DueScopeDaily:
		query += ` and source_kind = 'daily' order by due_date asc, source_path asc, line_number asc`
	case task.DueScopeWeekly:
		query += ` and source_kind = 'weekly' order by due_date asc, source_path asc, line_number asc`
	default:
		return nil, fmt.Errorf("bad due scope %q", filter.Scope)
	}
	rows := []taskRow{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func (r *TaskRepository) GetDocument(ctx context.Context, vaultID int64, sourcePath string) (task.DocumentSnapshot, bool, error) {
	var row documentRow
	err := r.db.GetContext(ctx, &row, `select vault_id, source_path, source_kind, daily_date, iso_week_year, iso_week_number, weekly_area, has_non_blank_task, has_daily_summary, synced_at from document_snapshots where vault_id = $1 and source_path = $2`, vaultID, sourcePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return task.DocumentSnapshot{}, false, nil
		}
		return task.DocumentSnapshot{}, false, err
	}
	return row.toDomain(), true, nil
}

func (r *TaskRepository) GetDailyDocument(ctx context.Context, vaultID int64, date task.Date) (task.DocumentSnapshot, bool, error) {
	var row documentRow
	err := r.db.GetContext(ctx, &row, `
		select vault_id, source_path, source_kind, daily_date, iso_week_year, iso_week_number, weekly_area, has_non_blank_task, has_daily_summary, synced_at
		from document_snapshots where vault_id = $1 and daily_date = $2 limit 1
	`, vaultID, string(date))
	if err != nil {
		if err == sql.ErrNoRows {
			return task.DocumentSnapshot{}, false, nil
		}
		return task.DocumentSnapshot{}, false, err
	}
	return row.toDomain(), true, nil
}

func (r *TaskRepository) ListWeeklyDocuments(ctx context.Context, vaultID int64, year int, week int) ([]task.DocumentSnapshot, error) {
	rows := []documentRow{}
	err := r.db.SelectContext(ctx, &rows, `
		select vault_id, source_path, source_kind, daily_date, iso_week_year, iso_week_number, weekly_area, has_non_blank_task, has_daily_summary, synced_at
		from document_snapshots
		where vault_id = $1 and source_kind = 'weekly' and iso_week_year = $2 and iso_week_number = $3
	`, vaultID, year, week)
	if err != nil {
		return nil, err
	}
	items := make([]task.DocumentSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain())
	}
	return items, nil
}

func (r *TaskRepository) ListCurrentWeekUnfinished(ctx context.Context, vaultID int64, year int, week int) ([]task.Snapshot, error) {
	rows := []taskRow{}
	err := r.db.SelectContext(ctx, &rows, `
		select vault_id, source_path, source_kind, line_number, section, fingerprint, body, completed, due_date, done_date, daily_date, iso_week_year, iso_week_number, weekly_area, seen_at, updated_at
		from task_snapshots
		where vault_id = $1 and source_kind = 'weekly' and iso_week_year = $2 and iso_week_number = $3 and completed = false
		order by weekly_area asc, source_path asc, line_number asc
	`, vaultID, year, week)
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func (r *ReminderRepository) ListActive(ctx context.Context) ([]reminder.Rule, error) {
	return r.list(ctx, `select id, chat_id, vault_id, kind, timezone, schedule_json, config_json, enabled, created_at, updated_at from reminder_rules where enabled = true`)
}

func (r *ReminderRepository) ListByChat(ctx context.Context, chatID int64) ([]reminder.Rule, error) {
	return r.list(ctx, `select id, chat_id, vault_id, kind, timezone, schedule_json, config_json, enabled, created_at, updated_at from reminder_rules where chat_id = $1 order by created_at desc`, chatID)
}

func (r *ReminderRepository) list(ctx context.Context, query string, args ...any) ([]reminder.Rule, error) {
	rows := []ruleRow{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	items := make([]reminder.Rule, 0, len(rows))
	for _, row := range rows {
		schedule := reminder.Schedule{}
		if err := json.Unmarshal(row.ScheduleJSON, &schedule); err != nil {
			return nil, err
		}
		cfg, err := reminder.UnmarshalConfig(reminder.RuleKind(row.Kind), row.ConfigJSON)
		if err != nil {
			return nil, err
		}
		items = append(items, reminder.Rule{ID: row.ID, ChatID: row.ChatID, VaultID: row.VaultID, Kind: reminder.RuleKind(row.Kind), Timezone: row.Timezone, Schedule: schedule, Config: cfg, Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return items, nil
}

func (r *ReminderRepository) Insert(ctx context.Context, rule reminder.Rule) error {
	scheduleJSON, err := json.Marshal(rule.Schedule)
	if err != nil {
		return err
	}
	configJSON, err := reminder.MarshalConfig(rule.Config)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		insert into reminder_rules (id, chat_id, vault_id, kind, timezone, schedule_json, config_json, enabled, created_at, updated_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, rule.ID, rule.ChatID, rule.VaultID, rule.Kind, rule.Timezone, scheduleJSON, configJSON, rule.Enabled, rule.CreatedAt, rule.UpdatedAt)
	return err
}

func (r *ReminderRepository) Disable(ctx context.Context, ruleID string, chatID int64) error {
	_, err := r.db.ExecContext(ctx, `delete from reminder_rules where id = $1 and chat_id = $2`, ruleID, chatID)
	return err
}

func (r *NotificationRepository) WasSent(ctx context.Context, dedupeKey string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `select exists(select 1 from sent_notifications where dedupe_key = $1)`, dedupeKey)
	return exists, err
}

func (r *NotificationRepository) RecordSent(ctx context.Context, intent notification.Intent, sentAt time.Time) error {
	payload := intent.Payload
	if payload == nil {
		payload = []byte(`{}`)
	}
	_, err := r.db.ExecContext(ctx, `
		insert into sent_notifications (dedupe_key, chat_id, rule_id, kind, payload, sent_at)
		values ($1, $2, $3, $4, $5, $6)
	`, intent.DedupeKey, intent.ChatID, nullableString(intent.RuleID), intent.Kind, payload, sentAt)
	return err
}

func (r *ChatRepository) Ensure(ctx context.Context, chatID int64) error {
	_, err := r.db.ExecContext(ctx, `insert into telegram_chats (chat_id) values ($1) on conflict (chat_id) do nothing`, chatID)
	return err
}

func (r *MessageRepository) SaveIncomingText(ctx context.Context, item message.IncomingText) error {
	_, err := r.db.ExecContext(ctx, `
		insert into telegram_messages (chat_id, telegram_message_id, text, sent_at)
		values ($1, $2, $3, $4)
		on conflict (chat_id, telegram_message_id) do nothing
	`, item.ChatID, item.TelegramMessageID, item.Text, item.SentAt)
	return err
}

func (r *MessageRepository) ListIncomingTexts(ctx context.Context, chatID int64, from time.Time, to time.Time) ([]message.IncomingText, error) {
	rows := []messageRow{}
	err := r.db.SelectContext(ctx, &rows, `
		select chat_id, telegram_message_id, text, sent_at
		from telegram_messages
		where chat_id = $1 and sent_at >= $2 and sent_at < $3
		order by sent_at asc, telegram_message_id asc
	`, chatID, from, to)
	if err != nil {
		return nil, err
	}
	items := make([]message.IncomingText, 0, len(rows))
	for _, row := range rows {
		items = append(items, message.IncomingText{ChatID: row.ChatID, TelegramMessageID: row.TelegramMessageID, Text: row.Text, SentAt: row.SentAt})
	}
	return items, nil
}

type taskRow struct {
	VaultID       int64          `db:"vault_id"`
	SourcePath    string         `db:"source_path"`
	SourceKind    string         `db:"source_kind"`
	LineNumber    int            `db:"line_number"`
	Section       string         `db:"section"`
	Fingerprint   string         `db:"fingerprint"`
	Body          string         `db:"body"`
	Completed     bool           `db:"completed"`
	DueDate       sql.NullString `db:"due_date"`
	DoneDate      sql.NullString `db:"done_date"`
	DailyDate     sql.NullString `db:"daily_date"`
	ISOWeekYear   sql.NullInt64  `db:"iso_week_year"`
	ISOWeekNumber sql.NullInt64  `db:"iso_week_number"`
	WeeklyArea    sql.NullString `db:"weekly_area"`
	SeenAt        time.Time      `db:"seen_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

type documentRow struct {
	VaultID         int64          `db:"vault_id"`
	SourcePath      string         `db:"source_path"`
	SourceKind      string         `db:"source_kind"`
	DailyDate       sql.NullString `db:"daily_date"`
	ISOWeekYear     sql.NullInt64  `db:"iso_week_year"`
	ISOWeekNumber   sql.NullInt64  `db:"iso_week_number"`
	WeeklyArea      sql.NullString `db:"weekly_area"`
	HasNonBlankTask bool           `db:"has_non_blank_task"`
	HasDailySummary bool           `db:"has_daily_summary"`
	SyncedAt        time.Time      `db:"synced_at"`
}

type messageRow struct {
	ChatID            int64     `db:"chat_id"`
	TelegramMessageID int       `db:"telegram_message_id"`
	Text              string    `db:"text"`
	SentAt            time.Time `db:"sent_at"`
}

type ruleRow struct {
	ID           string          `db:"id"`
	ChatID       int64           `db:"chat_id"`
	VaultID      int64           `db:"vault_id"`
	Kind         string          `db:"kind"`
	Timezone     string          `db:"timezone"`
	ScheduleJSON json.RawMessage `db:"schedule_json"`
	ConfigJSON   json.RawMessage `db:"config_json"`
	Enabled      bool            `db:"enabled"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

func mapTasks(rows []taskRow) []task.Snapshot {
	items := make([]task.Snapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, task.Snapshot{VaultID: row.VaultID, SourcePath: row.SourcePath, SourceKind: task.SourceKind(row.SourceKind), LineNumber: row.LineNumber, Section: row.Section, Fingerprint: row.Fingerprint, Body: row.Body, Completed: row.Completed, DueDate: nullStringDate(row.DueDate), DoneDate: nullStringDate(row.DoneDate), DailyDate: nullStringDate(row.DailyDate), ISOWeekYear: nullInt(row.ISOWeekYear), ISOWeekNumber: nullInt(row.ISOWeekNumber), WeeklyArea: nullString(row.WeeklyArea), SeenAt: row.SeenAt, UpdatedAt: row.UpdatedAt})
	}
	return items
}

func (r documentRow) toDomain() task.DocumentSnapshot {
	return task.DocumentSnapshot{VaultID: r.VaultID, SourcePath: r.SourcePath, SourceKind: task.SourceKind(r.SourceKind), DailyDate: nullStringDate(r.DailyDate), ISOWeekYear: nullInt(r.ISOWeekYear), ISOWeekNumber: nullInt(r.ISOWeekNumber), WeeklyArea: nullString(r.WeeklyArea), HasNonBlankTask: r.HasNonBlankTask, HasDailySummary: r.HasDailySummary, SyncedAt: r.SyncedAt}
}

func nullDate(date *task.Date) any {
	if date == nil {
		return nil
	}
	return string(*date)
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	item := int(value.Int64)
	return &item
}

func nullableString(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

func nullStringDate(value sql.NullString) *task.Date {
	if !value.Valid {
		return nil
	}
	date := task.Date(value.String)
	return &date
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	item := value.String
	return &item
}
