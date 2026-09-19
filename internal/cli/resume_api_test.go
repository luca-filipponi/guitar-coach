package cli

import (
	"testing"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/store"
)

func seedExercise(t *testing.T, a *api.API, name, topic string) model.Exercise {
	t.Helper()
	ex, err := a.CreateExercise(name, topic, "")
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

func openTestShell(t *testing.T) *Shell {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Shell{api: api.New(st), st: st}
}

// TestResumePointPersistRoundTrip drives the real API path used by runRounds:
// creating a session, recording entries, writing a resume point mid-round and
// reading it back with the same position resumeSession would use.
func TestResumePointPersistRoundTrip(t *testing.T) {
	sh := openTestShell(t)
	ex1 := seedExercise(t, sh.api, "Chord drills", "chords")
	ex2 := seedExercise(t, sh.api, "Scales", "scales")
	ex3 := seedExercise(t, sh.api, "Ear training", "ears")

	sess, err := sh.api.StartSession(api.StartSessionRequest{
		ExerciseIDs: []string{ex1.ID, ex2.ID, ex3.ID},
		Config:      model.Config{ExercisesPerRound: 3, DurationSec: 60, RestSec: 30, BreakSec: 300},
	})
	if err != nil {
		t.Fatal(err)
	}

	// One entry done (exercise 2 of round 1), then paused mid-round: the resume
	// point says round 1, same order, 1 exercise completed.
	entry := model.Entry{
		ExerciseID: ex2.ID, Name: ex2.Name, Round: 1, Sequence: 2,
		StartBPM: 60, EndBPM: 62, StartedAt: "2026-01-02T00:00:00Z", FinishedAt: "2026-01-02T00:01:00Z",
	}
	if _, err := sh.api.AddSessionEntry(sess.ID, entry); err != nil {
		t.Fatal(err)
	}
	if err := sh.api.SetResumePoint(sess.ID, 1, []string{ex1.ID, ex2.ID, ex3.ID}, 1); err != nil {
		t.Fatal(err)
	}

	round, cur, startSeq, err := sh2ResumePosition(sh, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if round != 1 || startSeq != 1 || len(cur) != 3 || cur[0].ID != ex1.ID || cur[1].ID != ex2.ID || cur[2].ID != ex3.ID {
		t.Fatalf("resume position wrong: round=%d seq=%d cur=%v", round, startSeq, idsOf(cur))
	}
}

// TestResumePositionFallbackReconstructs ensures sessions without a persisted
// point (pre-resume data) can still be resumed by rebuilding the round order
// from entries.
func TestResumePositionFallbackReconstructs(t *testing.T) {
	sh := openTestShell(t)
	ex1 := seedExercise(t, sh.api, "Chord drills", "chords")
	ex2 := seedExercise(t, sh.api, "Scales", "scales")

	sess, err := sh.api.StartSession(api.StartSessionRequest{
		ExerciseIDs: []string{ex1.ID, ex2.ID},
		Config:      model.Config{ExercisesPerRound: 2, DurationSec: 60, RestSec: 30, BreakSec: 300},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Round 1 partially done (1 of 2), sequence order in storage reversed.
	if _, err := sh.api.AddSessionEntry(sess.ID, model.Entry{ExerciseID: ex2.ID, Name: ex2.Name, Round: 1, Sequence: 2, StartBPM: 60, EndBPM: 62}); err != nil {
		t.Fatal(err)
	}

	// No resume point set: fallback must rebuild round 1 with 2 done.
	round, cur, startSeq, err := sh2ResumePosition(sh, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if round != 1 || startSeq != 1 || len(cur) != 2 {
		t.Fatalf("fallback position wrong: round=%d seq=%d cur=%v", round, startSeq, idsOf(cur))
	}
	if cur[0].ID != ex1.ID {
		t.Fatalf("fallback order should start with sequence-1 exercise, got %s", cur[0].ID)
	}
}

// TestResumeAdvancesPastCompletedRound verifies that a session paused with a
// fully complete round resumes into a freshly scrambled next round.
func TestResumeAdvancesPastCompletedRound(t *testing.T) {
	sh := openTestShell(t)
	ex1 := seedExercise(t, sh.api, "Chord drills", "chords")
	ex2 := seedExercise(t, sh.api, "Scales", "scales")

	sess, err := sh.api.StartSession(api.StartSessionRequest{
		ExerciseIDs: []string{ex1.ID, ex2.ID},
		Config:      model.Config{ExercisesPerRound: 2, DurationSec: 60, RestSec: 30, BreakSec: 300},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sh.api.AddSessionEntry(sess.ID, model.Entry{ExerciseID: ex1.ID, Name: ex1.Name, Round: 1, Sequence: 1, StartBPM: 60, EndBPM: 60}); err != nil {
		t.Fatal(err)
	}
	if _, err := sh.api.AddSessionEntry(sess.ID, model.Entry{ExerciseID: ex2.ID, Name: ex2.Name, Round: 1, Sequence: 2, StartBPM: 60, EndBPM: 60}); err != nil {
		t.Fatal(err)
	}
	if err := sh.api.SetResumePoint(sess.ID, 1, []string{ex1.ID, ex2.ID}, 2); err != nil {
		t.Fatal(err)
	}

	round, cur, startSeq, err := sh2ResumePosition(sh, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if round != 2 || startSeq != 0 || len(cur) != 2 {
		t.Fatalf("completed-round resume wrong: round=%d seq=%d cur=%v", round, startSeq, idsOf(cur))
	}
}

func sh2ResumePosition(sh *Shell, id string) (int, []model.Exercise, int, error) {
	sess, err := sh.api.GetSession(id)
	if err != nil {
		return 0, nil, 0, err
	}
	return sh.resumePosition(sess)
}
