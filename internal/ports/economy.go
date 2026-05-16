package ports

import (
	"context"
	"time"
)

type EconomyAccount struct {
	ChatID       int64
	UserID       int64
	Balance      int64
	DailyStreak  int
	LastDailyAt  time.Time
	CasinoWins   int64
	CasinoLosses int64
	DuelWins     int64
	DuelLosses   int64
	UpdatedAt    time.Time
}

type EconomyTransaction struct {
	ChatID    int64
	UserID    int64
	Delta     int64
	Reason    string
	RefID     int64
	CreatedAt time.Time
}

type EconomyProfile struct {
	Account      EconomyAccount
	KnownUser    KnownUser
	Reputation   User
	HasKnownUser bool
}

type EconomyDashboard struct {
	TopBalances []EconomyProfile
}

type DailyResult struct {
	Account     EconomyAccount
	Reward      int64
	Streak      int
	NextClaimIn time.Duration
	Claimed     bool
}

type RollResult struct {
	Account EconomyAccount
	Bet     int64
	Die     int
	Payout  int64
	Profit  int64
}

type DuelStatus string
type DuelKind string

const (
	DuelStatusPending   DuelStatus = "pending"
	DuelStatusCompleted DuelStatus = "completed"
	DuelStatusCanceled  DuelStatus = "canceled"

	DuelKindCoin DuelKind = "coin"
	DuelKindMute DuelKind = "mute"
)

type Duel struct {
	ID           int64
	ChatID       int64
	ChallengerID int64
	OpponentID   int64
	Kind         DuelKind
	Stake        int64
	Status       DuelStatus
	WinnerID     int64
	CreatedAt    time.Time
	ResolvedAt   time.Time
}

type DuelResult struct {
	Duel      Duel
	WinnerID  int64
	Bank      int64
	Completed bool
}

type EconomyUseCase interface {
	Profile(ctx context.Context, chatID, userID int64) (EconomyProfile, error)
	Dashboard(ctx context.Context, chatID int64, limit int) (EconomyDashboard, error)
	Award(ctx context.Context, chatID int64, user KnownUser, amount int64, reason string, refID int64) (EconomyAccount, error)
	ClaimDaily(ctx context.Context, chatID int64, user KnownUser) (DailyResult, error)
	Roll(ctx context.Context, chatID int64, user KnownUser, bet int64) (RollResult, error)
	CreateDuel(ctx context.Context, chatID int64, challenger, opponent KnownUser, stake int64) (DuelResult, error)
	AcceptDuel(ctx context.Context, chatID int64, opponent KnownUser, challengerID int64) (DuelResult, error)
	CancelDuel(ctx context.Context, chatID int64, challenger KnownUser, opponentID int64) (DuelResult, error)
	CreateMuteDuel(ctx context.Context, chatID int64, challenger, opponent KnownUser, minutes int64) (DuelResult, error)
	AcceptMuteDuel(ctx context.Context, chatID int64, opponent KnownUser, challengerID int64) (DuelResult, error)
	CancelMuteDuel(ctx context.Context, chatID int64, challenger KnownUser, opponentID int64) (DuelResult, error)
}

type EconomyRepository interface {
	GetEconomyAccount(ctx context.Context, chatID, userID int64) (EconomyAccount, bool, error)
	SaveEconomyAccount(ctx context.Context, account EconomyAccount) error
	SaveEconomyAccountWithTransaction(ctx context.Context, account EconomyAccount, tx EconomyTransaction) error
	TopEconomyAccounts(ctx context.Context, chatID int64, limit int) ([]EconomyProfile, error)
	AddTransaction(ctx context.Context, chatID, userID int64, delta int64, reason string, refID int64, at time.Time) error
	CreateDuel(ctx context.Context, duel Duel) (Duel, error)
	GetPendingDuel(ctx context.Context, chatID, challengerID, opponentID int64, kind DuelKind) (Duel, bool, error)
	CompleteDuel(ctx context.Context, duel Duel) error
	CompleteDuelWithAccounts(ctx context.Context, duel Duel, accounts []EconomyAccount, txs []EconomyTransaction) error
	CancelDuel(ctx context.Context, duel Duel) error
}
