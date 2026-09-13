package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

type Store struct {
	mu            sync.Mutex
	exercisesPath string
	sessionsPath  string
	Exercises     []model.Exercise
	Sessions      []model.Session
}

const currentVersion = 1

type doc struct {
	Version int             `json:"version"`
	Data    json.RawMessage `json:"data"`
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	s := &Store{
		exercisesPath: filepath.Join(dir, "exercises.json"),
		sessionsPath:  filepath.Join(dir, "sessions.json"),
	}
	if err := s.load(&s.Exercises, s.exercisesPath); err != nil {
		return nil, err
	}
	if err := s.load(&s.Sessions, s.sessionsPath); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Dir() string {
	return filepath.Dir(s.exercisesPath)
}

func (s *Store) lock() (unlock func()) {
	s.mu.Lock()
	return s.mu.Unlock
}

func (s *Store) load(dst any, path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	var d doc
	if err := json.Unmarshal(data, &d); err != nil || d.Data == nil {
		if err := json.Unmarshal(data, dst); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return writeVersioned(path, dst)
	}
	if d.Version > currentVersion {
		return fmt.Errorf("%s: data version %d is newer than supported version %d", path, d.Version, currentVersion)
	}
	raw, err := migrate(d.Version, currentVersion, d.Data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return json.Unmarshal(raw, dst)
}

func migrate(from, to int, data json.RawMessage) (json.RawMessage, error) {
	for v := from; v < to; v++ {
		switch v {
		case 1:
			return nil, fmt.Errorf("no migration path from data version %d", v)
		}
	}
	return data, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeVersioned(path string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return writeJSON(path, doc{Version: currentVersion, Data: raw})
}

func (s *Store) saveExercises() error {
	return writeVersioned(s.exercisesPath, s.Exercises)
}

func (s *Store) saveSessions() error {
	return writeVersioned(s.sessionsPath, s.Sessions)
}

func (s *Store) ListExercises() []model.Exercise {
	unlock := s.lock()
	defer unlock()
	out := make([]model.Exercise, len(s.Exercises))
	copy(out, s.Exercises)
	return out
}

func (s *Store) FindExerciseByName(name string) *model.Exercise {
	unlock := s.lock()
	defer unlock()
	for i := range s.Exercises {
		if s.Exercises[i].Name == name {
			ex := s.Exercises[i]
			return &ex
		}
	}
	return nil
}

func (s *Store) FindExerciseByID(id string) *model.Exercise {
	unlock := s.lock()
	defer unlock()
	for i := range s.Exercises {
		if s.Exercises[i].ID == id {
			ex := s.Exercises[i]
			return &ex
		}
	}
	return nil
}

func (s *Store) AddExercise(ex model.Exercise) error {
	unlock := s.lock()
	defer unlock()
	s.Exercises = append(s.Exercises, ex)
	return s.saveExercises()
}

func (s *Store) RemoveExerciseByID(id string) bool {
	unlock := s.lock()
	defer unlock()
	for i := range s.Exercises {
		if s.Exercises[i].ID == id {
			s.Exercises = append(s.Exercises[:i], s.Exercises[i+1:]...)
			_ = s.saveExercises()
			return true
		}
	}
	return false
}

func (s *Store) UpdateExercise(ex model.Exercise) error {
	unlock := s.lock()
	defer unlock()
	for i := range s.Exercises {
		if s.Exercises[i].ID == ex.ID {
			s.Exercises[i] = ex
			return s.saveExercises()
		}
	}
	return fmt.Errorf("exercise %q not found", ex.ID)
}

func (s *Store) ListSessions() []model.Session {
	unlock := s.lock()
	defer unlock()
	out := make([]model.Session, len(s.Sessions))
	copy(out, s.Sessions)
	return out
}

func (s *Store) SessionByID(id string) *model.Session {
	unlock := s.lock()
	defer unlock()
	for i := range s.Sessions {
		if s.Sessions[i].ID == id {
			ss := s.Sessions[i]
			return &ss
		}
	}
	return nil
}

func (s *Store) AddSession(ss model.Session) error {
	unlock := s.lock()
	defer unlock()
	s.Sessions = append(s.Sessions, ss)
	return s.saveSessions()
}

func (s *Store) UpdateSession(ss model.Session) error {
	unlock := s.lock()
	defer unlock()
	for i := range s.Sessions {
		if s.Sessions[i].ID == ss.ID {
			s.Sessions[i] = ss
			return s.saveSessions()
		}
	}
	return fmt.Errorf("session %q not found", ss.ID)
}
