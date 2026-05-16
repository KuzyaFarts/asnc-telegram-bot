package economy

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

const defaultDuelTTL = 10 * time.Minute

var (
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrDailyAlreadyClaimed = errors.New("daily already claimed")
	ErrSelfDuel            = errors.New("cannot duel yourself")
	ErrPendingDuelNotFound = errors.New("pending duel not found")
)

type Service struct {
	repo     ports.EconomyRepository
	reps     ports.ReputationUseCase
	settings Settings
	now      func() time.Time
	rnd      *rand.Rand
}

type Settings struct {
	DailyCooldown time.Duration
	DailyMin      int64
	DailyMax      int64
	DuelTTL       time.Duration
}

func DefaultSettings() Settings {
	return Settings{
		DailyCooldown: 24 * time.Hour,
		DailyMin:      5,
		DailyMax:      20,
		DuelTTL:       defaultDuelTTL,
	}
}

func New(repo ports.EconomyRepository, reps ports.ReputationUseCase, settings Settings) *Service {
	settings = normalizeSettings(settings)
	return &Service{
		repo:     repo,
		reps:     reps,
		settings: settings,
		now:      time.Now,
		rnd:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Service) Profile(ctx context.Context, chatID, userID int64) (ports.EconomyProfile, error) {
	account, err := s.ensureAccount(ctx, chatID, userID)
	if err != nil {
		return ports.EconomyProfile{}, err
	}
	rep, _, err := s.reps.GetUser(ctx, chatID, userID)
	if err != nil {
		return ports.EconomyProfile{}, err
	}
	return ports.EconomyProfile{Account: account, Reputation: rep}, nil
}

func (s *Service) Dashboard(ctx context.Context, chatID int64, limit int) (ports.EconomyDashboard, error) {
	if limit <= 0 {
		limit = 10
	}
	top, err := s.repo.TopEconomyAccounts(ctx, chatID, limit)
	if err != nil {
		return ports.EconomyDashboard{}, err
	}
	return ports.EconomyDashboard{TopBalances: top}, nil
}

func (s *Service) Award(ctx context.Context, chatID int64, user ports.KnownUser, amount int64, reason string, refID int64) (ports.EconomyAccount, error) {
	if amount <= 0 {
		return ports.EconomyAccount{}, ErrInvalidAmount
	}
	now := s.now()
	account, err := s.ensureAccount(ctx, chatID, user.UserID)
	if err != nil {
		return ports.EconomyAccount{}, err
	}
	account.Balance += amount
	account.UpdatedAt = now
	if err := s.repo.SaveEconomyAccountWithTransaction(ctx, account, ports.EconomyTransaction{
		ChatID:    chatID,
		UserID:    user.UserID,
		Delta:     amount,
		Reason:    reason,
		RefID:     refID,
		CreatedAt: now,
	}); err != nil {
		return ports.EconomyAccount{}, err
	}
	return account, nil
}

func (s *Service) ClaimDaily(ctx context.Context, chatID int64, user ports.KnownUser) (ports.DailyResult, error) {
	now := s.now()
	account, err := s.ensureAccount(ctx, chatID, user.UserID)
	if err != nil {
		return ports.DailyResult{}, err
	}

	if !account.LastDailyAt.IsZero() {
		next := account.LastDailyAt.Add(s.settings.DailyCooldown)
		if now.Before(next) {
			return ports.DailyResult{
				Account:     account,
				Streak:      account.DailyStreak,
				NextClaimIn: next.Sub(now),
			}, ErrDailyAlreadyClaimed
		}
	}

	streak := 1
	if !account.LastDailyAt.IsZero() && now.Sub(account.LastDailyAt) <= 48*time.Hour {
		streak = account.DailyStreak + 1
	}
	reward := s.settings.DailyMin + int64(s.rnd.Intn(int(s.settings.DailyMax-s.settings.DailyMin+1)))
	reward += int64(min(streak-1, 10))

	account.Balance += reward
	account.DailyStreak = streak
	account.LastDailyAt = now
	account.UpdatedAt = now
	if err := s.repo.SaveEconomyAccountWithTransaction(ctx, account, ports.EconomyTransaction{
		ChatID:    chatID,
		UserID:    user.UserID,
		Delta:     reward,
		Reason:    "daily",
		CreatedAt: now,
	}); err != nil {
		return ports.DailyResult{}, err
	}
	return ports.DailyResult{Account: account, Reward: reward, Streak: streak, Claimed: true}, nil
}

func (s *Service) Roll(ctx context.Context, chatID int64, user ports.KnownUser, bet int64) (ports.RollResult, error) {
	if bet <= 0 {
		return ports.RollResult{}, ErrInvalidAmount
	}
	now := s.now()
	account, err := s.ensureAccount(ctx, chatID, user.UserID)
	if err != nil {
		return ports.RollResult{}, err
	}
	if account.Balance < bet {
		return ports.RollResult{Account: account, Bet: bet}, ErrInsufficientBalance
	}

	die := s.rnd.Intn(6) + 1
	payout := int64(0)
	switch die {
	case 3, 4:
		payout = bet
	case 5:
		payout = bet + bet/2
	case 6:
		payout = bet * 2
	}

	account.Balance = account.Balance - bet + payout
	if payout > bet {
		account.CasinoWins++
	} else {
		account.CasinoLosses++
	}
	account.UpdatedAt = now
	if err := s.repo.SaveEconomyAccountWithTransaction(ctx, account, ports.EconomyTransaction{
		ChatID:    chatID,
		UserID:    user.UserID,
		Delta:     payout - bet,
		Reason:    "casino_roll",
		RefID:     int64(die),
		CreatedAt: now,
	}); err != nil {
		return ports.RollResult{}, err
	}
	return ports.RollResult{
		Account: account,
		Bet:     bet,
		Die:     die,
		Payout:  payout,
		Profit:  payout - bet,
	}, nil
}

func (s *Service) CreateDuel(ctx context.Context, chatID int64, challenger, opponent ports.KnownUser, stake int64) (ports.DuelResult, error) {
	if stake <= 0 {
		return ports.DuelResult{}, ErrInvalidAmount
	}
	if challenger.UserID == opponent.UserID {
		return ports.DuelResult{}, ErrSelfDuel
	}
	account, err := s.ensureAccount(ctx, chatID, challenger.UserID)
	if err != nil {
		return ports.DuelResult{}, err
	}
	if account.Balance < stake {
		return ports.DuelResult{}, ErrInsufficientBalance
	}
	duel, err := s.repo.CreateDuel(ctx, ports.Duel{
		ChatID:       chatID,
		ChallengerID: challenger.UserID,
		OpponentID:   opponent.UserID,
		Kind:         ports.DuelKindCoin,
		Stake:        stake,
		Status:       ports.DuelStatusPending,
		CreatedAt:    s.now(),
	})
	if err != nil {
		return ports.DuelResult{}, err
	}
	return ports.DuelResult{Duel: duel, Bank: stake * 2}, nil
}

func (s *Service) AcceptDuel(ctx context.Context, chatID int64, opponent ports.KnownUser, challengerID int64) (ports.DuelResult, error) {
	duel, ok, err := s.repo.GetPendingDuel(ctx, chatID, challengerID, opponent.UserID, ports.DuelKindCoin)
	if err != nil {
		return ports.DuelResult{}, err
	}
	if !ok {
		return ports.DuelResult{}, ErrPendingDuelNotFound
	}
	if s.now().Sub(duel.CreatedAt) > s.settings.DuelTTL {
		return ports.DuelResult{}, ErrPendingDuelNotFound
	}

	challenger, err := s.ensureAccount(ctx, chatID, duel.ChallengerID)
	if err != nil {
		return ports.DuelResult{}, err
	}
	acceptor, err := s.ensureAccount(ctx, chatID, opponent.UserID)
	if err != nil {
		return ports.DuelResult{}, err
	}
	if challenger.Balance < duel.Stake || acceptor.Balance < duel.Stake {
		return ports.DuelResult{}, ErrInsufficientBalance
	}

	winnerID := duel.ChallengerID
	if s.rnd.Intn(2) == 1 {
		winnerID = duel.OpponentID
	}
	loserID := duel.OpponentID
	if winnerID == duel.OpponentID {
		loserID = duel.ChallengerID
	}

	now := s.now()
	if winnerID == challenger.UserID {
		challenger.Balance += duel.Stake
		challenger.DuelWins++
		acceptor.Balance -= duel.Stake
		acceptor.DuelLosses++
	} else {
		acceptor.Balance += duel.Stake
		acceptor.DuelWins++
		challenger.Balance -= duel.Stake
		challenger.DuelLosses++
	}
	challenger.UpdatedAt = now
	acceptor.UpdatedAt = now
	duel.Status = ports.DuelStatusCompleted
	duel.WinnerID = winnerID
	duel.ResolvedAt = now
	if err := s.repo.CompleteDuelWithAccounts(ctx, duel,
		[]ports.EconomyAccount{challenger, acceptor},
		[]ports.EconomyTransaction{
			{ChatID: chatID, UserID: winnerID, Delta: duel.Stake, Reason: "duel_win", RefID: duel.ID, CreatedAt: now},
			{ChatID: chatID, UserID: loserID, Delta: -duel.Stake, Reason: "duel_loss", RefID: duel.ID, CreatedAt: now},
		},
	); err != nil {
		return ports.DuelResult{}, err
	}
	return ports.DuelResult{
		Duel:      duel,
		WinnerID:  winnerID,
		Bank:      duel.Stake * 2,
		Completed: true,
	}, nil
}

func (s *Service) CancelDuel(ctx context.Context, chatID int64, challenger ports.KnownUser, opponentID int64) (ports.DuelResult, error) {
	duel, ok, err := s.repo.GetPendingDuel(ctx, chatID, challenger.UserID, opponentID, ports.DuelKindCoin)
	if err != nil {
		return ports.DuelResult{}, err
	}
	if !ok {
		return ports.DuelResult{}, ErrPendingDuelNotFound
	}
	duel.Status = ports.DuelStatusCanceled
	duel.ResolvedAt = s.now()
	if err := s.repo.CancelDuel(ctx, duel); err != nil {
		return ports.DuelResult{}, err
	}
	return ports.DuelResult{Duel: duel}, nil
}

func (s *Service) CreateMuteDuel(ctx context.Context, chatID int64, challenger, opponent ports.KnownUser, minutes int64) (ports.DuelResult, error) {
	if minutes <= 0 {
		return ports.DuelResult{}, ErrInvalidAmount
	}
	if challenger.UserID == opponent.UserID {
		return ports.DuelResult{}, ErrSelfDuel
	}
	duel, err := s.repo.CreateDuel(ctx, ports.Duel{
		ChatID:       chatID,
		ChallengerID: challenger.UserID,
		OpponentID:   opponent.UserID,
		Kind:         ports.DuelKindMute,
		Stake:        minutes,
		Status:       ports.DuelStatusPending,
		CreatedAt:    s.now(),
	})
	if err != nil {
		return ports.DuelResult{}, err
	}
	return ports.DuelResult{Duel: duel}, nil
}

func (s *Service) AcceptMuteDuel(ctx context.Context, chatID int64, opponent ports.KnownUser, challengerID int64) (ports.DuelResult, error) {
	duel, ok, err := s.repo.GetPendingDuel(ctx, chatID, challengerID, opponent.UserID, ports.DuelKindMute)
	if err != nil {
		return ports.DuelResult{}, err
	}
	if !ok {
		return ports.DuelResult{}, ErrPendingDuelNotFound
	}
	if s.now().Sub(duel.CreatedAt) > s.settings.DuelTTL {
		return ports.DuelResult{}, ErrPendingDuelNotFound
	}

	winnerID := duel.ChallengerID
	if s.rnd.Intn(2) == 1 {
		winnerID = duel.OpponentID
	}
	now := s.now()
	duel.Status = ports.DuelStatusCompleted
	duel.WinnerID = winnerID
	duel.ResolvedAt = now
	if err := s.repo.CompleteDuel(ctx, duel); err != nil {
		return ports.DuelResult{}, err
	}
	return ports.DuelResult{
		Duel:      duel,
		WinnerID:  winnerID,
		Completed: true,
	}, nil
}

func (s *Service) CancelMuteDuel(ctx context.Context, chatID int64, challenger ports.KnownUser, opponentID int64) (ports.DuelResult, error) {
	duel, ok, err := s.repo.GetPendingDuel(ctx, chatID, challenger.UserID, opponentID, ports.DuelKindMute)
	if err != nil {
		return ports.DuelResult{}, err
	}
	if !ok {
		return ports.DuelResult{}, ErrPendingDuelNotFound
	}
	duel.Status = ports.DuelStatusCanceled
	duel.ResolvedAt = s.now()
	if err := s.repo.CancelDuel(ctx, duel); err != nil {
		return ports.DuelResult{}, err
	}
	return ports.DuelResult{Duel: duel}, nil
}

func (s *Service) ensureAccount(ctx context.Context, chatID, userID int64) (ports.EconomyAccount, error) {
	account, ok, err := s.repo.GetEconomyAccount(ctx, chatID, userID)
	if err != nil {
		return ports.EconomyAccount{}, err
	}
	if ok {
		return account, nil
	}
	account = ports.EconomyAccount{
		ChatID:    chatID,
		UserID:    userID,
		UpdatedAt: s.now(),
	}
	if err := s.repo.SaveEconomyAccount(ctx, account); err != nil {
		return ports.EconomyAccount{}, err
	}
	return account, nil
}

func normalizeSettings(settings Settings) Settings {
	defaults := DefaultSettings()
	if settings.DailyCooldown <= 0 {
		settings.DailyCooldown = defaults.DailyCooldown
	}
	if settings.DailyMin <= 0 {
		settings.DailyMin = defaults.DailyMin
	}
	if settings.DailyMax < settings.DailyMin {
		settings.DailyMax = settings.DailyMin
	}
	if settings.DuelTTL <= 0 {
		settings.DuelTTL = defaults.DuelTTL
	}
	return settings
}
