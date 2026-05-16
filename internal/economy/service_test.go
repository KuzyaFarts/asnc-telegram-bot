package economy

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

func TestClaimDailyCreatesAccountAndBlocksSecondClaim(t *testing.T) {
	ctx := context.Background()
	repo := newFakeEconomyRepo()
	svc := New(repo, fakeReputationUseCase{}, DefaultSettings())
	svc.now = func() time.Time { return time.Unix(1000, 0) }
	svc.rnd = rand.New(rand.NewSource(1))

	user := ports.KnownUser{UserID: 42}
	res, err := svc.ClaimDaily(ctx, 1, user)
	if err != nil {
		t.Fatalf("ClaimDaily first call error = %v", err)
	}
	if !res.Claimed {
		t.Fatalf("ClaimDaily first call Claimed = false, want true")
	}
	if res.Account.Balance <= 0 {
		t.Fatalf("ClaimDaily balance = %d, want positive", res.Account.Balance)
	}

	res, err = svc.ClaimDaily(ctx, 1, user)
	if err != ErrDailyAlreadyClaimed {
		t.Fatalf("ClaimDaily second call error = %v, want ErrDailyAlreadyClaimed", err)
	}
	if res.NextClaimIn <= 0 {
		t.Fatalf("ClaimDaily second call NextClaimIn = %s, want positive", res.NextClaimIn)
	}
}

func TestRollRejectsInsufficientBalance(t *testing.T) {
	ctx := context.Background()
	repo := newFakeEconomyRepo()
	svc := New(repo, fakeReputationUseCase{}, DefaultSettings())

	res, err := svc.Roll(ctx, 1, ports.KnownUser{UserID: 42}, 10)
	if err != ErrInsufficientBalance {
		t.Fatalf("Roll error = %v, want ErrInsufficientBalance", err)
	}
	if res.Account.Balance != 0 {
		t.Fatalf("Roll account balance = %d, want 0", res.Account.Balance)
	}
}

func TestAwardAddsCoins(t *testing.T) {
	ctx := context.Background()
	repo := newFakeEconomyRepo()
	svc := New(repo, fakeReputationUseCase{}, DefaultSettings())

	account, err := svc.Award(ctx, 1, ports.KnownUser{UserID: 42}, 3, "positive_rep", 99)
	if err != nil {
		t.Fatalf("Award error = %v", err)
	}
	if account.Balance != 3 {
		t.Fatalf("Award balance = %d, want 3", account.Balance)
	}
}

type fakeEconomyRepo struct {
	accounts map[[2]int64]ports.EconomyAccount
}

func newFakeEconomyRepo() *fakeEconomyRepo {
	return &fakeEconomyRepo{accounts: make(map[[2]int64]ports.EconomyAccount)}
}

func (r *fakeEconomyRepo) GetEconomyAccount(_ context.Context, chatID, userID int64) (ports.EconomyAccount, bool, error) {
	account, ok := r.accounts[[2]int64{chatID, userID}]
	return account, ok, nil
}

func (r *fakeEconomyRepo) SaveEconomyAccount(_ context.Context, account ports.EconomyAccount) error {
	r.accounts[[2]int64{account.ChatID, account.UserID}] = account
	return nil
}

func (r *fakeEconomyRepo) SaveEconomyAccountWithTransaction(_ context.Context, account ports.EconomyAccount, _ ports.EconomyTransaction) error {
	r.accounts[[2]int64{account.ChatID, account.UserID}] = account
	return nil
}

func (r *fakeEconomyRepo) TopEconomyAccounts(context.Context, int64, int) ([]ports.EconomyProfile, error) {
	return nil, nil
}

func (r *fakeEconomyRepo) AddTransaction(context.Context, int64, int64, int64, string, int64, time.Time) error {
	return nil
}

func (r *fakeEconomyRepo) CreateDuel(_ context.Context, duel ports.Duel) (ports.Duel, error) {
	duel.ID = 1
	return duel, nil
}

func (r *fakeEconomyRepo) GetPendingDuel(context.Context, int64, int64, int64, ports.DuelKind) (ports.Duel, bool, error) {
	return ports.Duel{}, false, nil
}

func (r *fakeEconomyRepo) CompleteDuel(context.Context, ports.Duel) error {
	return nil
}

func (r *fakeEconomyRepo) CompleteDuelWithAccounts(_ context.Context, _ ports.Duel, accounts []ports.EconomyAccount, _ []ports.EconomyTransaction) error {
	for _, account := range accounts {
		r.accounts[[2]int64{account.ChatID, account.UserID}] = account
	}
	return nil
}

func (r *fakeEconomyRepo) CancelDuel(context.Context, ports.Duel) error {
	return nil
}

type fakeReputationUseCase struct{}

func (fakeReputationUseCase) Apply(context.Context, int64, ports.Actor, ports.Actor, int) (*ports.ApplyResult, error) {
	return nil, nil
}

func (fakeReputationUseCase) GetUser(_ context.Context, chatID, userID int64) (ports.User, bool, error) {
	return ports.User{ChatID: chatID, UserID: userID}, false, nil
}

func (fakeReputationUseCase) Score(context.Context, int64, int64) (int64, error) {
	return 0, nil
}

func (fakeReputationUseCase) Top(context.Context, int64, int) ([]ports.User, error) {
	return nil, nil
}

func (fakeReputationUseCase) Remember(context.Context, int64, ports.KnownUser) error {
	return nil
}

func (fakeReputationUseCase) FindByUsername(context.Context, int64, string) (ports.KnownUser, bool, error) {
	return ports.KnownUser{}, false, nil
}
