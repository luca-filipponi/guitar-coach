package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
)

func (sh *Shell) newResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <session-id>",
		Short: "resume a paused session from where it stopped",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sh.resumeSession(args[0])
		},
		ValidArgsFunction: sh.completeSessionIDs,
	}
	return cmd
}

// resumeSession continues a session that was left in progress (ended_at empty).
// A persisted resume point gives the exact position; otherwise the position is
// reconstructed from the recorded entries so sessions from before the resume
// feature existed can still be picked up again.
func (sh *Shell) resumeSession(id string) error {
	sess, err := sh.api.GetSession(id)
	if err != nil {
		return err
	}
	if sess.EndedAt != "" {
		return fmt.Errorf("session %s already finished -- nothing to resume", id)
	}

	round, cur, startSeq, err := sh.resumePosition(sess)
	if err != nil {
		return err
	}

	fmt.Printf("\nresuming session %s at round %d\n", sess.ID, round)
	return sh.runRounds(sess, round, cur, startSeq)
}

// resumePosition decides where a session should pick up: from the persisted
// resume point when there is one, otherwise reconstructed from the entries.
// A fully completed round is advanced into a freshly scrambled next round.
func (sh *Shell) resumePosition(sess model.Session) (round int, cur []model.Exercise, startSeq int, err error) {
	if sess.ResumeRound > 0 && len(sess.ResumeOrder) > 0 {
		exes, err := sh.exercisesFor(sess.ResumeOrder)
		if err != nil {
			return 0, nil, 0, err
		}
		if len(exes) == 0 {
			return 0, nil, 0, errors.New("session has no exercises left to resume")
		}
		seq := sess.ResumeSeq
		if seq > len(exes) {
			seq = len(exes)
		}
		round, cur, startSeq = sess.ResumeRound, exes, seq
	} else {
		if len(sess.Entries) == 0 {
			return 0, nil, 0, errors.New("session has no recorded exercises yet -- start it fresh instead")
		}
		lastRound := model.MaxRound(sess.Entries)
		var order []string
		if lastRound == 1 {
			// Round 1's exact order is stored on the session itself.
			order = sess.Order
		} else {
			order, _ = roundOrderFromEntries(sess, lastRound)
		}
		exes, err := sh.exercisesFor(order)
		if err != nil {
			return 0, nil, 0, err
		}
		round, cur, startSeq = lastRound, exes, len(entriesForRound(sess, lastRound))
	}

	// A pause exactly at a round boundary leaves the round fully completed:
	// move to a fresh scrambled next round.
	if startSeq >= len(cur) {
		round++
		cur = api.ScrambleRotation(cur)
		startSeq = 0
	}
	return round, cur, startSeq, nil
}

// roundOrderFromEntries rebuilds a round's full exercise order from its
// recorded entries (ordered by sequence) and returns how many were completed.
func roundOrderFromEntries(sess model.Session, round int) ([]string, int) {
	var entries []model.Entry
	for _, e := range sess.Entries {
		if e.Round == round {
			entries = append(entries, e)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Sequence < entries[j].Sequence
	})
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		order = append(order, e.ExerciseID)
	}
	return order, len(entries)
}

func entriesForRound(sess model.Session, round int) []model.Entry {
	var entries []model.Entry
	for _, e := range sess.Entries {
		if e.Round == round {
			entries = append(entries, e)
		}
	}
	return entries
}

// exercisesFor resolves exercise IDs to exercises, dropping any that were
// deleted since. Returns an error when none of the order survives.
func (sh *Shell) exercisesFor(ids []string) ([]model.Exercise, error) {
	var out []model.Exercise
	for _, id := range ids {
		if ex, err := sh.api.GetExercise(id); err == nil {
			out = append(out, ex)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no exercises found for these ids")
	}
	return out, nil
}
