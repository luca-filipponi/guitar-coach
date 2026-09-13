package api

import (
	"errors"
	"testing"

	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/store"
)

func newTestAPI(t *testing.T) *API {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(st)
}

func TestExerciseCRUD(t *testing.T) {
	a := newTestAPI(t)

	ex, err := a.CreateExercise("Warmup", "")
	if err != nil {
		t.Fatal(err)
	}
	if ex.ID == "" || ex.Name != "Warmup" {
		t.Fatalf("unexpected exercise: %+v", ex)
	}

	if _, err := a.CreateExercise("Warmup", ""); !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists, got %v", err)
	}

	if _, err := a.CreateExercise("  ", ""); err == nil {
		t.Fatal("expected error for empty name")
	}

	if _, err := a.FindExercise("Warmup"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FindExercise("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := a.DeleteExercise(ex.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetExercise(ex.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	a := newTestAPI(t)
	ex1, _ := a.CreateExercise("Warmup", "")
	ex2, _ := a.CreateExercise("Arpeggios", "")

	sess, err := a.StartSession(StartSessionRequest{
		ExerciseIDs: []string{ex1.ID, ex2.ID},
		Config:      model.Config{ExercisesPerRound: 2, DurationSec: 60, RestSec: 10, BreakSec: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == "" || len(sess.Order) != 2 {
		t.Fatalf("unexpected session: %+v", sess)
	}

	if _, err := a.StartSession(StartSessionRequest{ExerciseIDs: []string{"nope"}}); err == nil {
		t.Fatal("expected error for unknown exercise in session")
	}

	entry := model.Entry{
		ExerciseID: ex1.ID,
		Name:       ex1.Name,
		Round:      1,
		Sequence:   1,
		StartBPM:   60,
		EndBPM:     70,
		Notes:      "felt good",
	}
	sess, err = a.AddSessionEntry(sess.ID, entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sess.Entries))
	}

	pts := a.ExerciseProgress(ex1.ID)
	if len(pts) != 1 || pts[0].EndBPM != 70 || pts[0].StartBPM != 60 {
		t.Fatalf("unexpected progress: %+v", pts)
	}

	sess, err = a.EndSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.EndedAt == "" {
		t.Fatal("expected ended_at to be set")
	}

	if _, err := a.GetSession("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStartSessionDefaults(t *testing.T) {
	a := newTestAPI(t)
	ex, _ := a.CreateExercise("Warmup", "")
	sess, err := a.StartSession(StartSessionRequest{ExerciseIDs: []string{ex.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if sess.Config.ExercisesPerRound != 5 || sess.Config.DurationSec != 300 || sess.Config.RestSec != 60 || sess.Config.BreakSec != 300 {
		t.Fatalf("expected default config, got %+v", sess.Config)
	}
}

func TestSetTopic(t *testing.T) {
	a := newTestAPI(t)
	ex, err := a.CreateExercise("Sweeps", "")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := a.SetTopic(ex.ID, "  sweep picking ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Topic != "sweep picking" {
		t.Fatalf("expected trimmed topic sweep picking, got %q", updated.Topic)
	}

	got, err := a.GetExercise(ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Topic != "sweep picking" {
		t.Fatalf("expected persisted topic, got %q", got.Topic)
	}

	cleared, err := a.SetTopic(ex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Topic != "" {
		t.Fatalf("expected empty topic after clear, got %q", cleared.Topic)
	}

	if _, err := a.SetTopic("missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
