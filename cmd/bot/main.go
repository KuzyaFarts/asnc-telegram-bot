package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/config"
	"github.com/KuzyaFarts/asnc-telegram-bot/internal/reputation"
	"github.com/KuzyaFarts/asnc-telegram-bot/internal/storage"
	"github.com/KuzyaFarts/asnc-telegram-bot/internal/tgbot"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := storage.NewSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer store.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := store.Init(ctx); err != nil {
		log.Fatalf("storage init: %v", err)
	}

	svc := reputation.New(store, cfg.Cooldown, cfg.MaxDelta)

	tb, err := tgbot.New(cfg.BotToken, svc, cfg.EphemeralTTL)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	log.Printf("bot: starting (db=%s, cooldown=%s, maxDelta=%d, ttl=%s)",
		cfg.DBPath, cfg.Cooldown, cfg.MaxDelta, cfg.EphemeralTTL)

	if err := tb.Run(ctx); err != nil {
		log.Fatalf("bot run: %v", err)
	}
	log.Printf("bot: shutdown complete")
}
