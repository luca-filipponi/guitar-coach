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

	ex, err := a.CreateExercise("Warmup", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if ex.ID == "" || ex.Name != "Warmup" {
		t.Fatalf("unexpected exercise: %+v", ex)
	}

	if _, err := a.CreateExercise("Warmup", "", ""); !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists, got %v", err)
	}

	if _, err := a.CreateExercise("  ", "", ""); err == nil {
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
	ex1, _ := a.CreateExercise("Warmup", "", "")
	ex2, _ := a.CreateExercise("Arpeggios", "", "")

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

func TestDiscardEmptySession(t *testing.T) {
	a := newTestAPI(t)
	ex, _ := a.CreateExercise("Warmup", "", "")
	sess, err := a.StartSession(StartSessionRequest{ExerciseIDs: []string{ex.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetSession(sess.ID); err != nil {
		t.Fatalf("expected session to exist before delete: %v", err)
	}
	if err := a.DeleteSession(sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetSession(sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestUpdateSessionEntry(t *testing.T) {
	a := newTestAPI(t)
	ex, _ := a.CreateExercise("Arpeggios", "", "")
	sess, err := a.StartSession(StartSessionRequest{ExerciseIDs: []string{ex.ID}})
	if err != nil {
		t.Fatal(err)
	}
	for _, bpm := range []int{60, 62} {
		sess, err = a.AddSessionEntry(sess.ID, model.Entry{
			ExerciseID: ex.ID,
			Name:       ex.Name,
			Round:      1,
			Sequence:   1,
			StartBPM:   bpm,
			EndBPM:     bpm + 10,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	got, err := a.UpdateSessionEntry(sess.ID, 1, func(e *model.Entry) {
		e.StartBPM = 64
		e.EndBPM = 75
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got.Entries))
	}
	if got.Entries[1].StartBPM != 64 || got.Entries[1].EndBPM != 75 {
		t.Fatalf("entry not updated: %+v", got.Entries[1])
	}
	reload, err := a.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reload.Entries[1].StartBPM != 64 {
		t.Fatalf("update not persisted: %+v", reload.Entries[1])
	}

	if _, err := a.UpdateSessionEntry(sess.ID, 99, func(*model.Entry) {}); err == nil {
		t.Fatal("expected error for out-of-range entry")
	}
}

func TestStartSessionDefaults(t *testing.T) {
	a := newTestAPI(t)
	ex, _ := a.CreateExercise("Warmup", "", "")
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
	ex, err := a.CreateExercise("Sweeps", "", "")
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

func TestSetDescription(t *testing.T) {
	a := newTestAPI(t)
	ex, err := a.CreateExercise("Bends", "bends", "slow half-step bends, tune by ear")
	if err != nil {
		t.Fatal(err)
	}
	if ex.Description != "slow half-step bends, tune by ear" {
		t.Fatalf("expected description at creation, got %q", ex.Description)
	}

	updated, err := a.SetDescription(ex.ID, "   add vibrato at the top  ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "add vibrato at the top" {
		t.Fatalf("expected trimmed description, got %q", updated.Description)
	}

	got, err := a.GetExercise(ex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "add vibrato at the top" {
		t.Fatalf("expected persisted description, got %q", got.Description)
	}

	cleared, err := a.SetDescription(ex.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Description != "" {
		t.Fatalf("expected empty description after clear, got %q", cleared.Description)
	}

	if _, err := a.SetDescription("missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestScrambleRotation(t *testing.T) {
	exercises := []model.Exercise{
		{ID: "ex1", Name: "A", Topic: "picking"},
		{ID: "ex2", Name: "B", Topic: "picking"},
		{ID: "ex3", Name: "C", Topic: "sweeping"},
		{ID: "ex4", Name: "D", Topic: "legato"},
		{ID: "ex5", Name: "E", Topic: "picking"},
	}
	seenReverse := 0
	seenSame := 0
	for i := 0; i < 40; i++ {
		got := ScrambleRotation(exercises)
		if len(got) != len(exercises) {
			t.Fatalf("expected %d exercises, got %d", len(exercises), len(got))
		}
		ids := make(map[string]bool)
		for _, ex := range got {
			ids[ex.ID] = true
		}
		if len(ids) != len(exercises) {
			t.Fatalf("scramble changed exercise set: got %v", got)
		}
		eq := sameOrder(got, exercises)
		rev := make([]model.Exercise, len(exercises))
		for j := range exercises {
			rev[len(exercises)-1-j] = exercises[j]
		}
		eqRev := sameOrder(got, rev)
		if eq {
			seenSame++
		}
		if eqRev {
			seenReverse++
		}
	}
	if seenSame > 0 {
		t.Errorf("scramble produced the same order too often (%d/40 runs identical)", seenSame)
	}
	if seenReverse > 0 {
		t.Errorf("scramble produced the exact reverse too often (%d/40 runs exact reverse)", seenReverse)
	}
}
