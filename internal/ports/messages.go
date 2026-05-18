package ports

import (
	"context"
	"time"
)

type ChatMessage struct {
	ID        int64
	ChatID    int64
	UserID    int64
	FirstName string
	Username  string
	Text      string
	CreatedAt time.Time
}

type MessageStore interface {
	SaveMessage(ctx context.Context, msg ChatMessage) error
	GetTodayMessages(ctx context.Context, chatID int64) ([]ChatMessage, error)
}
