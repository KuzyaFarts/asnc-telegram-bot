package ports

import (
	"context"
	"time"
)

type Reason string

const (
	ReasonOK        Reason = "ok"
	ReasonSelf      Reason = "self"
	ReasonBotTarget Reason = "bot_target"
	ReasonCooldown  Reason = "cooldown"
	ReasonZeroDelta Reason = "zero_delta"
)

type Actor struct {
	UserID      int64
	Username    string
	DisplayName string
	IsBot       bool
}

type ApplyResult struct {
	Reason        Reason
	AppliedDelta  int
	NewScore      int64
	PositiveTotal int64
	NegativeTotal int64
	CooldownLeft  time.Duration
}

type User struct {
	ChatID        int64
	UserID        int64
	Username      string
	DisplayName   string
	Score         int64
	PositiveGiven int64
	NegativeGiven int64
}

type KnownUser struct {
	UserID    int64
	Username  string
	FirstName string
	LastName  string
	IsBot     bool
}

type ReputationUseCase interface {
	Apply(ctx context.Context, chatID int64, from, to Actor, rawDelta int) (*ApplyResult, error)
	GetUser(ctx context.Context, chatID, userID int64) (User, bool, error)
	Score(ctx context.Context, chatID, userID int64) (int64, error)
	Top(ctx context.Context, chatID int64, n int) ([]User, error)
	Remember(ctx context.Context, chatID int64, u KnownUser) error
	FindByUsername(ctx context.Context, chatID int64, username string) (KnownUser, bool, error)
}

type ReputationRepository interface {
	ApplyDelta(ctx context.Context, chatID, userID int64, username, displayName string, delta int, at time.Time) (User, error)
	GetUser(ctx context.Context, chatID, userID int64) (User, bool, error)
	GetScore(ctx context.Context, chatID, userID int64) (int64, error)
	Top(ctx context.Context, chatID int64, limit int) ([]User, error)
	GetLastChange(ctx context.Context, chatID, fromID, toID int64) (time.Time, bool, error)
	TouchCooldown(ctx context.Context, chatID, fromID, toID int64, at time.Time) error
	RememberUser(ctx context.Context, chatID int64, u KnownUser, at time.Time) error
	FindByUsername(ctx context.Context, chatID int64, username string) (KnownUser, bool, error)
}
