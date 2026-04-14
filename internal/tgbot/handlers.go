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

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/reputation"
	"github.com/KuzyaFarts/asnc-telegram-bot/internal/storage"
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
	if msg.ReplyToMessage == nil {
		return
	}

	trig := reputation.Parse(msg)
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

	users, err := tb.svc.Top(ctx, msg.Chat.ID, 10)
	if err != nil {
		log.Printf("onTop: Top: %v", err)
		return
	}
	if len(users) == 0 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"📭 Пока никто не получал репутации.", tb.ttl)
		return
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
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, sb.String(), tb.ttl)
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
	until := time.Now().Add(duration).Unix()

	_, err := b.RestrictChatMember(ctx, &bot.RestrictChatMemberParams{
		ChatID: msg.Chat.ID,
		UserID: msg.From.ID,
		// TODO(dami): add CanSendOtherMessages permission after bug fix
		// See: https://github.com/go-telegram/bot/issues/271
		Permissions: &models.ChatPermissions{},
		UntilDate:   int(until),
	})
	if err != nil {
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

	tb.applyAndReport(ctx, b, msg, targetUser, delta, msg.ID)
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
	case reputation.ReasonOK:
		positive := res.AppliedDelta > 0
		if err := reactThumb(ctx, b, msg.Chat.ID, reactMsgID, positive); err != nil {
			log.Printf("applyAndReport: reactThumb: %v", err)
		}
		icon := "📈"
		if !positive {
			icon = "📉"
		}
		text := fmt.Sprintf(
			"%s %s → <b>%s</b> репутации\n└ итого: <b>%s</b>  %s",
			icon,
			mentionHTML(target),
			signedDelta(res.AppliedDelta),
			scoreHTML(res.NewScore),
			breakdownHTML(res.PositiveTotal, res.NegativeTotal),
		)
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, text, tb.ttl)

	case reputation.ReasonSelf:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"Нельзя менять репутацию <b>самому себе</b>.", tb.ttl)

	case reputation.ReasonBotTarget:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"<b>Ботам</b> нельзя менять репутацию.", tb.ttl)

	case reputation.ReasonCooldown:
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			fmt.Sprintf("Подожди ещё <b>%s</b>, прежде чем снова менять репутацию %s.",
				html.EscapeString(humanDuration(res.CooldownLeft)),
				mentionHTML(target)),
			tb.ttl)

	case reputation.ReasonZeroDelta:
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

func knownFromUser(u *models.User) storage.KnownUser {
	return storage.KnownUser{
		UserID:    u.ID,
		Username:  u.Username,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		IsBot:     u.IsBot,
	}
}

func userFromKnown(k storage.KnownUser) *models.User {
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

func actorFromUser(u *models.User) reputation.Actor {
	return reputation.Actor{
		UserID:      u.ID,
		Username:    u.Username,
		DisplayName: strings.TrimSpace(u.FirstName + " " + u.LastName),
		IsBot:       u.IsBot,
	}
}

func mentionHTML(u *models.User) string {
	if u.Username != "" {
		return "@" + html.EscapeString(u.Username)
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = fmt.Sprintf("id%d", u.ID)
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, u.ID, html.EscapeString(name))
}

func storedUserHTML(username, displayName string, userID int64) string {
	if username != "" {
		return "@" + html.EscapeString(username)
	}
	name := displayName
	if name == "" {
		name = fmt.Sprintf("id%d", userID)
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, userID, html.EscapeString(name))
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
