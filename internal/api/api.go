package api

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/store"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

var (
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
)

type API struct {
	store *store.Store
}

func New(st *store.Store) *API {
	return &API{store: st}
}

func (a *API) CreateExercise(name, topic, description string) (model.Exercise, error) {
	name = strings.TrimSpace(name)
	topic = strings.TrimSpace(topic)
	description = strings.TrimSpace(description)
	if name == "" {
		return model.Exercise{}, errors.New("exercise name is required")
	}
	if a.store.FindExerciseByName(name) != nil {
		return model.Exercise{}, fmt.Errorf("exercise %q %w", name, ErrExists)
	}
	ex := model.Exercise{ID: util.NewID("ex"), Name: name, Topic: topic, Description: description, CreatedAt: util.NowRFC()}
	if err := a.store.AddExercise(ex); err != nil {
		return model.Exercise{}, err
	}
	return ex, nil
}

func (a *API) ListExercises() []model.Exercise {
	return a.store.ListExercises()
}

func (a *API) FindExercise(nameOrID string) (model.Exercise, error) {
	for _, ex := range a.store.ListExercises() {
		if ex.ID == nameOrID || ex.Name == nameOrID {
			return ex, nil
		}
	}
	return model.Exercise{}, fmt.Errorf("exercise %q %w", nameOrID, ErrNotFound)
}

func (a *API) GetExercise(id string) (model.Exercise, error) {
	if ex := a.store.FindExerciseByID(id); ex != nil {
		return *ex, nil
	}
	return model.Exercise{}, fmt.Errorf("exercise %w", ErrNotFound)
}

func (a *API) DeleteExercise(id string) error {
	if ok := a.store.RemoveExerciseByID(id); !ok {
		return fmt.Errorf("exercise %w", ErrNotFound)
	}
	return nil
}

func (a *API) SetTopic(id, topic string) (model.Exercise, error) {
	ex, err := a.GetExercise(id)
	if err != nil {
		return model.Exercise{}, err
	}
	ex.Topic = strings.TrimSpace(topic)
	if err := a.store.UpdateExercise(ex); err != nil {
		return model.Exercise{}, err
	}
	return ex, nil
}

func (a *API) SetDescription(id, description string) (model.Exercise, error) {
	ex, err := a.GetExercise(id)
	if err != nil {
		return model.Exercise{}, err
	}
	ex.Description = strings.TrimSpace(description)
	if err := a.store.UpdateExercise(ex); err != nil {
		return model.Exercise{}, err
	}
	return ex, nil
}

type ProgressPoint struct {
	Date      string `json:"date"`
	SessionID string `json:"session_id,omitempty"`
	Round     int    `json:"round,omitempty"`
	StartBPM  int    `json:"start_bpm,omitempty"`
	EndBPM    int    `json:"end_bpm"`
	Notes     string `json:"notes,omitempty"`
}

func (a *API) ExerciseProgress(exerciseID string) []ProgressPoint {
	pts := make([]ProgressPoint, 0)
	for _, s := range a.store.ListSessions() {
		for _, e := range s.Entries {
			if e.ExerciseID == exerciseID {
				pts = append(pts, ProgressPoint{
					Date:      util.ParseTime(e.FinishedAt).Format(time.RFC3339),
					SessionID: s.ID,
					Round:     e.Round,
					StartBPM:  e.StartBPM,
					EndBPM:    e.EndBPM,
					Notes:     e.Notes,
				})
			}
		}
	}
	return pts
}

type StartSessionRequest struct {
	ExerciseIDs []string     `json:"exercise_ids"`
	Config      model.Config `json:"config"`
}

