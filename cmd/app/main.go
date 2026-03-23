package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"obsidian-notify/internal/adapter/config"
	"obsidian-notify/internal/adapter/fswatcher"
	"obsidian-notify/internal/adapter/obsidian"
	pg "obsidian-notify/internal/adapter/postgres"
	tgbot "obsidian-notify/internal/adapter/telegram"
	"obsidian-notify/internal/app/remind"
	"obsidian-notify/internal/app/syncer"
	telegramapp "obsidian-notify/internal/app/telegram"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	taskRepo := pg.NewTaskRepository(db)
	ruleRepo := pg.NewReminderRepository(db)
	notificationRepo := pg.NewNotificationRepository(db)
	chatRepo := pg.NewChatRepository(db)

	if err := vaultRepo.EnsureConfigured(ctx, cfg.Vaults); err != nil {
		log.Fatalf("seed vaults: %v", err)
	}

	classifier := obsidian.NewClassifier(cfg.App)
	parser := obsidian.NewParser(classifier)
	clock := remind.RealClock{}
	goals := remind.NewGoalsService(taskRepo, classifier)
	evaluator := remind.NewEvaluator(clock, taskRepo, ruleRepo, notificationRepo, goals)
	vaultRoots := make(map[int64]string, len(cfg.Vaults))
	for _, vault := range cfg.Vaults {
		vaultRoots[vault.ID] = vault.RootPath
	}
	evaluator.SetVaultRoots(vaultRoots)

	botGateway, err := tgbot.New(cfg.Telegram.Token)
	if err != nil {
		log.Fatalf("telegram bot: %v", err)
	}

	telegramService := telegramapp.NewService(chatRepo, ruleRepo, taskRepo, cfg.Telegram.AllowedChatIDs, cfg.App.DefaultVaultID, cfg.App.Timezone)
	syncService := syncer.NewService(taskRepo, ruleRepo, notificationRepo, botGateway, parser, cfg.App.TaskMatchThreshold)
	botRunner := tgbot.NewRunner(botGateway.Bot(), telegramService, syncService, evaluator, botGateway)

	debounceWindow, err := cfg.App.DebounceDuration()
	if err != nil {
		log.Fatalf("parse debounce window: %v", err)
	}
	watcher, err := fswatcher.New(cfg.Vaults, syncService, debounceWindow)
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}
	defer watcher.Close()

	go func() {
		if err := watcher.Start(ctx); err != nil {
			log.Printf("watcher stopped: %v", err)
			stop()
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := evaluator.RunDue(ctx); err != nil {
				log.Printf("evaluate reminders: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	if err := syncService.SyncAll(ctx, cfg.Vaults); err != nil {
		log.Printf("initial sync: %v", err)
	}

	if err := botRunner.Start(ctx); err != nil {
		log.Fatalf("bot runner: %v", err)
	}
}
