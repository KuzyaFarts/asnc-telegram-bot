PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS reputation (
    chat_id        INTEGER NOT NULL,
    user_id        INTEGER NOT NULL,
    username       TEXT    NOT NULL DEFAULT '',
    display_name   TEXT    NOT NULL DEFAULT '',
    score          INTEGER NOT NULL DEFAULT 0,
    positive_given INTEGER NOT NULL DEFAULT 0,
    negative_given INTEGER NOT NULL DEFAULT 0,
    updated_at     INTEGER NOT NULL,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_reputation_chat_score
    ON reputation (chat_id, score DESC);

CREATE TABLE IF NOT EXISTS cooldown (
    chat_id        INTEGER NOT NULL,
    from_user_id   INTEGER NOT NULL,
    to_user_id     INTEGER NOT NULL,
    last_change_at INTEGER NOT NULL,
    PRIMARY KEY (chat_id, from_user_id, to_user_id)
);

CREATE TABLE IF NOT EXISTS known_users (
    chat_id    INTEGER NOT NULL,
    user_id    INTEGER NOT NULL,
    username   TEXT    NOT NULL DEFAULT '',
    first_name TEXT    NOT NULL DEFAULT '',
    last_name  TEXT    NOT NULL DEFAULT '',
    is_bot     INTEGER NOT NULL DEFAULT 0,
    seen_at    INTEGER NOT NULL,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_known_users_chat_username
    ON known_users (chat_id, username);

CREATE TABLE IF NOT EXISTS economy_accounts (
    chat_id        INTEGER NOT NULL,
    user_id        INTEGER NOT NULL,
    balance        INTEGER NOT NULL DEFAULT 0,
    daily_streak   INTEGER NOT NULL DEFAULT 0,
    last_daily_at  INTEGER NOT NULL DEFAULT 0,
    casino_wins    INTEGER NOT NULL DEFAULT 0,
    casino_losses  INTEGER NOT NULL DEFAULT 0,
    duel_wins      INTEGER NOT NULL DEFAULT 0,
    duel_losses    INTEGER NOT NULL DEFAULT 0,
    updated_at     INTEGER NOT NULL,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_economy_accounts_chat_balance
    ON economy_accounts (chat_id, balance DESC);

CREATE TABLE IF NOT EXISTS economy_transactions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id    INTEGER NOT NULL,
    user_id    INTEGER NOT NULL,
    delta      INTEGER NOT NULL,
    reason     TEXT    NOT NULL,
    ref_id     INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_economy_transactions_user
    ON economy_transactions (chat_id, user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS duels (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id       INTEGER NOT NULL,
    challenger_id INTEGER NOT NULL,
    opponent_id   INTEGER NOT NULL,
    kind          TEXT    NOT NULL DEFAULT 'coin',
    stake         INTEGER NOT NULL,
    status        TEXT    NOT NULL,
    winner_id     INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL,
    resolved_at   INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_duels_pending
    ON duels (chat_id, challenger_id, opponent_id, kind, status, created_at DESC);
