-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cefr_self   TEXT,
    theta       REAL    NOT NULL DEFAULT 0,
    sem         REAL    NOT NULL DEFAULT 1,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS words (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    lemma       TEXT NOT NULL UNIQUE,
    cefr        TEXT,
    freq_rank   INTEGER,
    pos         TEXT,
    gloss_ja    TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_words_cefr ON words(cefr);
CREATE INDEX IF NOT EXISTS idx_words_freq ON words(freq_rank);

CREATE TABLE IF NOT EXISTS word_vec (
    word_id     INTEGER PRIMARY KEY,
    embedding   BLOB    NOT NULL,
    model       TEXT    NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (word_id) REFERENCES words(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS examples (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    word_id     INTEGER NOT NULL,
    sentence    TEXT    NOT NULL,
    source      TEXT,
    license     TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (word_id) REFERENCES words(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_examples_word ON examples(word_id);

CREATE TABLE IF NOT EXISTS attempts (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL,
    word_id             INTEGER NOT NULL,
    content_hash        TEXT,
    correct             INTEGER NOT NULL,
    latency_ms          INTEGER NOT NULL DEFAULT 0,
    quality             INTEGER,
    ease                REAL,
    interval_days       REAL,
    reps                INTEGER,
    lapses              INTEGER,
    next_review_at      TIMESTAMP,
    fsrs_stability      REAL,
    fsrs_difficulty     REAL,
    fsrs_last_review    TIMESTAMP,
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (word_id) REFERENCES words(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_attempts_user ON attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_attempts_due  ON attempts(user_id, next_review_at);

CREATE TABLE IF NOT EXISTS generated_content (
    cache_key   TEXT PRIMARY KEY,
    model       TEXT NOT NULL,
    schema_ver  TEXT NOT NULL,
    prompt_ver  TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    hit_count   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS placement_items (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    prompt      TEXT NOT NULL,
    choices_json TEXT NOT NULL,
    answer_index INTEGER NOT NULL,
    irt_a       REAL NOT NULL DEFAULT 1.0,
    irt_b       REAL NOT NULL DEFAULT 0.0,
    cefr        TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS friction_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER,
    kind        TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_friction_user ON friction_log(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS friction_log;
DROP TABLE IF EXISTS placement_items;
DROP TABLE IF EXISTS generated_content;
DROP TABLE IF EXISTS attempts;
DROP TABLE IF EXISTS examples;
DROP TABLE IF EXISTS word_vec;
DROP TABLE IF EXISTS words;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
