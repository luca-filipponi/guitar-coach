-- guitar-coach database schema (v1)
-- One SQLite file per data dir (guitar-coach.db). WAL journal is enabled at
-- open so a long-running `serve` and a separate `start` process can read and
-- write concurrently without stale views.

PRAGMA user_version = 1;

CREATE TABLE IF NOT EXISTS exercises (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    topic       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id                  TEXT PRIMARY KEY,
    started_at          TEXT NOT NULL,
    ended_at            TEXT NOT NULL DEFAULT '',
    order_json          TEXT NOT NULL DEFAULT '[]',
    exercises_per_round INTEGER NOT NULL,
    duration_sec        INTEGER NOT NULL,
    rest_sec            INTEGER NOT NULL,
    break_sec           INTEGER NOT NULL
);

-- Entries are addressed positionally within a session (the ledger order shown
-- by `show`). `position` is the 0-based index inside the session's entry list.
CREATE TABLE IF NOT EXISTS entries (
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    exercise_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    round       INTEGER NOT NULL,
    sequence    INTEGER NOT NULL,
    start_bpm   INTEGER NOT NULL DEFAULT 0,
    end_bpm     INTEGER NOT NULL DEFAULT 0,
    notes       TEXT NOT NULL DEFAULT '',
    started_at  TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (session_id, position)
);

CREATE INDEX IF NOT EXISTS idx_entries_exercise ON entries (exercise_id);