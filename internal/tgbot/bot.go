package tgbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/reputation"
)

type Bot struct {
	api *bot.Bot
	svc *reputation.Service
	ttl time.Duration
}

func New(token string, svc *reputation.Service, ttl time.Duration) (*Bot, error) {

	tb := &Bot{svc: svc, ttl: ttl}

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

	return tb, nil
}

func (tb *Bot) Run(ctx context.Context) error {

	tb.api.Start(ctx)
	return nil
}
