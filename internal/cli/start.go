package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

func (sh *Shell) newStartCmd() *cobra.Command {
	var (
		numEx    int
		durStr   string
		restStr  string
		breakStr string
		pick     string
		sel      bool
	)
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start a practice session",
		RunE: func(cmd *cobra.Command, args []string) error {
			durD, err := util.ParseDuration(durStr)
			if err != nil {
				return fmt.Errorf("invalid --duration %q (use e.g. 5m or 300)", durStr)
			}
			restD, err := util.ParseDuration(restStr)
			if err != nil {
				return fmt.Errorf("invalid --rest %q", restStr)
			}
			breakD, err := util.ParseDuration(breakStr)
			if err != nil {
				return fmt.Errorf("invalid --break %q", breakStr)
			}
			if numEx < 1 {
				return errors.New("--exercises must be at least 1")
			}

			cfg := model.Config{
				ExercisesPerRound: numEx,
				DurationSec:       int(durD / time.Second),
				RestSec:           int(restD / time.Second),
				BreakSec:          int(breakD / time.Second),
			}

			pool := sh.api.ListExercises()
			if len(pool) == 0 {
				return errors.New(`no exercises yet — add some: guitar-coach add "Warmup"`)
			}

			order, err := sh.chooseExercises(pool, cfg.ExercisesPerRound, pick, sel)
			if err != nil {
				return err
			}
			return sh.runSession(cfg, order)
		},
		ValidArgsFunction: sh.noCompletion,
	}
	cmd.Flags().IntVar(&numEx, "exercises", 5, "exercises per round")
	cmd.Flags().StringVar(&durStr, "duration", "5m", "time per exercise")
	cmd.Flags().StringVar(&restStr, "rest", "1m", "rest between exercises")
	cmd.Flags().StringVar(&breakStr, "break", "5m", "break between rounds")
	cmd.Flags().StringVar(&pick, "pick", "", "comma-separated exercises to use")
	cmd.Flags().BoolVar(&sel, "select", false, "choose exercises interactively")
	_ = cmd.RegisterFlagCompletionFunc("pick", sh.completeExerciseNames)
	return cmd
}

func (sh *Shell) chooseExercises(pool []model.Exercise, n int, pick string, sel bool) ([]model.Exercise, error) {
	if pick != "" {
		var chosen []model.Exercise
		for _, token := range strings.Split(pick, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			ex, err := sh.api.FindExercise(token)
			if err != nil {
				return nil, err
			}
			chosen = append(chosen, ex)
		}
		if len(chosen) == 0 {
			return nil, errors.New("no exercises given in --pick")
		}
		return chosen, nil
	}
	if sel {
		return sh.selectExercises(pool, n)
	}
	return api.BuildPlan(pool, n), nil
}

