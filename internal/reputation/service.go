package reputation

import (
	"context"
	"time"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

type Service struct {
	store    ports.ReputationRepository
	cooldown time.Duration
	maxDelta int
	now      func() time.Time // для тестов
}

func New(store ports.ReputationRepository, cooldown time.Duration, maxDelta int) *Service {

	return &Service{
		store:    store,
		cooldown: cooldown,
		maxDelta: maxDelta,
		now:      time.Now,
	}
}

func (s *Service) Apply(ctx context.Context, chatID int64, from, to ports.Actor, rawDelta int) (*ports.ApplyResult, error) {

	if from.UserID == to.UserID {
		return &ports.ApplyResult{Reason: ports.ReasonSelf}, nil
	}

	if to.IsBot {
		return &ports.ApplyResult{Reason: ports.ReasonBotTarget}, nil
	}

	delta := clamp(rawDelta, -s.maxDelta, s.maxDelta)
	if delta == 0 {
		return &ports.ApplyResult{Reason: ports.ReasonZeroDelta}, nil
	}

	now := s.now()
	if s.cooldown > 0 {
		last, ok, err := s.store.GetLastChange(ctx, chatID, from.UserID, to.UserID)
		if err != nil {
			return nil, err
		}
		if ok {
			elapsed := now.Sub(last)
			if elapsed < s.cooldown {
				return &ports.ApplyResult{
					Reason:       ports.ReasonCooldown,
					CooldownLeft: s.cooldown - elapsed,
				}, nil
			}
		}
	}

	u, err := s.store.ApplyDelta(ctx, chatID, to.UserID, to.Username, to.DisplayName, delta, now)
	if err != nil {
		return nil, err
	}
	if err := s.store.TouchCooldown(ctx, chatID, from.UserID, to.UserID, now); err != nil {
		return nil, err
	}
	return &ports.ApplyResult{
		Reason:        ports.ReasonOK,
		AppliedDelta:  delta,
		NewScore:      u.Score,
		PositiveTotal: u.PositiveGiven,
		NegativeTotal: u.NegativeGiven,
	}, nil
}

func (s *Service) GetUser(ctx context.Context, chatID, userID int64) (ports.User, bool, error) {
	return s.store.GetUser(ctx, chatID, userID)
}

func (s *Service) Score(ctx context.Context, chatID, userID int64) (int64, error) {
	return s.store.GetScore(ctx, chatID, userID)
}

func (s *Service) Top(ctx context.Context, chatID int64, n int) ([]ports.User, error) {
	return s.store.Top(ctx, chatID, n)
}

func (s *Service) Remember(ctx context.Context, chatID int64, u ports.KnownUser) error {
	return s.store.RememberUser(ctx, chatID, u, s.now())
}

func (s *Service) FindByUsername(ctx context.Context, chatID int64, username string) (ports.KnownUser, bool, error) {
	return s.store.FindByUsername(ctx, chatID, username)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
