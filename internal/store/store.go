package store

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

//go:embed schema.sql
var schemaSQL string

const currentVersion = 1

// Store keeps all data in a single SQLite database inside the data dir
// (guitar-coach.db). Every read hits the database, so a web UI running in one
// process always sees sessions recorded by another process (no stale cache).
// On first open in a dir that still has the legacy JSON store, the data is
// imported into SQLite and the JSON files are removed.
type Store struct {
	db  *sql.DB
	dir string
}

// legacy doc, kept for parsing the pre-SQLite JSON files during migration.
type doc struct {
	Version int             `json:"version"`
	Data    json.RawMessage `json:"data"`
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	s := &Store{dir: dir}
	db, err := sql.Open("sqlite", filepath.Join(dir, "guitar-coach.db"))
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s.db = db

	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configuring sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}
	if err := s.migrateFromJSON(dir); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating legacy JSON data: %w", err)
	}
	return s, nil
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) close() error {
	return s.db.Close()
}

// migrateFromJSON imports a legacy exercises.json/sessions.json pair into
// SQLite the first time a database file is created, then removes the JSON
// files. If the database already holds sessions, the import is skipped.
func (s *Store) migrateFromJSON(dir string) error {
	exJSON, exErr := os.ReadFile(filepath.Join(dir, "exercises.json"))
	sessJSON, sessErr := os.ReadFile(filepath.Join(dir, "sessions.json"))
	haveEx := exErr == nil && len(bytes.TrimSpace(exJSON)) > 0
	haveSess := sessErr == nil && len(bytes.TrimSpace(sessJSON)) > 0
	if !haveEx && !haveSess {
		return nil
	}

	// Already migrated: a database with any exercises/sessions is authoritative
	// (an old JSON leftover, e.g. from a concurrent first run, must not re-import).
	var exCount, sessCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM exercises`).Scan(&exCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessCount)
	if exCount > 0 || sessCount > 0 {
		return nil
	}

	var exercises []model.Exercise
	if haveEx {
		if err := parseLegacy(exJSON, &exercises); err != nil {
			return fmt.Errorf("exercises.json: %w", err)
		}
	}
	var sessions []model.Session
	if haveSess {
		if err := parseLegacy(sessJSON, &sessions); err != nil {
			return fmt.Errorf("sessions.json: %w", err)
		}
	}

	if err := s.importExercises(exercises); err != nil {
		return err
	}
	if err := s.importSessions(sessions); err != nil {
		return err
	}
	os.Remove(filepath.Join(dir, "exercises.json"))
	os.Remove(filepath.Join(dir, "sessions.json"))
	return nil
}

// parseLegacy decodes a legacy JSON file: either the raw array form or the
// {version, data} envelope. Never-written files parse the array form.
func parseLegacy(data []byte, dst any) error {
	var d doc
	if err := json.Unmarshal(data, &d); err != nil || d.Data == nil {
		return json.Unmarshal(data, dst)
	}
	if d.Version > currentVersion {
		return fmt.Errorf("data version %d is newer than supported version %d", d.Version, currentVersion)
	}
	return json.Unmarshal(d.Data, dst)
}

func (s *Store) ListExercises() []model.Exercise {
	rows, err := s.db.Query(`SELECT id, name, topic, description, created_at FROM exercises ORDER BY rowid`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []model.Exercise
	for rows.Next() {
		var e model.Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.Topic, &e.Description, &e.CreatedAt); err != nil {
			return nil
		}
		out = append(out, e)
	}
	return out
}

func (s *Store) FindExerciseByName(name string) *model.Exercise {
	var e model.Exercise
	err := s.db.QueryRow(`SELECT id, name, topic, description, created_at FROM exercises WHERE name = ?`, name).
		Scan(&e.ID, &e.Name, &e.Topic, &e.Description, &e.CreatedAt)
	if err != nil {
		return nil
	}
	return &e
}

func (s *Store) FindExerciseByID(id string) *model.Exercise {
	var e model.Exercise
	err := s.db.QueryRow(`SELECT id, name, topic, description, created_at FROM exercises WHERE id = ?`, id).
		Scan(&e.ID, &e.Name, &e.Topic, &e.Description, &e.CreatedAt)
	if err != nil {
		return nil
	}
	return &e
}

func (s *Store) AddExercise(ex model.Exercise) error {
	_, err := s.db.Exec(`INSERT INTO exercises (id, name, topic, description, created_at) VALUES (?, ?, ?, ?, ?)`,
		ex.ID, ex.Name, ex.Topic, ex.Description, ex.CreatedAt)
	return err
}

func (s *Store) RemoveExerciseByID(id string) bool {
	res, err := s.db.Exec(`DELETE FROM exercises WHERE id = ?`, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *Store) UpdateExercise(ex model.Exercise) error {
	res, err := s.db.Exec(`UPDATE exercises SET name = ?, topic = ?, description = ?, created_at = ? WHERE id = ?`,
		ex.Name, ex.Topic, ex.Description, ex.CreatedAt, ex.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("exercise %q not found", ex.ID)
	}
	return nil
}

func (s *Store) ListSessions() []model.Session {
	out := s.listSessions()
	if out == nil {
		return nil
	}
	entries := s.listEntries()
	for i := range out {
		out[i].Entries = entries[out[i].ID]
		if out[i].Entries == nil {
			out[i].Entries = []model.Entry{}
		}
	}
	return out
}

func (s *Store) listSessions() []model.Session {
	rows, err := s.db.Query(`SELECT id, started_at, ended_at, order_json,
		exercises_per_round, duration_sec, rest_sec, break_sec FROM sessions ORDER BY rowid`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var sess model.Session
		var orderJSON string
		if err := rows.Scan(&sess.ID, &sess.StartedAt, &sess.EndedAt, &orderJSON,
			&sess.Config.ExercisesPerRound, &sess.Config.DurationSec, &sess.Config.RestSec, &sess.Config.BreakSec); err != nil {
			return nil
		}
		if err := json.Unmarshal([]byte(orderJSON), &sess.Order); err != nil {
			return nil
		}
		out = append(out, sess)
	}
	return out
}

func (s *Store) listEntries() map[string][]model.Entry {
	rows, err := s.db.Query(`SELECT session_id, position, exercise_id, name, round, sequence,
		start_bpm, end_bpm, notes, started_at, finished_at FROM entries ORDER BY session_id, position`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string][]model.Entry{}
	for rows.Next() {
		var (
			sessionID string
			position  int
			e         model.Entry
		)
		if err := rows.Scan(&sessionID, &position, &e.ExerciseID, &e.Name, &e.Round, &e.Sequence,
			&e.StartBPM, &e.EndBPM, &e.Notes, &e.StartedAt, &e.FinishedAt); err != nil {
			return nil
		}
		ls := out[sessionID]
		for len(ls) <= position {
			ls = append(ls, model.Entry{})
		}
		ls[position] = e
		out[sessionID] = ls
	}
	return out
}

func (s *Store) SessionByID(id string) *model.Session {
	row := s.db.QueryRow(`SELECT id, started_at, ended_at, order_json,
		exercises_per_round, duration_sec, rest_sec, break_sec FROM sessions WHERE id = ?`, id)
	var sess model.Session
	var orderJSON string
	err := row.Scan(&sess.ID, &sess.StartedAt, &sess.EndedAt, &orderJSON,
		&sess.Config.ExercisesPerRound, &sess.Config.DurationSec, &sess.Config.RestSec, &sess.Config.BreakSec)
	if err != nil {
		return nil
	}
	sess.Order = []string{}
	if err := json.Unmarshal([]byte(orderJSON), &sess.Order); err != nil {
		return nil
	}
	sess.Entries = s.entriesFor(id)
	return &sess
}

func (s *Store) entriesFor(sessionID string) []model.Entry {
	rows, err := s.db.Query(`SELECT position, exercise_id, name, round, sequence,
		start_bpm, end_bpm, notes, started_at, finished_at FROM entries WHERE session_id = ? ORDER BY position`, sessionID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []model.Entry
	for rows.Next() {
		var (
			position int
			e        model.Entry
		)
		if err := rows.Scan(&position, &e.ExerciseID, &e.Name, &e.Round, &e.Sequence,
			&e.StartBPM, &e.EndBPM, &e.Notes, &e.StartedAt, &e.FinishedAt); err != nil {
			return nil
		}
		for len(out) <= position {
			out = append(out, model.Entry{})
		}
		out[position] = e
	}
	return out
}

func (s *Store) AddSession(ss model.Session) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertSession(tx, ss); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpdateSession(ss model.Session) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE sessions SET started_at = ?, ended_at = ?, order_json = ?,
		exercises_per_round = ?, duration_sec = ?, rest_sec = ?, break_sec = ? WHERE id = ?`,
		ss.StartedAt, ss.EndedAt, mustOrderJSON(ss.Order),
		ss.Config.ExercisesPerRound, ss.Config.DurationSec, ss.Config.RestSec, ss.Config.BreakSec, ss.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %q not found", ss.ID)
	}
	if _, err := tx.Exec(`DELETE FROM entries WHERE session_id = ?`, ss.ID); err != nil {
		return err
	}
	if err := insertEntries(tx, ss.ID, ss.Entries); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteSession(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM entries WHERE session_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("session %q not found", id)
	}
	return tx.Commit()
}

