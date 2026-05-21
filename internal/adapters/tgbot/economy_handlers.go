package tgbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/economy"
	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

func (tb *Bot) onProfile(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	target := msg.From
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		target = msg.ReplyToMessage.From
	}

	profile, err := tb.economy.Profile(ctx, msg.Chat.ID, target.ID)
	if err != nil {
		log.Printf("onProfile: Profile: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, profileHTML(target, profile), tb.ttl)
}

func (tb *Bot) onBalance(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	target := msg.From
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		target = msg.ReplyToMessage.From
	}
	profile, err := tb.economy.Profile(ctx, msg.Chat.ID, target.ID)
	if err != nil {
		log.Printf("onBalance: Profile: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("🏦 %s: <b>%d</b> ASNC-coin", mentionHTML(target), profile.Account.Balance),
		tb.ttl)
}

func (tb *Bot) onDaily(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	res, err := tb.economy.ClaimDaily(ctx, msg.Chat.ID, knownFromUser(msg.From))
	if errors.Is(err, economy.ErrDailyAlreadyClaimed) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			fmt.Sprintf("⏳ Daily уже забран. Следующая награда через <b>%s</b>.", html.EscapeString(humanDuration(res.NextClaimIn))),
			tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onDaily: ClaimDaily: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("🎁 %s получил <b>%d</b> ASNC-coin\n└ streak: <b>%d</b>, баланс: <b>%d</b>",
			mentionHTML(msg.From), res.Reward, res.Streak, res.Account.Balance),
		tb.ttl)
}

func (tb *Bot) onRoll(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	args, err := commandArgs(msg.Text)
	if err != nil || len(args) != 1 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Использование: <code>/roll 25</code>", tb.ttl)
		return
	}
	bet, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Ставка должна быть числом.", tb.ttl)
		return
	}
	res, err := tb.economy.Roll(ctx, msg.Chat.ID, knownFromUser(msg.From), bet)
	if errors.Is(err, economy.ErrInvalidAmount) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Ставка должна быть больше нуля.", tb.ttl)
		return
	}
	if errors.Is(err, economy.ErrInsufficientBalance) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			fmt.Sprintf("🏦 Не хватает ASNC-coin: ставка <b>%d</b>, баланс <b>%d</b>.", bet, res.Account.Balance),
			tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onRoll: Roll: %v", err)
		return
	}

	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, rollHTML(msg.From, res), tb.ttl)
}

func (tb *Bot) onDashboard(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) {
		return
	}
	tb.rememberParticipants(ctx, msg)

	repTop, err := tb.reputationTopHTML(ctx, msg.Chat.ID, 5)
	if err != nil {
		log.Printf("onDashboard: reputationTopHTML: %v", err)
		return
	}
	dashboard, err := tb.economy.Dashboard(ctx, msg.Chat.ID, 5)
	if err != nil {
		log.Printf("onDashboard: Dashboard: %v", err)
		return
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>Дашборд чата</b>\n\n")
	sb.WriteString(repTop)
	sb.WriteString("\n")
	if len(dashboard.TopBalances) == 0 {
		sb.WriteString("🏦 <b>Банк ASNC-coin</b>\n📭 Пока пусто. Начни с <code>/daily</code>.\n")
	} else {
		sb.WriteString(coinTopHTML(dashboard, "🏦 <b>Топ ASNC-coin</b>", true))
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, sb.String(), tb.ttl)
}

func (tb *Bot) sendCoinTop(ctx context.Context, b *bot.Bot, msg *models.Message) {
	dashboard, err := tb.economy.Dashboard(ctx, msg.Chat.ID, 10)
	if err != nil {
		log.Printf("sendCoinTop: Dashboard: %v", err)
		return
	}
	if len(dashboard.TopBalances) == 0 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "🏦 Банк ASNC-coin пока пуст. Начни с <code>/daily</code>.", tb.ttl)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, coinTopHTML(dashboard, "🏦 <b>Топ ASNC-coin</b>", true), tb.ttl)
}

