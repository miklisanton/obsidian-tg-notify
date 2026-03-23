package notification

import "obsidian-notify/internal/domain/reminder"

type Intent struct {
	DedupeKey string
	ChatID    int64
	RuleID    string
	Kind      reminder.RuleKind
	Text      string
	Payload   []byte
}
