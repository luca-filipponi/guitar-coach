package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

func TestMigrateBareArray(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exercises.json"), []byte(`[{"id":"ex_1","name":"Warmup","created_at":"2026-01-01T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	exs := s.ListExercises()
	if len(exs) != 1 || exs[0].Name != "Warmup" {
		t.Fatalf("unexpected exercises: %+v", exs)
	}
	for _, f := range []string{"exercises.json", "sessions.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed", f)
		}
	}
}

func TestMigrateVersionedDoc(t *testing.T) {
	dir := t.TempDir()
	v1 := doc{Version: 1, Data: mustJSON([]model.Exercise{{ID: "ex_1", Name: "Warmup", CreatedAt: "2026-01-01T00:00:00Z"}})}
	if err := os.WriteFile(filepath.Join(dir, "exercises.json"), mustJSON(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ListExercises()) != 1 {
		t.Fatalf("expected 1 exercise, got %d", len(s.ListExercises()))
	}
}

func TestMigrateSkipsIfDBHasData(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession(model.Session{ID: "sess_existing", StartedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	s.close()

	if err := os.WriteFile(filepath.Join(dir, "exercises.json"), []byte(`[{"id":"ex_new","name":"should skip","created_at":"2026-01-01T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.ListExercises()) != 0 {
		t.Fatalf("expected no exercises imported over existing data, got %d", len(s2.ListExercises()))
	}
}

func TestRejectNewerVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sessions.json"), []byte(`{"version":2,"data":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("expected error for newer version")
	}
}

func TestMigrateIgnoresUnknownFields(t *testing.T) {
	dir := t.TempDir()
	data := `{"version":1,"data":[{"id":"ex_1","name":"Warmup","created_at":"2026-01-01T00:00:00Z","bpm_target":999}]}`
	if err := os.WriteFile(filepath.Join(dir, "exercises.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ListExercises()) != 1 || s.ListExercises()[0].Name != "Warmup" {
		t.Fatalf("unexpected exercises: %+v", s.ListExercises())
	}
}

func TestRoundTripReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddExercise(model.Exercise{ID: "ex_1", Name: "Warmup", CreatedAt: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession(model.Session{
		ID:        "sess_1",
		StartedAt: "2026-01-01T00:00:00Z",
		Entries:   []model.Entry{{ExerciseID: "ex_1", Name: "Warmup", Round: 1, Sequence: 1, StartBPM: 60, EndBPM: 60}},
	}); err != nil {
		t.Fatal(err)
	}
	s.close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	exs := s2.ListExercises()
	if len(exs) != 1 || exs[0].Name != "Warmup" {
		t.Fatalf("unexpected exercises: %+v", exs)
	}
	sess := s2.SessionByID("sess_1")
	if sess == nil || len(sess.Entries) != 1 || sess.Entries[0].StartBPM != 60 {
		t.Fatalf("unexpected session: %+v", sess)
	}
}

func TestDeleteSessionCascade(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession(model.Session{
		ID:        "sess_1",
		StartedAt: "2026-01-01T00:00:00Z",
		Entries:   []model.Entry{{ExerciseID: "ex_1", StartBPM: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession("sess_1"); err != nil {
		t.Fatal(err)
	}
	if s.SessionByID("sess_1") != nil {
		t.Fatal("expected session to be deleted")
	}
}

func TestUpdateExerciseNotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateExercise(model.Exercise{ID: "missing"}); err == nil {
		t.Fatal("expected error for missing exercise")
	}
}

func TestRemoveExerciseNotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok := s.RemoveExerciseByID("missing"); ok {
		t.Fatal("expected false for missing exercise")
	}
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