func (tb *Bot) onDuel(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	target, stake, err := tb.parseDuelTarget(ctx, b, msg)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "⚔️ "+html.EscapeString(err.Error()), tb.ttl)
		return
	}
	res, err := tb.economy.CreateDuel(ctx, msg.Chat.ID, knownFromUser(msg.From), knownFromUser(target), stake)
	if errors.Is(err, economy.ErrSelfDuel) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Нельзя вызвать на дуэль самого себя.", tb.ttl)
		return
	}
	if errors.Is(err, economy.ErrInsufficientBalance) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "🏦 Не хватает ASNC-coin для такой ставки.", tb.ttl)
		return
	}
	if errors.Is(err, economy.ErrInvalidAmount) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Ставка должна быть больше нуля.", tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onDuel: CreateDuel: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("⚔️ %s вызывает %s на дуэль за <b>%d</b> ASNC-coin\n└ принять: <code>/accept %d</code>",
			mentionHTML(msg.From), mentionHTML(target), res.Duel.Stake, msg.From.ID),
		tb.ttl)
}

func (tb *Bot) onAcceptDuel(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	challenger, err := tb.parseAcceptTarget(ctx, b, msg)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "⚔️ "+html.EscapeString(err.Error()), tb.ttl)
		return
	}
	res, err := tb.economy.AcceptDuel(ctx, msg.Chat.ID, knownFromUser(msg.From), challenger.ID)
	if errors.Is(err, economy.ErrPendingDuelNotFound) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Активной дуэли от этого игрока не найдено.", tb.ttl)
		return
	}
	if errors.Is(err, economy.ErrInsufficientBalance) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "🏦 У одного из участников уже не хватает ASNC-coin.", tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onAcceptDuel: AcceptDuel: %v", err)
		return
	}

	winner := msg.From
	if res.WinnerID == challenger.ID {
		winner = challenger
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("⚔️ Дуэль завершена\n└ победитель: %s, выигрыш: <b>%d</b> ASNC-coin",
			mentionHTML(winner), res.Duel.Stake),
		20*time.Second)
}

func (tb *Bot) onCancelDuel(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	opponent, err := tb.parseAcceptTarget(ctx, b, msg)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "⚔️ "+html.EscapeString(err.Error()), tb.ttl)
		return
	}
	res, err := tb.economy.CancelDuel(ctx, msg.Chat.ID, knownFromUser(msg.From), opponent.ID)
	if errors.Is(err, economy.ErrPendingDuelNotFound) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Активной дуэли с этим игроком не найдено.", tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onCancelDuel: CancelDuel: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("⚔️ Дуэль с %s отменена\n└ ставка была <b>%d</b> ASNC-coin", mentionHTML(opponent), res.Duel.Stake),
		tb.ttl)
}

func (tb *Bot) onMuteDuel(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	target, minutes, err := tb.parseDuelTarget(ctx, b, msg)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "🤐 "+html.EscapeString(err.Error()), tb.ttl)
		return
	}
	if minutes > 60 {
		minutes = 60
	}
	res, err := tb.economy.CreateMuteDuel(ctx, msg.Chat.ID, knownFromUser(msg.From), knownFromUser(target), minutes)
	if errors.Is(err, economy.ErrSelfDuel) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Нельзя вызвать на мут-дуэль самого себя.", tb.ttl)
		return
	}
	if errors.Is(err, economy.ErrInvalidAmount) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Количество минут должно быть больше нуля.", tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onMuteDuel: CreateMuteDuel: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("🤐 %s вызывает %s на мут-дуэль на <b>%d</b> мин.\n└ принять: <code>/accept_mute %d</code>",
			mentionHTML(msg.From), mentionHTML(target), res.Duel.Stake, msg.From.ID),
		tb.ttl)
}

