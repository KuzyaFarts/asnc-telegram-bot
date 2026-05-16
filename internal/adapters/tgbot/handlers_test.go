package tgbot

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestExplicitReactMessageID(t *testing.T) {
	msg := &models.Message{
		ID: 10,
		ReplyToMessage: &models.Message{
			ID: 20,
		},
	}

	if got := explicitReactMessageID(msg, ""); got != 20 {
		t.Fatalf("explicitReactMessageID(reply command) = %d, want reply message", got)
	}
	if got := explicitReactMessageID(msg, "@user"); got != 10 {
		t.Fatalf("explicitReactMessageID(targeted command) = %d, want command message", got)
	}
}
