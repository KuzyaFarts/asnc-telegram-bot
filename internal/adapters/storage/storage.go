package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KuzyaFarts/asnc-telegram-bot/internal/ports"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

var migrations = []string{
	`ALTER TABLE reputation ADD COLUMN positive_given INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE reputation ADD COLUMN negative_given INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE duels ADD COLUMN kind TEXT NOT NULL DEFAULT 'coin'`,
}

type SQLite struct {
	db *sql.DB
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func NewSQLite(path string) (*SQLite, error) {

	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("mkdir %s: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &SQLite{db: db}, nil
}

func (s *SQLite) Init(ctx context.Context) error {

	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	for _, m := range migrations {
		if _, err := s.db.ExecContext(ctx, m); err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return fmt.Errorf("migration %q: %w", m, err)
		}
	}
	return nil
}

func (s *SQLite) ApplyDelta(ctx context.Context, chatID, userID int64, username, displayName string, delta int, at time.Time) (ports.User, error) {

	var posDelta, negDelta int
	if delta >= 0 {
		posDelta = delta
	} else {
		negDelta = -delta
	}

	const q = `
INSERT INTO reputation (chat_id, user_id, username, display_name, score, positive_given, negative_given, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chat_id, user_id) DO UPDATE SET
    score          = reputation.score + excluded.score,
    positive_given = reputation.positive_given + excluded.positive_given,
    negative_given = reputation.negative_given + excluded.negative_given,
    username       = excluded.username,
    display_name   = excluded.display_name,
    updated_at     = excluded.updated_at
RETURNING score, positive_given, negative_given;
`
	u := ports.User{
		ChatID:      chatID,
		UserID:      userID,
		Username:    username,
		DisplayName: displayName,
	}
	row := s.db.QueryRowContext(ctx, q,
		chatID, userID, username, displayName,
		delta, posDelta, negDelta, at.Unix())
	if err := row.Scan(&u.Score, &u.PositiveGiven, &u.NegativeGiven); err != nil {
		return ports.User{}, fmt.Errorf("apply delta: %w", err)
	}
	return u, nil
}

func (s *SQLite) GetUser(ctx context.Context, chatID, userID int64) (ports.User, bool, error) {

	const q = `
SELECT chat_id, user_id, username, display_name, score, positive_given, negative_given
FROM reputation
WHERE chat_id = ? AND user_id = ?`
	var u ports.User
	err := s.db.QueryRowContext(ctx, q, chatID, userID).
		Scan(&u.ChatID, &u.UserID, &u.Username, &u.DisplayName, &u.Score, &u.PositiveGiven, &u.NegativeGiven)
	if err == sql.ErrNoRows {
		return ports.User{ChatID: chatID, UserID: userID}, false, nil
	}
	if err != nil {
		return ports.User{}, false, fmt.Errorf("get user: %w", err)
	}
	return u, true, nil
}

func (s *SQLite) GetScore(ctx context.Context, chatID, userID int64) (int64, error) {
	const q = `SELECT score FROM reputation WHERE chat_id = ? AND user_id = ?`
	var score int64
	err := s.db.QueryRowContext(ctx, q, chatID, userID).Scan(&score)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get score: %w", err)
	}
	return score, nil
}