func (tb *Bot) onAcceptMuteDuel(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	challenger, err := tb.parseAcceptTarget(ctx, b, msg)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "🤐 "+html.EscapeString(err.Error()), tb.ttl)
		return
	}
	res, err := tb.economy.AcceptMuteDuel(ctx, msg.Chat.ID, knownFromUser(msg.From), challenger.ID)
	if errors.Is(err, economy.ErrPendingDuelNotFound) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Активной мут-дуэли от этого игрока не найдено.", tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onAcceptMuteDuel: AcceptMuteDuel: %v", err)
		return
	}

	winner := msg.From
	loser := challenger
	if res.WinnerID == challenger.ID {
		winner = challenger
		loser = msg.From
	}
	duration := time.Duration(res.Duel.Stake) * time.Minute
	if err := muteUser(ctx, b, msg.Chat.ID, loser.ID, duration); err != nil {
		log.Printf("onAcceptMuteDuel: muteUser: %v", err)
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"⚠️ Дуэль сыграна, но замутить не вышло — нужны права админа (Restrict members).",
			tb.ttl)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("🤐 Мут-дуэль завершена\n└ победитель: %s, проигравший %s молчит <b>%d</b> мин.",
			mentionHTML(winner), mentionHTML(loser), res.Duel.Stake),
		20*time.Second)
}

func (tb *Bot) onCancelMuteDuel(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	opponent, err := tb.parseAcceptTarget(ctx, b, msg)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "🤐 "+html.EscapeString(err.Error()), tb.ttl)
		return
	}
	res, err := tb.economy.CancelMuteDuel(ctx, msg.Chat.ID, knownFromUser(msg.From), opponent.ID)
	if errors.Is(err, economy.ErrPendingDuelNotFound) {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Активной мут-дуэли с этим игроком не найдено.", tb.ttl)
		return
	}
	if err != nil {
		log.Printf("onCancelMuteDuel: CancelMuteDuel: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("🤐 Мут-дуэль с %s отменена\n└ ставка была <b>%d</b> мин.", mentionHTML(opponent), res.Duel.Stake),
		tb.ttl)
}

func (tb *Bot) onHelp(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) {
		return
	}
	text := strings.Join([]string{
		"🥰 <b>ASNC bot</b>",
		"",
		"⭐ <code>+rep</code> reply — дать репутацию и ASNC-coin",
		"📉 <code>-rep</code> reply — снять репутацию",
		"🔎 <code>/rep</code> — коротко про репутацию",
		"👤 <code>/profile</code> — полный профиль",
		"🏦 <code>/balance</code> — только ASNC-coin",
		"🏆 <code>/top rep</code> / <code>/top coin</code> — топы",
		"📊 <code>/dashboard</code> — общий обзор",
		"🏦 <code>/daily</code> — ежедневный ASNC-coin",
		"🎲 <code>/roll 25</code> — казино",
		"⚔️ <code>/duel @user 50</code> — вызвать на дуэль",
		"✅ <code>/accept @user</code> — принять дуэль",
		"✖️ <code>/cancel_duel @user</code> — отменить свой вызов",
		"🤐 <code>/mute_duel @user 5</code> — дуэль на мут",
		"✅ <code>/accept_mute @user</code> — принять мут-дуэль",
		"✖️ <code>/cancel_mute_duel @user</code> — отменить мут-дуэль",
	}, "\n")
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, text, tb.ttl)
}

func (tb *Bot) parseDuelTarget(ctx context.Context, b *bot.Bot, msg *models.Message) (*models.User, int64, error) {
	args, err := commandArgs(msg.Text)
	if err != nil {
		return nil, 0, errors.New("использование: /duel @user 50 или reply + /duel 50")
	}
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && len(args) == 1 {
		stake, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return nil, 0, errors.New("ставка должна быть числом")
		}
		return msg.ReplyToMessage.From, stake, nil
	}
	if len(args) != 2 {
		return nil, 0, errors.New("использование: /duel @user 50 или reply + /duel 50")
	}
	target, err := tb.resolveTarget(ctx, b, msg, args[0])
	if err != nil {
		return nil, 0, err
	}
	stake, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return nil, 0, errors.New("ставка должна быть числом")
	}
	return target, stake, nil
}

