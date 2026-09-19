package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

// TestMigrateAddsResumeColumns builds a pre-resume (v1) database and
// verifies Open() adds the resume columns and keeps existing data readable.
func TestMigrateAddsResumeColumns(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "guitar-coach.db"))
	if err != nil {
		t.Fatal(err)
	}
	v1 := `PRAGMA user_version = 1;
CREATE TABLE exercises (id TEXT PRIMARY KEY, name TEXT NOT NULL, topic TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL);
CREATE TABLE sessions (id TEXT PRIMARY KEY, started_at TEXT NOT NULL, ended_at TEXT NOT NULL DEFAULT '', order_json TEXT NOT NULL DEFAULT '[]', exercises_per_round INTEGER NOT NULL, duration_sec INTEGER NOT NULL, rest_sec INTEGER NOT NULL, break_sec INTEGER NOT NULL);
CREATE TABLE entries (session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE, position INTEGER NOT NULL, exercise_id TEXT NOT NULL, name TEXT NOT NULL, round INTEGER NOT NULL, sequence INTEGER NOT NULL, start_bpm INTEGER NOT NULL DEFAULT 0, end_bpm INTEGER NOT NULL DEFAULT 0, notes TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL DEFAULT '', finished_at TEXT NOT NULL DEFAULT '', PRIMARY KEY (session_id, position));
CREATE INDEX idx_entries_exercise ON entries (exercise_id);
INSERT INTO exercises VALUES ('ex_1','Warmup','','','2026-01-01T00:00:00Z');
INSERT INTO exercises VALUES ('ex_2','Ear training','','','2026-01-01T00:00:00Z');
INSERT INTO sessions VALUES ('sess_1','2026-01-02T00:00:00Z','','["ex_1","ex_2"]',2,60,30,300);
INSERT INTO entries VALUES ('sess_1',0,'ex_1','Warmup',1,1,60,60,'','','');`
	if _, err := db.Exec(v1); err != nil {
		db.Close()
		t.Fatalf("building v1 db: %v", err)
	}
	db.Close()

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open on v1 db: %v", err)
	}
	defer s.close()

	sess := s.SessionByID("sess_1")
	if sess == nil || len(sess.Entries) != 1 || sess.Entries[0].Name != "Warmup" {
		t.Fatalf("v1 data not loadable after migration: %+v", sess)
	}

	updated := model.Session{
		ID:          sess.ID,
		StartedAt:   sess.StartedAt,
		EndedAt:     sess.EndedAt,
		Config:      sess.Config,
		Order:       sess.Order,
		Entries:     sess.Entries,
		ResumeRound: 1,
		ResumeOrder: []string{"ex_2", "ex_1"},
		ResumeSeq:   0,
	}
	if err := s.UpdateSession(updated); err != nil {
		t.Fatalf("UpdateSession with resume point: %v", err)
	}
	loaded := s.SessionByID("sess_1")
	if loaded.ResumeRound != 1 || loaded.ResumeSeq != 0 || len(loaded.ResumeOrder) != 2 {
		t.Fatalf("resume point not persisted: %+v", loaded)
	}
}