func (s *SQLite) Top(ctx context.Context, chatID int64, limit int) ([]ports.User, error) {
	const q = `
SELECT chat_id, user_id, username, display_name, score, positive_given, negative_given
FROM reputation
WHERE chat_id = ?
ORDER BY score DESC, updated_at ASC
LIMIT ?;
`
	rows, err := s.db.QueryContext(ctx, q, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("top query: %w", err)
	}
	defer rows.Close()

	var out []ports.User
	for rows.Next() {
		var u ports.User
		if err := rows.Scan(&u.ChatID, &u.UserID, &u.Username, &u.DisplayName, &u.Score, &u.PositiveGiven, &u.NegativeGiven); err != nil {
			return nil, fmt.Errorf("top scan: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLite) GetLastChange(ctx context.Context, chatID, fromID, toID int64) (time.Time, bool, error) {

	const q = `SELECT last_change_at FROM cooldown WHERE chat_id = ? AND from_user_id = ? AND to_user_id = ?`
	var ts int64
	err := s.db.QueryRowContext(ctx, q, chatID, fromID, toID).Scan(&ts)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("get last change: %w", err)
	}
	return time.Unix(ts, 0), true, nil
}

func (s *SQLite) TouchCooldown(ctx context.Context, chatID, fromID, toID int64, at time.Time) error {
	const q = `
INSERT INTO cooldown (chat_id, from_user_id, to_user_id, last_change_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(chat_id, from_user_id, to_user_id) DO UPDATE SET
    last_change_at = excluded.last_change_at;
`
	if _, err := s.db.ExecContext(ctx, q, chatID, fromID, toID, at.Unix()); err != nil {
		return fmt.Errorf("touch cooldown: %w", err)
	}
	return nil
}

func (s *SQLite) RememberUser(ctx context.Context, chatID int64, u ports.KnownUser, at time.Time) error {

	const q = `
INSERT INTO known_users (chat_id, user_id, username, first_name, last_name, is_bot, seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chat_id, user_id) DO UPDATE SET
    username   = excluded.username,
    first_name = excluded.first_name,
    last_name  = excluded.last_name,
    is_bot     = excluded.is_bot,
    seen_at    = excluded.seen_at;
`
	var isBot int
	if u.IsBot {
		isBot = 1
	}
	if _, err := s.db.ExecContext(ctx, q,
		chatID, u.UserID, u.Username, u.FirstName, u.LastName, isBot, at.Unix()); err != nil {
		return fmt.Errorf("remember user: %w", err)
	}
	return nil
}

func (s *SQLite) FindByUsername(ctx context.Context, chatID int64, username string) (ports.KnownUser, bool, error) {

	const q = `
SELECT user_id, username, first_name, last_name, is_bot
FROM known_users
WHERE chat_id = ? AND username != '' AND lower(username) = lower(?)
ORDER BY seen_at DESC
LIMIT 1;
`
	var (
		u     ports.KnownUser
		isBot int
	)
	err := s.db.QueryRowContext(ctx, q, chatID, username).
		Scan(&u.UserID, &u.Username, &u.FirstName, &u.LastName, &isBot)
	if err == sql.ErrNoRows {
		return ports.KnownUser{}, false, nil
	}
	if err != nil {
		return ports.KnownUser{}, false, fmt.Errorf("find by username: %w", err)
	}
	u.IsBot = isBot != 0
	return u, true, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) GetEconomyAccount(ctx context.Context, chatID, userID int64) (ports.EconomyAccount, bool, error) {
	const q = `
SELECT chat_id, user_id, balance, daily_streak, last_daily_at,
       casino_wins, casino_losses, duel_wins, duel_losses, updated_at
FROM economy_accounts
WHERE chat_id = ? AND user_id = ?;`
	var (
		a           ports.EconomyAccount
		lastDailyAt int64
		updatedAt   int64
	)
	err := s.db.QueryRowContext(ctx, q, chatID, userID).Scan(
		&a.ChatID, &a.UserID, &a.Balance, &a.DailyStreak, &lastDailyAt,
		&a.CasinoWins, &a.CasinoLosses, &a.DuelWins, &a.DuelLosses, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return ports.EconomyAccount{ChatID: chatID, UserID: userID}, false, nil
	}
	if err != nil {
		return ports.EconomyAccount{}, false, fmt.Errorf("get economy account: %w", err)
	}
	a.LastDailyAt = unixTime(lastDailyAt)
	a.UpdatedAt = unixTime(updatedAt)
	return a, true, nil
}

func (s *SQLite) SaveEconomyAccount(ctx context.Context, account ports.EconomyAccount) error {
	if err := saveEconomyAccount(ctx, s.db, account); err != nil {
		return fmt.Errorf("save economy account: %w", err)
	}
	return nil
}

func (s *SQLite) SaveEconomyAccountWithTransaction(ctx context.Context, account ports.EconomyAccount, tr ports.EconomyTransaction) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin economy transaction: %w", err)
	}
	defer tx.Rollback()

	if err := saveEconomyAccount(ctx, tx, account); err != nil {
		return fmt.Errorf("save economy account: %w", err)
	}
	if err := addEconomyTransaction(ctx, tx, tr); err != nil {
		return fmt.Errorf("add transaction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit economy transaction: %w", err)
	}
	return nil
}

func saveEconomyAccount(ctx context.Context, execer sqlExecer, account ports.EconomyAccount) error {
	const q = `
INSERT INTO economy_accounts (
    chat_id, user_id, balance, daily_streak, last_daily_at,
    casino_wins, casino_losses, duel_wins, duel_losses, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chat_id, user_id) DO UPDATE SET
    balance       = excluded.balance,
    daily_streak  = excluded.daily_streak,
    last_daily_at = excluded.last_daily_at,
    casino_wins   = excluded.casino_wins,
    casino_losses = excluded.casino_losses,
    duel_wins     = excluded.duel_wins,
    duel_losses   = excluded.duel_losses,
    updated_at    = excluded.updated_at;
`
	if _, err := execer.ExecContext(ctx, q,
		account.ChatID, account.UserID, account.Balance, account.DailyStreak, unix(account.LastDailyAt),
		account.CasinoWins, account.CasinoLosses, account.DuelWins, account.DuelLosses, unix(account.UpdatedAt),
	); err != nil {
		return err
	}
	return nil
}

