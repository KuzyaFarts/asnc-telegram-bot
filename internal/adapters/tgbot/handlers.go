package tgbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
	"github.com/KuzyaFarts/asnc-telegram-bot/internal/reputation"
)

func (tb *Bot) onMessage(ctx context.Context, b *bot.Bot, u *models.Update) {

	msg := u.Message
	if msg == nil {
		return
	}
	if !isGroupChat(msg.Chat.Type) {
		return
	}
	tb.rememberParticipants(ctx, msg)
	if msg.From == nil {
		return
	}

	if msg.Text != "" && !strings.HasPrefix(msg.Text, "/") {
		name := msg.From.FirstName
		uname := msg.From.Username
		_ = tb.msgStore.SaveMessage(ctx, ports.ChatMessage{
			ChatID:    msg.Chat.ID,
			UserID:    msg.From.ID,
			FirstName: name,
			Username:  uname,
			Text:      msg.Text,
			CreatedAt: time.Now().UTC(),
		})
	}

	if msg.ReplyToMessage == nil {
		return
	}

	trig := parseReputationTrigger(msg)
	if trig == nil {
		return
	}

	target := msg.ReplyToMessage.From
	if target == nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"У исходного сообщения нет автора — репутация не меняется.", tb.ttl)
		return
	}

	tb.applyAndReport(ctx, b, msg, target, trig.Delta, msg.ReplyToMessage.ID)
}

func (tb *Bot) onRep(ctx context.Context, b *bot.Bot, u *models.Update) {

	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	target := msg.From
	ownRequest := true
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		target = msg.ReplyToMessage.From
		ownRequest = false
	}

	usr, _, err := tb.svc.GetUser(ctx, msg.Chat.ID, target.ID)
	if err != nil {
		log.Printf("onRep: GetUser: %v", err)
		return
	}

	var who string
	if ownRequest {
		who = "Твоя репутация"
	} else {
		who = "Репутация " + mentionHTML(target)
	}
	text := fmt.Sprintf("⭐ %s: <b>%s</b>\n└ %s",
		who, scoreHTML(usr.Score), breakdownHTML(usr.PositiveGiven, usr.NegativeGiven))
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, text, tb.ttl)
}

func (tb *Bot) onTop(ctx context.Context, b *bot.Bot, u *models.Update) {

	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) {
		return
	}
	tb.rememberParticipants(ctx, msg)

	args, _ := commandArgs(msg.Text)
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "coin", "coins", "bank", "balance":
			tb.sendCoinTop(ctx, b, msg)
			return
		case "rep", "reputation":
		default:
			sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Использование: <code>/top</code>, <code>/top rep</code> или <code>/top coin</code>.", tb.ttl)
			return
		}
	}

	text, err := tb.reputationTopHTML(ctx, msg.Chat.ID, 10)
	if err != nil {
		log.Printf("onTop: Top: %v", err)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, text, tb.ttl)
}

func (tb *Bot) reputationTopHTML(ctx context.Context, chatID int64, limit int) (string, error) {
	users, err := tb.svc.Top(ctx, chatID, limit)
	if err != nil {
		return "", err
	}
	if len(users) == 0 {
		return "📭 Пока никто не получал репутации.", nil
	}

	var sb strings.Builder
	sb.WriteString("🏆 <b>Топ репутации</b>\n")
	for i, u := range users {
		sb.WriteString(medal(i))
		sb.WriteString(" ")
		sb.WriteString(storedUserHTML(u.Username, u.DisplayName, u.UserID))
		sb.WriteString(" — <b>")
		sb.WriteString(scoreHTML(u.Score))
		sb.WriteString("</b>  ")
		sb.WriteString(breakdownHTML(u.PositiveGiven, u.NegativeGiven))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func (tb *Bot) onPlusRep(ctx context.Context, b *bot.Bot, u *models.Update) {
	tb.handleExplicitRep(ctx, b, u, +1)
}

func (tb *Bot) onMinusRep(ctx context.Context, b *bot.Bot, u *models.Update) {
	tb.handleExplicitRep(ctx, b, u, -1)
}

func (tb *Bot) onPremiddle(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	duration := time.Duration(rand.Int31n(29)+1) * time.Minute

	if err := muteUser(ctx, b, msg.Chat.ID, msg.From.ID, duration); err != nil {
		log.Printf("onPremiddle: RestrictChatMember: %v", err)
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"⚠️ Не могу замутить — нужны права админа (Restrict members).", tb.ttl)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("🤐 %s замучен на <b>%.0f</b> мин. Сам виноват.", mentionHTML(msg.From), duration.Minutes()),
		tb.ttl)
}

func (tb *Bot) handleExplicitRep(ctx context.Context, b *bot.Bot, u *models.Update, sign int) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}
	tb.rememberParticipants(ctx, msg)

	args, err := commandArgs(msg.Text)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"⚠️ "+html.EscapeString(err.Error()), tb.ttl)
		return
	}

	hasReply := msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil

	var (
		targetSpec string
		amount     int = 1
	)

	switch len(args) {
	case 0:
		if !hasReply {
			sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
				"Использование: <code>/plus_rep @user [число]</code>, <code>/plus_rep tg_id [число]</code> или reply + число.", tb.ttl)
			return
		}

	case 1:
		if n, err := strconv.Atoi(args[0]); err == nil {
			if hasReply {
				amount = n
			} else {
				targetSpec = args[0]
			}
		} else {
			targetSpec = args[0]
		}

	case 2:
		targetSpec = args[0]
		n, err := strconv.Atoi(args[1])
		if err != nil {
			sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
				"Второй аргумент должен быть числом.", tb.ttl)
			return
		}
		amount = n

	default:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"Слишком много аргументов. Ожидается <code>[@user|tg_id] [число]</code>.", tb.ttl)
		return
	}

	if amount < 0 {
		amount = -amount
	}
	if amount == 0 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"Число должно быть не нулевым.", tb.ttl)
		return
	}
	delta := sign * amount

	targetUser, err := tb.resolveTarget(ctx, b, msg, targetSpec)
	if err != nil {
		log.Printf("handleExplicitRep: resolveTarget %q: %v", targetSpec, err)
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"⚠️ "+html.EscapeString(err.Error()), tb.ttl)
		return
	}

	tb.applyAndReport(ctx, b, msg, targetUser, delta, explicitReactMessageID(msg, targetSpec))
}