func (a *API) StartSession(req StartSessionRequest) (model.Session, error) {
	if len(req.ExerciseIDs) == 0 {
		return model.Session{}, errors.New("session requires at least one exercise")
	}
	for _, id := range req.ExerciseIDs {
		if _, err := a.GetExercise(id); err != nil {
			return model.Session{}, err
		}
	}
	cfg := req.Config
	if cfg.ExercisesPerRound == 0 {
		cfg.ExercisesPerRound = 5
	}
	if cfg.DurationSec == 0 {
		cfg.DurationSec = 300
	}
	if cfg.RestSec == 0 {
		cfg.RestSec = 60
	}
	if cfg.BreakSec == 0 {
		cfg.BreakSec = 300
	}
	sess := model.Session{
		ID:        util.NewID("sess"),
		StartedAt: util.NowRFC(),
		Config:    cfg,
		Order:     append([]string(nil), req.ExerciseIDs...),
	}
	if err := a.store.AddSession(sess); err != nil {
		return model.Session{}, err
	}
	return sess, nil
}

func (a *API) ListSessions() []model.Session {
	return a.store.ListSessions()
}

// ListSessionsPage returns a page of sessions ordered newest-first, optionally
// filtered to a single local calendar day (YYYY-MM-DD). It returns the page
// and the total number of sessions matching the filter.
func (a *API) ListSessionsPage(page, perPage int, date string) ([]model.Session, int) {
	all := a.store.ListSessions()
	filtered := make([]model.Session, 0, len(all))
	for _, s := range all {
		if date != "" {
			day := util.ParseTime(s.StartedAt).Local().Format("2006-01-02")
			if day != date {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].StartedAt > filtered[j].StartedAt
	})
	total := len(filtered)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func (a *API) GetSession(id string) (model.Session, error) {
	s := a.store.SessionByID(id)
	if s == nil {
		return model.Session{}, fmt.Errorf("session %w", ErrNotFound)
	}
	return *s, nil
}

func (a *API) AddSessionEntry(sessionID string, entry model.Entry) (model.Session, error) {
	sess, err := a.GetSession(sessionID)
	if err != nil {
		return model.Session{}, err
	}
	if entry.ExerciseID == "" {
		return model.Session{}, errors.New("entry requires an exercise id")
	}
	sess.Entries = append(sess.Entries, entry)
	if err := a.store.UpdateSession(sess); err != nil {
		return model.Session{}, err
	}
	return sess, nil
}

// UpdateSessionEntry mutates a single entry (by 0-based index into the
// session's Entries) and persists the session.
func (a *API) UpdateSessionEntry(sessionID string, idx int, mutate func(*model.Entry)) (model.Session, error) {
	sess, err := a.GetSession(sessionID)
	if err != nil {
		return model.Session{}, err
	}
	if idx < 0 || idx >= len(sess.Entries) {
		return model.Session{}, fmt.Errorf("entry number out of range (1..%d)", len(sess.Entries))
	}
	mutate(&sess.Entries[idx])
	if err := a.store.UpdateSession(sess); err != nil {
		return model.Session{}, err
	}
	return sess, nil
}

func (a *API) EndSession(sessionID string) (model.Session, error) {
	sess, err := a.GetSession(sessionID)
	if err != nil {
		return model.Session{}, err
	}
	sess.EndedAt = util.NowRFC()
	sess.ResumeRound = 0
	sess.ResumeOrder = nil
	sess.ResumeSeq = 0
	if err := a.store.UpdateSession(sess); err != nil {
		return model.Session{}, err
	}
	return sess, nil
}

// SetResumePoint records where a running session can be picked up again: the
// round in progress, that round's exact exercise order, and how many exercises
// of it are already completed. It does not change the ended_at field, so the
// session stays "in progress" until EndSession clears the point again.
func (a *API) SetResumePoint(sessionID string, round int, order []string, seq int) error {
	sess, err := a.GetSession(sessionID)
	if err != nil {
		return err
	}
	sess.ResumeRound = round
	sess.ResumeOrder = append([]string(nil), order...)
	sess.ResumeSeq = seq
	return a.store.UpdateSession(sess)
}

func (a *API) DeleteSession(sessionID string) error {
	if err := a.store.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("session %w", ErrNotFound)
	}
	return nil
}
