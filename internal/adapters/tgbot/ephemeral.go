package tgbot

import (
	"context"
	"log"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func sendEphemeral(ctx context.Context, b *bot.Bot, chatID int64, replyTo int, html string, ttl time.Duration) {

	disableLinkPreview := true
	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      html,
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: &disableLinkPreview,
		},
		ReplyParameters: &models.ReplyParameters{
			MessageID:                replyTo,
			AllowSendingWithoutReply: true,
		},
	})
	if err != nil {
		log.Printf("sendEphemeral: SendMessage: %v", err)
		return
	}
	time.AfterFunc(ttl, func() {
		delCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := b.DeleteMessage(delCtx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: msg.ID,
		}); err != nil {
			log.Printf("sendEphemeral: DeleteMessage: %v", err)
		}
	})
}

func reactThumb(ctx context.Context, b *bot.Bot, chatID int64, msgID int, positive bool) error {
	_, err := b.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    chatID,
		MessageID: msgID,
		Reaction: []models.ReactionType{{
			Type: models.ReactionTypeTypeEmoji,
			ReactionTypeEmoji: &models.ReactionTypeEmoji{
				Type:  models.ReactionTypeTypeEmoji,
				Emoji: reactionEmoji(positive),
			},
		}},
	})
	return err
}

func reactionEmoji(positive bool) string {
	if positive {
		return "🥰"
	}
	return "🤡"
}
