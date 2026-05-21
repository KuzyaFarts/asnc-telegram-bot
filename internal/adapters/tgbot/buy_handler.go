package tgbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type buyPackage struct {
	Stars int
	Coins int64
	Bonus string
}

var buyPackages = []buyPackage{
	{Stars: 50, Coins: 87, Bonus: ""},
	{Stars: 150, Coins: 300, Bonus: "+15%"},
	{Stars: 500, Coins: 1100, Bonus: "+26%"},
}

func pkgByPayload(payload string) *buyPackage {
	for i, p := range buyPackages {
		if fmt.Sprintf("buy_%d", p.Stars) == payload {
			return &buyPackages[i]
		}
	}
	return nil
}

func (tb *Bot) onBuy(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || msg.From == nil {
		return
	}

	var rows [][]models.InlineKeyboardButton
	for _, pkg := range buyPackages {
		label := fmt.Sprintf("⭐ %d Stars → %d ASNC-coin", pkg.Stars, pkg.Coins)
		if pkg.Bonus != "" {
			label += " " + pkg.Bonus
		}
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         label,
			CallbackData: fmt.Sprintf("buy_%d", pkg.Stars),
		}})
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    msg.Chat.ID,
		Text:      "💰 <b>Купить ASNC-coin за Telegram Stars</b>\n\nВыбери пакет:",
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: rows,
		},
	})
}

func (tb *Bot) onBuyCallback(ctx context.Context, b *bot.Bot, u *models.Update) {
	cq := u.CallbackQuery
	if cq == nil {
		return
	}
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	pkg := pkgByPayload(cq.Data)
	if pkg == nil {
		return
	}

	if cq.Message.Message == nil {
		return
	}
	chatID := cq.Message.Message.Chat.ID
	msgID := cq.Message.Message.ID

	label := fmt.Sprintf("⭐ %d Stars → %d ASNC-coin", pkg.Stars, pkg.Coins)
	if pkg.Bonus != "" {
		label += " " + pkg.Bonus
	}
	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      fmt.Sprintf("💰 <b>Купить ASNC-coin за Telegram Stars</b>\n\n%s\n\n⏳ Выставляем счёт...", label),
		ParseMode: models.ParseModeHTML,
	})

	b.SendInvoice(ctx, &bot.SendInvoiceParams{
		ChatID:      chatID,
		Title:       "ASNC-coin",
		Description: fmt.Sprintf("%d ASNC-coin за %d Telegram Stars", pkg.Coins, pkg.Stars),
		Payload:     fmt.Sprintf("buy_%d", pkg.Stars),
		Currency:    "XTR",
		Prices: []models.LabeledPrice{
			{Label: fmt.Sprintf("%d ASNC-coin", pkg.Coins), Amount: pkg.Stars},
		},
	})
}

func (tb *Bot) onPreCheckoutQuery(ctx context.Context, b *bot.Bot, u *models.Update) {
	pq := u.PreCheckoutQuery
	ok := pkgByPayload(pq.InvoicePayload) != nil
	errMsg := ""
	if !ok {
		errMsg = "Неизвестный пакет."
	}
	b.AnswerPreCheckoutQuery(ctx, &bot.AnswerPreCheckoutQueryParams{
		PreCheckoutQueryID: pq.ID,
		OK:                 ok,
		ErrorMessage:       errMsg,
	})
}

func (tb *Bot) onSuccessfulPayment(ctx context.Context, b *bot.Bot, msg *models.Message) {
	sp := msg.SuccessfulPayment
	if sp == nil || msg.From == nil {
		return
	}

	pkg := pkgByPayload(sp.InvoicePayload)
	if pkg == nil {
		return
	}

	acc, err := tb.economy.Award(ctx, msg.Chat.ID, knownFromUser(msg.From), pkg.Coins, "stars_purchase", 0)
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
			"⚠️ Оплата прошла, но зачислить монеты не удалось — напиши администратору.", tb.ttl)
		return
	}

	label := fmt.Sprintf("buy_%d", pkg.Stars)
	_ = label

	name := msg.From.FirstName
	if msg.From.Username != "" {
		name = msg.From.Username
	}
	bonusStr := ""
	if pkg.Bonus != "" {
		bonusStr = fmt.Sprintf(" (%s)", pkg.Bonus)
	}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text: fmt.Sprintf(
			"✅ <b>%s</b> купил <b>%d ASNC-coin%s</b> за <b>%d ⭐</b>\n└ баланс: <b>%d</b>",
			strings.TrimSpace(name), pkg.Coins, bonusStr, pkg.Stars, acc.Balance,
		),
		ParseMode: models.ParseModeHTML,
	})
}

func (tb *Bot) onGive(ctx context.Context, b *bot.Bot, u *models.Update) {
	msg := u.Message
	if msg == nil || !isGroupChat(msg.Chat.Type) || msg.From == nil {
		return
	}

	cm, err := b.GetChatMember(ctx, &bot.GetChatMemberParams{ChatID: msg.Chat.ID, UserID: msg.From.ID})
	if err != nil {
		return
	}
	if cm.Type != models.ChatMemberTypeOwner && cm.Type != models.ChatMemberTypeAdministrator {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "⛔ Только администраторы могут использовать /give.", tb.ttl)
		return
	}

	args, _ := commandArgs(msg.Text)
	if len(args) != 2 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Использование: <code>/give @user 87</code>", tb.ttl)
		return
	}
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "Количество монет должно быть положительным числом.", tb.ttl)
		return
	}

	target, err := tb.resolveTarget(ctx, b, msg, args[0])
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "⚠️ "+err.Error(), tb.ttl)
		return
	}

	acc, err := tb.economy.Award(ctx, msg.Chat.ID, knownFromUser(target), amount, "admin_give", int64(msg.ID))
	if err != nil {
		sendEphemeral(ctx, b, msg.Chat.ID, msg.ID, "⚠️ Не удалось зачислить монеты.", tb.ttl)
		return
	}
	sendEphemeral(ctx, b, msg.Chat.ID, msg.ID,
		fmt.Sprintf("✅ %s зачислено <b>%d</b> ASNC-coin\n└ баланс: <b>%d</b>", mentionHTML(target), amount, acc.Balance),
		tb.ttl)
}