func (s *Store) importExercises(exs []model.Exercise) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range exs {
		if _, err := tx.Exec(`INSERT INTO exercises (id, name, topic, description, created_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`, e.ID, e.Name, e.Topic, e.Description, e.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) importSessions(sess []model.Session) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, ss := range sess {
		if err := insertSession(tx, ss); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertSession(tx *sql.Tx, ss model.Session) error {
	_, err := tx.Exec(`INSERT INTO sessions (id, started_at, ended_at, order_json,
			exercises_per_round, duration_sec, rest_sec, break_sec) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
		ss.ID, ss.StartedAt, ss.EndedAt, mustOrderJSON(ss.Order),
		ss.Config.ExercisesPerRound, ss.Config.DurationSec, ss.Config.RestSec, ss.Config.BreakSec)
	if err != nil {
		return err
	}
	return insertEntries(tx, ss.ID, ss.Entries)
}

func insertEntries(tx *sql.Tx, sessionID string, entries []model.Entry) error {
	stmt, err := tx.Prepare(`INSERT INTO entries (session_id, position, exercise_id, name, round, sequence,
		start_bpm, end_bpm, notes, started_at, finished_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, e := range entries {
		if _, err := stmt.Exec(sessionID, i, e.ExerciseID, e.Name, e.Round, e.Sequence,
			e.StartBPM, e.EndBPM, e.Notes, e.StartedAt, e.FinishedAt); err != nil {
			return err
		}
	}
	return nil
}

func mustOrderJSON(order []string) string {
	if order == nil {
		order = []string{}
	}
	data, _ := json.Marshal(order)
	return string(data)
}
