package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/luca-filipponi/guitar-coach/internal/model"
)

func TestLegacyBareArrayMigratesToVersioned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exercises.json")
	legacy := `[{"id":"ex_1","name":"Warmup","created_at":"2026-01-01T00:00:00Z"}]`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Exercises) != 1 || s.Exercises[0].Name != "Warmup" {
		t.Fatalf("unexpected exercises: %+v", s.Exercises)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("expected versioned file after migration, got raw: %s", data)
	}
	if d.Version != currentVersion || d.Data == nil {
		t.Fatalf("unexpected versioned payload: %s", data)
	}
}

func TestVersionedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddExercise(model.Exercise{ID: "ex_1", Name: "Warmup", CreatedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession(model.Session{ID: "sess_1", StartedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Exercises) != 1 || s2.Exercises[0].Name != "Warmup" {
		t.Fatalf("unexpected exercises after reopen: %+v", s2.Exercises)
	}
	if len(s2.Sessions) != 1 || s2.Sessions[0].ID != "sess_1" {
		t.Fatalf("unexpected sessions after reopen: %+v", s2.Sessions)
	}
}

func TestNewerVersionRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	v99 := `{"version":99,"data":[]}`
	if err := os.WriteFile(path, []byte(v99), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("expected error for newer version, got nil")
	}
}

func TestUnknownFieldsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exercises.json")
	fut := `{"version":1,"data":[{"id":"ex_1","name":"Warmup","created_at":"2026-01-01","bpm_target":999}]}`
	if err := os.WriteFile(path, []byte(fut), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Exercises) != 1 || s.Exercises[0].Name != "Warmup" {
		t.Fatalf("unexpected exercises: %+v", s.Exercises)
	}
}