func (tb *Bot) resolveTarget(ctx context.Context, b *bot.Bot, msg *models.Message, spec string) (*models.User, error) {

	if spec == "" {
		if msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
			return nil, errors.New("нет цели: ответь на сообщение или укажи @user / tg_id")
		}
		return msg.ReplyToMessage.From, nil
	}

	if id, err := strconv.ParseInt(spec, 10, 64); err == nil {
		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.ID == id {
			return msg.ReplyToMessage.From, nil
		}
		tu, err := resolveUser(ctx, b, msg.Chat.ID, id)
		if err != nil {
			return nil, errors.New("не удалось найти пользователя в этом чате")
		}
		return tu, nil
	}

	name := strings.TrimPrefix(spec, "@")
	if name == "" {
		return nil, errors.New("пустой @username")
	}
	known, ok, err := tb.svc.FindByUsername(ctx, msg.Chat.ID, name)
	if err != nil {
		return nil, errors.New("ошибка поиска пользователя")
	}
	if !ok {
		return nil, fmt.Errorf("я не видел @%s в этом чате — пусть напишет что-нибудь или ответь на его сообщение", html.EscapeString(name))
	}
	return userFromKnown(known), nil
}

func (tb *Bot) applyAndReport(ctx context.Context, b *bot.Bot, msg *models.Message, target *models.User, delta int, reactMsgID int) {

	from := actorFromUser(msg.From)
	to := actorFromUser(target)

	res, err := tb.svc.Apply(ctx, msg.Chat.ID, from, to, delta)
	if err != nil {
		log.Printf("applyAndReport: svc.Apply: %v", err)
		return
	}

	switch res.Reason {
	case ports.ReasonOK:
		positive := res.AppliedDelta > 0
		if err := reactThumb(ctx, b, msg.Chat.ID, reactMsgID, positive); err != nil {
			log.Printf("applyAndReport: reactThumb: %v", err)
		}
		var coinLine string
		if positive {
			account, err := tb.economy.Award(ctx, msg.Chat.ID, knownFromUser(target), int64(res.AppliedDelta), "positive_rep", int64(msg.ID))
			if err != nil {
				log.Printf("applyAndReport: economy.Award: %v", err)
			} else {
				coinLine = fmt.Sprintf("\n└ +<b>%d</b> ASNC-coin, баланс: <b>%d</b>", res.AppliedDelta, account.Balance)
			}
		}
		icon := "📈"
		if !positive {
			icon = "📉"
		}
		text := fmt.Sprintf(
			"%s %s → <b>%s</b> репутации\n└ итого: <b>%s</b>  %s%s",
			icon,
			mentionHTML(target),
			signedDelta(res.AppliedDelta),
			scoreHTML(res.NewScore),
			breakdownHTML(res.PositiveTotal, res.NegativeTotal),
			coinLine,
		)
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, text, tb.ttl)

	case ports.ReasonSelf:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"Нельзя менять репутацию <b>самому себе</b>.", tb.ttl)

	case ports.ReasonBotTarget:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"<b>Ботам</b> нельзя менять репутацию.", tb.ttl)

	case ports.ReasonCooldown:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			fmt.Sprintf("Подожди ещё <b>%s</b>, прежде чем снова менять репутацию %s.",
				html.EscapeString(humanDuration(res.CooldownLeft)),
				mentionHTML(target)),
			tb.ttl)

	case ports.ReasonZeroDelta:
		return
	}
}

