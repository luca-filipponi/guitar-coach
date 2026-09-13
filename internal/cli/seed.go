package cli

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

func (sh *Shell) newSeedCmd() *cobra.Command {
	var weeks int
	var force bool
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "generate fake historical practice data",
		Long: `Generates a few weeks of plausible past sessions (growing BPM levels,
per-exercise notes, occasional second rounds) so the history and the web UI
have something real to show. Data is written to the same store files you use
in practice and can be viewed with "serve".

By default this refuses to touch a data dir that already has exercises. Pass
--force to mix demo data into existing history, or use --dir for a fresh dir.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if weeks < 1 {
				return errors.New("--weeks must be at least 1")
			}
			if len(sh.api.ListExercises()) > 0 && !force {
				return errors.New("data dir already contains exercises; pass --force to add demo data, or use --dir for a fresh demo dir")
			}
			return sh.seedDemo(weeks)
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 4, "how many weeks of history to generate")
	cmd.Flags().BoolVar(&force, "force", false, "add demo data even if the dir already has exercises")
	return cmd
}

func (sh *Shell) seedDemo(weeks int) error {
	type def struct{ name, topic, desc string }
	defs := []def{
		{"chromatic warm-up", "warmup", "4 notes per string, up and down, loose wrist"},
		{"alternate picking level 1", "alternate picking", "16th notes on one string, strict down-up"},
		{"alternate picking level 2", "alternate picking", "2 strings per run, crossing stays clean"},
		{"economy picking runs", "economy picking", "3-notes-per-string economy sweeps"},
		{"sweep arpeggios", "sweep picking", "5-string major arpeggios, legato on the change"},
		{"legato pentatonic licks", "legato", "hammer-on / pull-off runs over the box"},
	}

	var pool []model.Exercise
	for _, d := range defs {
		var ex model.Exercise
		ex, err := sh.api.CreateExercise(d.name, d.topic, d.desc)
		if errors.Is(err, api.ErrExists) {
			ex, err = sh.api.FindExercise(d.name)
		}
		if err != nil {
			return fmt.Errorf("seeding exercises: %w", err)
		}
		if ex.Description == "" && d.desc != "" {
			ex, err = sh.api.SetDescription(ex.ID, d.desc)
			if err != nil {
				return fmt.Errorf("backfilling exercise description: %w", err)
			}
		}
		pool = append(pool, ex)
	}

	rng := rand.New(rand.NewSource(7))
	cfg := model.Config{ExercisesPerRound: 5, DurationSec: 300, RestSec: 60, BreakSec: 300}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	generated := 0
	for day := 0; day < weeks*7; day++ {
		if day != 0 && rng.Intn(7) < 2 {
			continue
		}
		t := now.AddDate(0, 0, -day)
		sess, err := sh.seedSession(rng, pool, cfg, practiceTime(t), weeks)
		if err != nil {
			return err
		}
		if err := sh.st.AddSession(sess); err != nil {
			return fmt.Errorf("writing demo session: %w", err)
		}
		generated++
	}

	fmt.Printf("generated %d demo sessions over %d weeks (%d exercises)\n", generated, weeks, len(pool))
	fmt.Println("start the UI with the same --dir:")
	fmt.Println("  guitar-coach serve")
	return nil
}

func (sh *Shell) seedSession(rng *rand.Rand, pool []model.Exercise, cfg model.Config, start time.Time, weeks int) (model.Session, error) {
	order := api.BuildPlan(pool, cfg.ExercisesPerRound)
	sess := model.Session{
		ID:        util.NewID("sess"),
		StartedAt: start.Format(time.RFC3339),
		Config:    cfg,
	}
	for _, ex := range order {
		sess.Order = append(sess.Order, ex.ID)
	}

	cursor := start
	rounds := 1
	if rng.Intn(4) == 0 {
		rounds = 2
	}
	for round := 1; round <= rounds; round++ {
		cur := order
		if round == 2 {
			cur = make([]model.Exercise, len(order))
			copy(cur, order)
			for i, j := 0, len(cur)-1; i < j; i, j = i+1, j-1 {
				cur[i], cur[j] = cur[j], cur[i]
			}
			cursor = cursor.Add(time.Duration(cfg.BreakSec) * time.Second)
		}
		for seq, ex := range cur {
			startedAt := cursor
			finishedAt := startedAt.Add(time.Duration(cfg.DurationSec) * time.Second)

			startBPM, endBPM := sh.seedBPM(rng, ex, weeks, dayOf(start))
			entry := model.Entry{
				ExerciseID: ex.ID,
				Name:       ex.Name,
				Round:      round,
				Sequence:   seq + 1,
				StartBPM:   startBPM,
				EndBPM:     endBPM,
				Notes:      seedNotes[rng.Intn(len(seedNotes))],
				StartedAt:  startedAt.Format(time.RFC3339),
				FinishedAt: finishedAt.Format(time.RFC3339),
			}
			sess.Entries = append(sess.Entries, entry)
			cursor = finishedAt.Add(time.Duration(cfg.RestSec) * time.Second)
		}
	}
	sess.EndedAt = cursor.Format(time.RFC3339)
	return sess, nil
}

// seedBPM returns start/end BPM that grow steadily across the generated
// weeks, with a per-exercise difficulty offset and some daily wobble.
func (sh *Shell) seedBPM(rng *rand.Rand, ex model.Exercise, weeks, day int) (int, int) {
	progress := 55 + 3*(weeks-1-day/7) + 4*len(ex.Topic)%11 + rng.Intn(7) - 3
	start := progress + rng.Intn(5) - 2
	end := start + rng.Intn(7) - 2
	if end < 40 {
		end = start
	}
	return start, end
}

func dayOf(t time.Time) int {
	y1, m1, d1 := t.UTC().Date()
	y2, m2, d2 := time.Now().UTC().Date()
	a := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	b := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	return int(a.Sub(b).Hours() / 24)
}

func practiceTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 18, 30, 0, 0, time.UTC)
}

var seedNotes = []string{
	"",
	"",
	"clean runs",
	"metronome 16ths",
	"wrist tension",
	"starting to click",
}
