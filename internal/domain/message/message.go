package message

import "time"

type IncomingText struct {
	ChatID            int64
	TelegramMessageID int
	Text              string
	SentAt            time.Time
}