func (s *SQLite) TopEconomyAccounts(ctx context.Context, chatID int64, limit int) ([]ports.EconomyProfile, error) {
	const q = `
SELECT
    a.chat_id, a.user_id, a.balance, a.daily_streak, a.last_daily_at,
    a.casino_wins, a.casino_losses, a.duel_wins, a.duel_losses, a.updated_at,
    COALESCE(k.user_id, 0), COALESCE(k.username, ''), COALESCE(k.first_name, ''),
    COALESCE(k.last_name, ''), COALESCE(k.is_bot, 0),
    COALESCE(r.chat_id, a.chat_id), COALESCE(r.user_id, a.user_id),
    COALESCE(r.username, ''), COALESCE(r.display_name, ''), COALESCE(r.score, 0),
    COALESCE(r.positive_given, 0), COALESCE(r.negative_given, 0)
FROM economy_accounts a
LEFT JOIN known_users k ON k.chat_id = a.chat_id AND k.user_id = a.user_id
LEFT JOIN reputation r ON r.chat_id = a.chat_id AND r.user_id = a.user_id
WHERE a.chat_id = ?
ORDER BY a.balance DESC, a.updated_at ASC
LIMIT ?;`
	rows, err := s.db.QueryContext(ctx, q, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("top economy query: %w", err)
	}
	defer rows.Close()

	var out []ports.EconomyProfile
	for rows.Next() {
		var (
			p                         ports.EconomyProfile
			lastDailyAt, updatedAt    int64
			knownUserID, knownUserBot int64
		)
		if err := rows.Scan(
			&p.Account.ChatID, &p.Account.UserID, &p.Account.Balance, &p.Account.DailyStreak, &lastDailyAt,
			&p.Account.CasinoWins, &p.Account.CasinoLosses, &p.Account.DuelWins, &p.Account.DuelLosses, &updatedAt,
			&knownUserID, &p.KnownUser.Username, &p.KnownUser.FirstName, &p.KnownUser.LastName, &knownUserBot,
			&p.Reputation.ChatID, &p.Reputation.UserID, &p.Reputation.Username, &p.Reputation.DisplayName,
			&p.Reputation.Score, &p.Reputation.PositiveGiven, &p.Reputation.NegativeGiven,
		); err != nil {
			return nil, fmt.Errorf("top economy scan: %w", err)
		}
		p.Account.LastDailyAt = unixTime(lastDailyAt)
		p.Account.UpdatedAt = unixTime(updatedAt)
		p.KnownUser.UserID = knownUserID
		p.KnownUser.IsBot = knownUserBot != 0
		p.HasKnownUser = knownUserID != 0
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLite) AddTransaction(ctx context.Context, chatID, userID int64, delta int64, reason string, refID int64, at time.Time) error {
	if err := addEconomyTransaction(ctx, s.db, ports.EconomyTransaction{
		ChatID:    chatID,
		UserID:    userID,
		Delta:     delta,
		Reason:    reason,
		RefID:     refID,
		CreatedAt: at,
	}); err != nil {
		return fmt.Errorf("add transaction: %w", err)
	}
	return nil
}

func addEconomyTransaction(ctx context.Context, execer sqlExecer, tr ports.EconomyTransaction) error {
	const q = `
INSERT INTO economy_transactions (chat_id, user_id, delta, reason, ref_id, created_at)
VALUES (?, ?, ?, ?, ?, ?);`
	if _, err := execer.ExecContext(ctx, q, tr.ChatID, tr.UserID, tr.Delta, tr.Reason, tr.RefID, unix(tr.CreatedAt)); err != nil {
		return err
	}
	return nil
}

func (s *SQLite) CreateDuel(ctx context.Context, duel ports.Duel) (ports.Duel, error) {
	const q = `
INSERT INTO duels (chat_id, challenger_id, opponent_id, kind, stake, status, winner_id, created_at, resolved_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;`
	err := s.db.QueryRowContext(ctx, q,
		duel.ChatID, duel.ChallengerID, duel.OpponentID, string(duel.Kind), duel.Stake, string(duel.Status),
		duel.WinnerID, unix(duel.CreatedAt), unix(duel.ResolvedAt),
	).Scan(&duel.ID)
	if err != nil {
		return ports.Duel{}, fmt.Errorf("create duel: %w", err)
	}
	return duel, nil
}

func (s *SQLite) GetPendingDuel(ctx context.Context, chatID, challengerID, opponentID int64, kind ports.DuelKind) (ports.Duel, bool, error) {
	const q = `
SELECT id, chat_id, challenger_id, opponent_id, kind, stake, status, winner_id, created_at, resolved_at
FROM duels
WHERE chat_id = ? AND challenger_id = ? AND opponent_id = ? AND kind = ? AND status = ?
ORDER BY created_at DESC
LIMIT 1;`
	return s.getDuel(ctx, q, chatID, challengerID, opponentID, string(kind), string(ports.DuelStatusPending))
}

func (s *SQLite) CompleteDuel(ctx context.Context, duel ports.Duel) error {
	if err := completeDuel(ctx, s.db, duel); err != nil {
		return fmt.Errorf("complete duel: %w", err)
	}
	return nil
}

func (s *SQLite) CompleteDuelWithAccounts(ctx context.Context, duel ports.Duel, accounts []ports.EconomyAccount, txs []ports.EconomyTransaction) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete duel: %w", err)
	}
	defer tx.Rollback()

	for _, account := range accounts {
		if err := saveEconomyAccount(ctx, tx, account); err != nil {
			return fmt.Errorf("save duel account: %w", err)
		}
	}
	if err := completeDuel(ctx, tx, duel); err != nil {
		return fmt.Errorf("complete duel: %w", err)
	}
	for _, tr := range txs {
		if err := addEconomyTransaction(ctx, tx, tr); err != nil {
			return fmt.Errorf("add duel transaction: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit complete duel: %w", err)
	}
	return nil
}

