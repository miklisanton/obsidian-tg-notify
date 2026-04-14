package telegram

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"obsidian-notify/internal/app/remind"
	"obsidian-notify/internal/app/syncer"
	telegramapp "obsidian-notify/internal/app/telegram"
)

type Gateway struct {
	bot *bot.Bot
}

func New(token string) (*Gateway, error) {
	b, err := bot.New(token)
	if err != nil {
		return nil, err
	}
	return &Gateway{bot: b}, nil
}

func (g *Gateway) Bot() *bot.Bot {
	return g.bot
}

func (g *Gateway) Send(ctx context.Context, chatID int64, text string) error {
	_, err := g.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
	return err
}

type Runner struct {
	bot     *bot.Bot
	service *telegramapp.Service
	eval    *remind.Evaluator
	sender  *Gateway
}

func NewRunner(bot *bot.Bot, service *telegramapp.Service, syncService *syncer.Service, evaluator *remind.Evaluator, sender *Gateway) *Runner {
	evaluator.SetSender(sender)
	_ = syncService
	return &Runner{bot: bot, service: service, eval: evaluator, sender: sender}
}

func (r *Runner) Start(ctx context.Context) error {
	r.bot.RegisterHandler(bot.HandlerTypeMessageText, "/", bot.MatchTypePrefix, func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		text, err := r.service.Handle(ctx, update.Message.Chat.ID, update.Message.Text)
		if err != nil {
			text = err.Error()
		}
		if text == "" {
			return
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})
	})
	r.bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypeContains, func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		_ = r.service.TrackIncomingText(ctx, update.Message.Chat.ID, update.Message.ID, update.Message.Text, time.Unix(int64(update.Message.Date), 0).UTC())
	})
	r.bot.Start(ctx)
	return nil
}