func (sh *Shell) selectExercises(pool []model.Exercise, n int) ([]model.Exercise, error) {
	fmt.Println("\navailable exercises:")
	for i, ex := range pool {
		name := ex.Name
		if ex.Topic != "" {
			name = fmt.Sprintf("%s (%s)", ex.Name, ex.Topic)
		}
		fmt.Printf("  %2d. %s\n", i+1, name)
	}
	line, aborted := sh.readLineSig(fmt.Sprintf("choose up to %d (indices, comma-separated): ", n))
	if aborted {
		return nil, errors.New("aborted")
	}
	var chosen []model.Exercise
	seen := map[int]bool{}
	for _, tok := range strings.Split(line, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		idx, err := strconv.Atoi(tok)
		if err != nil || idx < 1 || idx > len(pool) {
			return nil, fmt.Errorf("invalid index %q", tok)
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		chosen = append(chosen, pool[idx-1])
		if len(chosen) >= n {
			break
		}
	}
	if len(chosen) == 0 {
		return nil, errors.New("no exercises chosen")
	}
	return chosen, nil
}

func (sh *Shell) runSession(cfg model.Config, order []model.Exercise) error {
	exerciseIDs := make([]string, 0, len(order))
	for _, ex := range order {
		exerciseIDs = append(exerciseIDs, ex.ID)
	}

	sess, err := sh.api.StartSession(api.StartSessionRequest{
		ExerciseIDs: exerciseIDs,
		Config:      cfg,
	})
	if err != nil {
		return err
	}

	fmt.Println("\nsession plan:")
	for i, ex := range order {
		label := ex.Name
		if ex.Topic != "" {
			label = fmt.Sprintf("%s (%s)", ex.Name, ex.Topic)
		}
		fmt.Printf("  %d. %s -- %s, then %s rest\n",
			i+1, label, util.FormatDuration(durationOf(cfg.DurationSec)), util.FormatDuration(durationOf(cfg.RestSec)))
	}
	fmt.Printf("after %d exercises: order is swapped, and you're asked to start another round (then %s break)\n",
		len(order), util.FormatDuration(durationOf(cfg.BreakSec)))
	fmt.Println("\nenter start BPM before each exercise. After the set, the rest timer runs while you record end BPM and notes. Ctrl-C skips a timer or aborts a prompt.")

	round := 1
	cur := order
	for {
		fmt.Printf("\n=== round %d ===\n", round)
		quit := false
		for i, ex := range cur {
			seq := i + 1
			label := ex.Name
			if ex.Topic != "" {
				label = fmt.Sprintf("%s (%s)", ex.Name, ex.Topic)
			}
			fmt.Printf("\n[%d/%d] %s  (%s each)\n", seq, len(cur), label, util.FormatDuration(durationOf(cfg.DurationSec)))
			sh.showHistory(ex.ID)

			suggest := sh.lastEndBPM(ex.ID)
			startBPM, aborted := sh.promptBPM("start BPM", suggest)
			if aborted {
				quit = true
				break
			}

			startedAt := util.NowRFC()
			sh.countdown(durationOf(cfg.DurationSec), "Doing: "+label)
			finishedAt := util.NowRFC()

			restDone := sh.restWindow(label, durationOf(cfg.RestSec))
			endBPM, aborted := sh.promptBPM("end BPM", startBPM)
			notes := ""
			if !aborted {
				notes, aborted = sh.readLineSig("notes: ")
			}
			if aborted {
				quit = true
				break
			}
			<-restDone

			entry := model.Entry{
				ExerciseID: ex.ID,
				Name:       ex.Name,
				Round:      round,
				Sequence:   seq,
				StartBPM:   startBPM,
				EndBPM:     endBPM,
				Notes:      notes,
				StartedAt:  startedAt,
				FinishedAt: finishedAt,
			}
			sess, err = sh.api.AddSessionEntry(sess.ID, entry)
			if err != nil {
				return err
			}
			fmt.Printf("  recorded: %s %d -> %d bpm\n", ex.Name, startBPM, endBPM)
		}
		if quit {
			fmt.Println("\nsession ended (quit requested).")
			break
		}

		line, aborted := sh.readLineSig("\nstart another round? [Y/n]: ")
		if aborted || strings.EqualFold(line, "n") || strings.EqualFold(line, "no") {
			break
		}
		cur = sh.swapOrder(cur)
		sh.countdown(durationOf(cfg.BreakSec), "Break -- order swapped, next round ready")
		round++
	}

	sess, err = sh.api.EndSession(sess.ID)
	if err != nil {
		return err
	}

	total := util.ParseTime(sess.EndedAt).Sub(util.ParseTime(sess.StartedAt))
	fmt.Printf("\nsession %s finished -- %d rounds, %d exercises done, %s total\n",
		sess.ID, model.MaxRound(sess.Entries), len(sess.Entries), util.FormatDuration(total))
	return nil
}

func (sh *Shell) swapOrder(in []model.Exercise) []model.Exercise {
	out := make([]model.Exercise, len(in))
	for i := range in {
		out[len(in)-1-i] = in[i]
	}
	return out
}

func durationOf(sec int) time.Duration {
	return time.Duration(sec) * time.Second
}