func (tb *Bot) parseAcceptTarget(ctx context.Context, b *bot.Bot, msg *models.Message) (*models.User, error) {
	args, _ := commandArgs(msg.Text)
	if len(args) == 0 {
		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
			return msg.ReplyToMessage.From, nil
		}
		return nil, errors.New("использование: /accept @user, /accept tg_id или reply + /accept")
	}
	if len(args) != 1 {
		return nil, errors.New("ожидается один аргумент: @user или tg_id")
	}
	return tb.resolveTarget(ctx, b, msg, args[0])
}

func profileHTML(user *models.User, profile ports.EconomyProfile) string {
	a := profile.Account
	return fmt.Sprintf(
		"🥰 <b>Профиль %s</b>\n\n⭐ Репутация: <b>%s</b>\n🏦 ASNC-coin: <b>%d</b>\n🔥 Daily streak: <b>%d</b>\n🎲 Казино: <b>%d</b> побед / <b>%d</b> игр\n⚔️ Дуэли: <b>%d</b> побед / <b>%d</b> игр\n🏷 Ранг: <b>%s</b>",
		mentionHTML(user),
		scoreHTML(profile.Reputation.Score),
		a.Balance,
		a.DailyStreak,
		a.CasinoWins,
		a.CasinoWins+a.CasinoLosses,
		a.DuelWins,
		a.DuelWins+a.DuelLosses,
		rank(a.Balance, profile.Reputation.Score),
	)
}

func rollHTML(user *models.User, res ports.RollResult) string {
	outcome := "проигрыш"
	if res.Profit == 0 {
		outcome = "возврат ставки"
	}
	if res.Profit > 0 {
		outcome = "выигрыш " + signedDelta(int(res.Profit))
	}
	return fmt.Sprintf("🎲 %s бросает кубик: <b>%d</b>\n└ %s, ASNC-coin: <b>%d</b>",
		mentionHTML(user), res.Die, html.EscapeString(outcome), res.Account.Balance)
}

func economyProfileNameHTML(profile ports.EconomyProfile) string {
	if profile.HasKnownUser {
		return knownUserHTML(profile.KnownUser)
	}
	return storedUserHTML(profile.Reputation.Username, profile.Reputation.DisplayName, profile.Account.UserID)
}

func coinTopHTML(dashboard ports.EconomyDashboard, title string, includeRep bool) string {
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n")
	for i, profile := range dashboard.TopBalances {
		sb.WriteString(medal(i))
		sb.WriteString(" ")
		sb.WriteString(economyProfileNameHTML(profile))
		sb.WriteString(" — <b>")
		sb.WriteString(strconv.FormatInt(profile.Account.Balance, 10))
		sb.WriteString("</b> coin")
		if includeRep && profile.Reputation.Score != 0 {
			sb.WriteString(" · rep <b>")
			sb.WriteString(scoreHTML(profile.Reputation.Score))
			sb.WriteString("</b>")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func knownUserHTML(user ports.KnownUser) string {
	if user.Username != "" {
		return "<code>@" + html.EscapeString(user.Username) + "</code>"
	}
	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if name == "" {
		name = fmt.Sprintf("id%d", user.UserID)
	}
	return "<b>" + html.EscapeString(name) + "</b>"
}

func rank(balance, score int64) string {
	power := balance + score*2
	switch {
	case power >= 1000:
		return "Легенда"
	case power >= 500:
		return "Магнат"
	case power >= 150:
		return "Уважаемый"
	case power >= 50:
		return "На подъёме"
	default:
		return "Новичок"
	}
}
