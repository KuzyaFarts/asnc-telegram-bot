package tgbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

type Bot struct {
	api     *bot.Bot
	svc     ports.ReputationUseCase
	economy ports.EconomyUseCase
	ttl     time.Duration
}

func New(token string, svc ports.ReputationUseCase, economy ports.EconomyUseCase, ttl time.Duration) (*Bot, error) {

	tb := &Bot{svc: svc, economy: economy, ttl: ttl}

	opts := []bot.Option{
		bot.WithDefaultHandler(tb.onMessage),
	}

	api, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("bot.New: %w", err)
	}
	tb.api = api

	api.RegisterHandler(bot.HandlerTypeMessageText, "rep", bot.MatchTypeCommandStartOnly, tb.onRep)
	api.RegisterHandler(bot.HandlerTypeMessageText, "top", bot.MatchTypeCommandStartOnly, tb.onTop)
	api.RegisterHandler(bot.HandlerTypeMessageText, "plus_rep", bot.MatchTypeCommandStartOnly, tb.onPlusRep)
	api.RegisterHandler(bot.HandlerTypeMessageText, "minus_rep", bot.MatchTypeCommandStartOnly, tb.onMinusRep)
	api.RegisterHandler(bot.HandlerTypeMessageText, "premiddle", bot.MatchTypeCommandStartOnly, tb.onPremiddle)
	api.RegisterHandler(bot.HandlerTypeMessageText, "balance", bot.MatchTypeCommandStartOnly, tb.onBalance)
	api.RegisterHandler(bot.HandlerTypeMessageText, "profile", bot.MatchTypeCommandStartOnly, tb.onProfile)
	api.RegisterHandler(bot.HandlerTypeMessageText, "me", bot.MatchTypeCommandStartOnly, tb.onProfile)
	api.RegisterHandler(bot.HandlerTypeMessageText, "daily", bot.MatchTypeCommandStartOnly, tb.onDaily)
	api.RegisterHandler(bot.HandlerTypeMessageText, "roll", bot.MatchTypeCommandStartOnly, tb.onRoll)
	api.RegisterHandler(bot.HandlerTypeMessageText, "dashboard", bot.MatchTypeCommandStartOnly, tb.onDashboard)
	api.RegisterHandler(bot.HandlerTypeMessageText, "duel", bot.MatchTypeCommandStartOnly, tb.onDuel)
	api.RegisterHandler(bot.HandlerTypeMessageText, "accept", bot.MatchTypeCommandStartOnly, tb.onAcceptDuel)
	api.RegisterHandler(bot.HandlerTypeMessageText, "cancel_duel", bot.MatchTypeCommandStartOnly, tb.onCancelDuel)
	api.RegisterHandler(bot.HandlerTypeMessageText, "mute_duel", bot.MatchTypeCommandStartOnly, tb.onMuteDuel)
	api.RegisterHandler(bot.HandlerTypeMessageText, "accept_mute", bot.MatchTypeCommandStartOnly, tb.onAcceptMuteDuel)
	api.RegisterHandler(bot.HandlerTypeMessageText, "cancel_mute_duel", bot.MatchTypeCommandStartOnly, tb.onCancelMuteDuel)
	api.RegisterHandler(bot.HandlerTypeMessageText, "help", bot.MatchTypeCommandStartOnly, tb.onHelp)
	api.RegisterHandler(bot.HandlerTypeMessageText, "games", bot.MatchTypeCommandStartOnly, tb.onHelp)

	return tb, nil
}

func (tb *Bot) Run(ctx context.Context) error {

	tb.api.Start(ctx)
	return nil
}