func (tb *Bot) rememberParticipants(ctx context.Context, msg *models.Message) {
	if msg == nil {
		return
	}
	if msg.From != nil {
		if err := tb.svc.Remember(ctx, msg.Chat.ID, knownFromUser(msg.From)); err != nil {
			log.Printf("rememberParticipants: sender: %v", err)
		}
	}
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		if err := tb.svc.Remember(ctx, msg.Chat.ID, knownFromUser(msg.ReplyToMessage.From)); err != nil {
			log.Printf("rememberParticipants: reply target: %v", err)
		}
	}
}

func knownFromUser(u *models.User) ports.KnownUser {
	return ports.KnownUser{
		UserID:    u.ID,
		Username:  u.Username,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		IsBot:     u.IsBot,
	}
}

func parseReputationTrigger(msg *models.Message) *reputation.Trigger {
	if msg.Sticker != nil {
		return reputation.ParseSticker(msg.Sticker.FileUniqueID)
	}
	return reputation.ParseText(msg.Text)
}

func userFromKnown(k ports.KnownUser) *models.User {
	return &models.User{
		ID:        k.UserID,
		Username:  k.Username,
		FirstName: k.FirstName,
		LastName:  k.LastName,
		IsBot:     k.IsBot,
	}
}

func commandArgs(text string) ([]string, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil, errors.New("пустая команда")
	}
	return fields[1:], nil
}

func explicitReactMessageID(msg *models.Message, targetSpec string) int {
	if targetSpec == "" && msg.ReplyToMessage != nil {
		return msg.ReplyToMessage.ID
	}
	return msg.ID
}

func resolveUser(ctx context.Context, b *bot.Bot, chatID, userID int64) (*models.User, error) {
	cm, err := b.GetChatMember(ctx, &bot.GetChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return userFromChatMember(cm)
}

func muteUser(ctx context.Context, b *bot.Bot, chatID, userID int64, duration time.Duration) error {
	until := time.Now().Add(duration).Unix()
	_, err := b.RestrictChatMember(ctx, &bot.RestrictChatMemberParams{
		ChatID: chatID,
		UserID: userID,
		// TODO(dami): add CanSendOtherMessages permission after bug fix
		// See: https://github.com/go-telegram/bot/issues/271
		Permissions: &models.ChatPermissions{},
		UntilDate:   int(until),
	})
	return err
}

func userFromChatMember(cm *models.ChatMember) (*models.User, error) {
	switch cm.Type {
	case models.ChatMemberTypeOwner:
		if cm.Owner != nil {
			return cm.Owner.User, nil
		}
	case models.ChatMemberTypeAdministrator:
		if cm.Administrator != nil {
			u := cm.Administrator.User
			return &u, nil
		}
	case models.ChatMemberTypeMember:
		if cm.Member != nil {
			return cm.Member.User, nil
		}
	case models.ChatMemberTypeRestricted:
		if cm.Restricted != nil {
			return cm.Restricted.User, nil
		}
	case models.ChatMemberTypeLeft:
		if cm.Left != nil {
			return cm.Left.User, nil
		}
	case models.ChatMemberTypeBanned:
		if cm.Banned != nil {
			return cm.Banned.User, nil
		}
	}
	return nil, errors.New("chat member has no user")
}

func isGroupChat(t models.ChatType) bool {
	return t == models.ChatTypeGroup || t == models.ChatTypeSupergroup
}

func actorFromUser(u *models.User) ports.Actor {
	return ports.Actor{
		UserID:      u.ID,
		Username:    u.Username,
		DisplayName: strings.TrimSpace(u.FirstName + " " + u.LastName),
		IsBot:       u.IsBot,
	}
}

func mentionHTML(u *models.User) string {
	if u.Username != "" {
		return "<code>@" + html.EscapeString(u.Username) + "</code>"
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = fmt.Sprintf("id%d", u.ID)
	}
	return "<b>" + html.EscapeString(name) + "</b>"
}

func storedUserHTML(username, displayName string, userID int64) string {
	if username != "" {
		return "<code>@" + html.EscapeString(username) + "</code>"
	}
	name := displayName
	if name == "" {
		name = fmt.Sprintf("id%d", userID)
	}
	return "<b>" + html.EscapeString(name) + "</b>"
}

func signedDelta(d int) string {
	if d >= 0 {
		return fmt.Sprintf("+%d", d)
	}
	return fmt.Sprintf("%d", d)
}

func scoreHTML(s int64) string {
	if s > 0 {
		return fmt.Sprintf("+%d", s)
	}
	return fmt.Sprintf("%d", s)
}

func breakdownHTML(pos, neg int64) string {
	return fmt.Sprintf("(👍 <b>%d</b> / 👎 <b>%d</b>)", pos, neg)
}

func medal(idx int) string {
	switch idx {
	case 0:
		return "🥇"
	case 1:
		return "🥈"
	case 2:
		return "🥉"
	default:
		return fmt.Sprintf("<b>%d.</b>", idx+1)
	}
}

func humanDuration(d time.Duration) string {

	if d < time.Minute {
		secs := max(int((d+time.Second-1)/time.Second), 1)
		return fmt.Sprintf("%d сек", secs)
	}
	mins := int((d + time.Minute - 1) / time.Minute)
	return fmt.Sprintf("%d мин", mins)
}
