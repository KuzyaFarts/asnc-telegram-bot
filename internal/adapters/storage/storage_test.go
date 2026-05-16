package storage

import (
	"context"
	"testing"
	"time"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"
)

func TestEconomyAccountWithTransaction(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)

	account := ports.EconomyAccount{
		ChatID:    1,
		UserID:    42,
		Balance:   7,
		UpdatedAt: time.Unix(100, 0),
	}
	err := store.SaveEconomyAccountWithTransaction(ctx, account, ports.EconomyTransaction{
		ChatID:    1,
		UserID:    42,
		Delta:     7,
		Reason:    "test",
		CreatedAt: time.Unix(100, 0),
	})
	if err != nil {
		t.Fatalf("SaveEconomyAccountWithTransaction error = %v", err)
	}

	got, ok, err := store.GetEconomyAccount(ctx, 1, 42)
	if err != nil {
		t.Fatalf("GetEconomyAccount error = %v", err)
	}
	if !ok {
		t.Fatalf("GetEconomyAccount ok = false, want true")
	}
	if got.Balance != 7 {
		t.Fatalf("GetEconomyAccount balance = %d, want 7", got.Balance)
	}
}

func TestDuelLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)

	duel, err := store.CreateDuel(ctx, ports.Duel{
		ChatID:       1,
		ChallengerID: 10,
		OpponentID:   20,
		Kind:         ports.DuelKindCoin,
		Stake:        5,
		Status:       ports.DuelStatusPending,
		CreatedAt:    time.Unix(100, 0),
	})
	if err != nil {
		t.Fatalf("CreateDuel error = %v", err)
	}

	got, ok, err := store.GetPendingDuel(ctx, 1, 10, 20, ports.DuelKindCoin)
	if err != nil {
		t.Fatalf("GetPendingDuel error = %v", err)
	}
	if !ok {
		t.Fatalf("GetPendingDuel ok = false, want true")
	}
	if got.ID != duel.ID {
		t.Fatalf("GetPendingDuel ID = %d, want %d", got.ID, duel.ID)
	}

	duel.Status = ports.DuelStatusCanceled
	duel.ResolvedAt = time.Unix(200, 0)
	if err := store.CancelDuel(ctx, duel); err != nil {
		t.Fatalf("CancelDuel error = %v", err)
	}
	if _, ok, err := store.GetPendingDuel(ctx, 1, 10, 20, ports.DuelKindCoin); err != nil {
		t.Fatalf("GetPendingDuel after cancel error = %v", err)
	} else if ok {
		t.Fatalf("GetPendingDuel after cancel ok = true, want false")
	}
}

func TestDuelKindsDoNotOverlap(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)

	_, err := store.CreateDuel(ctx, ports.Duel{
		ChatID:       1,
		ChallengerID: 10,
		OpponentID:   20,
		Kind:         ports.DuelKindMute,
		Stake:        3,
		Status:       ports.DuelStatusPending,
		CreatedAt:    time.Unix(100, 0),
	})
	if err != nil {
		t.Fatalf("CreateDuel mute error = %v", err)
	}
	if _, ok, err := store.GetPendingDuel(ctx, 1, 10, 20, ports.DuelKindCoin); err != nil {
		t.Fatalf("GetPendingDuel coin error = %v", err)
	} else if ok {
		t.Fatalf("GetPendingDuel coin ok = true, want false")
	}
	if got, ok, err := store.GetPendingDuel(ctx, 1, 10, 20, ports.DuelKindMute); err != nil {
		t.Fatalf("GetPendingDuel mute error = %v", err)
	} else if !ok {
		t.Fatalf("GetPendingDuel mute ok = false, want true")
	} else if got.Stake != 3 {
		t.Fatalf("GetPendingDuel mute stake = %d, want 3", got.Stake)
	}
}

func newTestStore(t *testing.T, ctx context.Context) *SQLite {
	t.Helper()
	store, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close error = %v", err)
		}
	})
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init error = %v", err)
	}
	return store
}