func (s *SQLite) CancelDuel(ctx context.Context, duel ports.Duel) error {
	const q = `
UPDATE duels
SET status = ?, resolved_at = ?
WHERE id = ? AND status = ?;`
	res, err := s.db.ExecContext(ctx, q,
		string(duel.Status), unix(duel.ResolvedAt), duel.ID, string(ports.DuelStatusPending),
	)
	if err != nil {
		return fmt.Errorf("cancel duel: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("cancel duel rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("cancel duel: duel %d is not pending", duel.ID)
	}
	return nil
}

func completeDuel(ctx context.Context, execer sqlExecer, duel ports.Duel) error {
	const q = `
UPDATE duels
SET status = ?, winner_id = ?, resolved_at = ?
WHERE id = ? AND status = ?;`
	res, err := execer.ExecContext(ctx, q,
		string(duel.Status), duel.WinnerID, unix(duel.ResolvedAt), duel.ID, string(ports.DuelStatusPending),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("duel %d is not pending", duel.ID)
	}
	return nil
}

func (s *SQLite) getDuel(ctx context.Context, q string, args ...any) (ports.Duel, bool, error) {
	var (
		d                     ports.Duel
		kind                  string
		status                string
		createdAt, resolvedAt int64
	)
	err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&d.ID, &d.ChatID, &d.ChallengerID, &d.OpponentID, &kind, &d.Stake,
		&status, &d.WinnerID, &createdAt, &resolvedAt,
	)
	if err == sql.ErrNoRows {
		return ports.Duel{}, false, nil
	}
	if err != nil {
		return ports.Duel{}, false, fmt.Errorf("get duel: %w", err)
	}
	d.Kind = ports.DuelKind(kind)
	d.Status = ports.DuelStatus(status)
	d.CreatedAt = unixTime(createdAt)
	d.ResolvedAt = unixTime(resolvedAt)
	return d, true, nil
}

func (s *SQLite) SaveMessage(ctx context.Context, msg ports.ChatMessage) error {
	const q = `
INSERT INTO chat_messages (chat_id, user_id, first_name, username, text, created_at)
VALUES (?, ?, ?, ?, ?, ?);`
	if _, err := s.db.ExecContext(ctx, q,
		msg.ChatID, msg.UserID, msg.FirstName, msg.Username, msg.Text, msg.CreatedAt.Unix()); err != nil {
		return fmt.Errorf("save message: %w", err)
	}
	return nil
}

func (s *SQLite) GetTodayMessages(ctx context.Context, chatID int64) ([]ports.ChatMessage, error) {
	now := time.Now().UTC()
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	const q = `
SELECT id, chat_id, user_id, first_name, username, text, created_at
FROM chat_messages
WHERE chat_id = ? AND created_at >= ?
ORDER BY created_at ASC
LIMIT 300;`
	rows, err := s.db.QueryContext(ctx, q, chatID, todayMidnight.Unix())
	if err != nil {
		return nil, fmt.Errorf("get today messages: %w", err)
	}
	defer rows.Close()

	var out []ports.ChatMessage
	for rows.Next() {
		var (
			m  ports.ChatMessage
			ts int64
		)
		if err := rows.Scan(&m.ID, &m.ChatID, &m.UserID, &m.FirstName, &m.Username, &m.Text, &ts); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.CreatedAt = time.Unix(ts, 0)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func unixTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}
