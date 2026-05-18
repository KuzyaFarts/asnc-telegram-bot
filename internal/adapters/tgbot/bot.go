package tgbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

type Bot struct {
	api       *bot.Bot
	svc       ports.ReputationUseCase
	economy   ports.EconomyUseCase
	msgStore  ports.MessageStore
	openAIKey string
	ttl       time.Duration
}

func New(token string, svc ports.ReputationUseCase, economy ports.EconomyUseCase, msgStore ports.MessageStore, openAIKey string, ttl time.Duration) (*Bot, error) {

	tb := &Bot{svc: svc, economy: economy, msgStore: msgStore, openAIKey: openAIKey, ttl: ttl}

	opts := []bot.Option{
		bot.WithDefaultHandler(tb.onMessage),
	}

	api, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}
	tb.api = api

	api.RegisterHandler(bot.HandlerTypeMessageText, "rep", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onRep))
	api.RegisterHandler(bot.HandlerTypeMessageText, "top", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onTop))
	api.RegisterHandler(bot.HandlerTypeMessageText, "plus_rep", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onPlusRep))
	api.RegisterHandler(bot.HandlerTypeMessageText, "minus_rep", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onMinusRep))
	api.RegisterHandler(bot.HandlerTypeMessageText, "premiddle", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onPremiddle))
	api.RegisterHandler(bot.HandlerTypeMessageText, "balance", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onBalance))
	api.RegisterHandler(bot.HandlerTypeMessageText, "profile", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onProfile))
	api.RegisterHandler(bot.HandlerTypeMessageText, "me", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onProfile))
	api.RegisterHandler(bot.HandlerTypeMessageText, "daily", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onDaily))
	api.RegisterHandler(bot.HandlerTypeMessageText, "roll", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onRoll))
	api.RegisterHandler(bot.HandlerTypeMessageText, "dashboard", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onDashboard))
	api.RegisterHandler(bot.HandlerTypeMessageText, "duel", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onDuel))
	api.RegisterHandler(bot.HandlerTypeMessageText, "accept", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onAcceptDuel))
	api.RegisterHandler(bot.HandlerTypeMessageText, "cancel_duel", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onCancelDuel))
	api.RegisterHandler(bot.HandlerTypeMessageText, "mute_duel", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onMuteDuel))
	api.RegisterHandler(bot.HandlerTypeMessageText, "accept_mute", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onAcceptMuteDuel))
	api.RegisterHandler(bot.HandlerTypeMessageText, "cancel_mute_duel", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onCancelMuteDuel))
	api.RegisterHandler(bot.HandlerTypeMessageText, "help", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onHelp))
	api.RegisterHandler(bot.HandlerTypeMessageText, "games", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onHelp))
	api.RegisterHandler(bot.HandlerTypeMessageText, "summary", bot.MatchTypeCommandStartOnly, withCommandCleanup(tb.onSummary))

	return tb, nil
}

func (tb *Bot) Run(ctx context.Context) error {

	tb.api.Start(ctx)
	return nil
}

func withCommandCleanup(h bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, u *models.Update) {
		h(ctx, b, u)
		if u.Message == nil {
			return
		}
		chatID := u.Message.Chat.ID
		msgID := u.Message.ID
		time.AfterFunc(10*time.Second, func() {
			delCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			deleteCommandMessage(delCtx, b, chatID, msgID)
		})
	}
}
