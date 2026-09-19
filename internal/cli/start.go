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
		numEx      int
		durStr     string
		restStr    string
		breakStr   string
		warmupStr  string
		alarmSound string
		alarmVol   float64
		alarmCount int
		alarmGap   string
		pick       string
		sel        bool
		freshRound bool
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
			warmupD, err := util.ParseDuration(warmupStr)
			if err != nil {
				return fmt.Errorf("invalid --warmup %q", warmupStr)
			}
			gapD, err := util.ParseDuration(alarmGap)
			if err != nil {
				return fmt.Errorf("invalid --alarm-gap %q", alarmGap)
			}
			if alarmCount < 1 {
				return errors.New("--alarm-count must be at least 1")
			}
			sh.warmup = warmupD
			sh.alarmSound = alarmSound
			sh.alarmVolume = alarmVol
			sh.alarmCount = alarmCount
			sh.alarmGap = gapD
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

			order, err := sh.chooseExercises(pool, cfg.ExercisesPerRound, pick, sel, freshRound)
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
	cmd.Flags().StringVar(&warmupStr, "warmup", "5m", "warm-up time before the first set")
	cmd.Flags().StringVar(&alarmSound, "alarm-sound", "/System/Library/Sounds/Ping.aiff", "alert sound file")
	cmd.Flags().Float64Var(&alarmVol, "alarm-volume", 2.5, "alert volume (afplay -v), 1.0 is the default loudness")
	cmd.Flags().IntVar(&alarmCount, "alarm-count", 3, "alert beeps per alarm")
	cmd.Flags().StringVar(&alarmGap, "alarm-gap", "220ms", "pause between alert beeps")
	cmd.Flags().StringVar(&pick, "pick", "", "comma-separated exercises to use")
	cmd.Flags().BoolVar(&sel, "select", false, "choose exercises interactively")
	cmd.Flags().BoolVar(&freshRound, "new", false, "fresh random rotation (skip the keep-previous prompt)")
	_ = cmd.RegisterFlagCompletionFunc("pick", sh.completeExerciseNames)
	return cmd
}

func (sh *Shell) chooseExercises(pool []model.Exercise, n int, pick string, sel bool, freshRound bool) ([]model.Exercise, error) {
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
	if !freshRound {
		if last := sh.lastRotation(); len(last) > 0 {
			line, aborted := sh.readLineSig(fmt.Sprintf("keep previous rotation (%d exercises)? [Y/n]: ", len(last)))
			if !aborted && !strings.EqualFold(line, "n") && !strings.EqualFold(line, "no") {
				fmt.Printf("using previous rotation\n")
				return last, nil
			}
		}
	}
	return api.BuildPlan(pool, n), nil
}

// lastRotation returns the exercises ordered in the most recent session that
// still exist, or nil when there is no prior rotation to reuse.
func (sh *Shell) lastRotation() []model.Exercise {
	sessions := sh.api.ListSessions()
	for i := len(sessions) - 1; i >= 0; i-- {
		var exes []model.Exercise
		for _, id := range sessions[i].Order {
			if ex, err := sh.api.GetExercise(id); err == nil {
				exes = append(exes, ex)
			}
		}
		if len(exes) > 0 {
			return exes
		}
	}
	return nil
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
	exerciseIDs := idsOf(order)

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
	fmt.Printf("after %d exercises: the order is scrambled for the next round (then %s break)\n",
		len(order), util.FormatDuration(durationOf(cfg.BreakSec)))
	fmt.Println("\nenter start BPM before each exercise. After the set, the rest timer runs while you record end BPM and notes. Ctrl-C skips a timer or aborts a prompt.")

	fmt.Println("\nwarm up before the first set:")
	fmt.Println("press Enter when you are ready to start the warmup (Ctrl-C to skip it)")
	if _, aborted := sh.readLineSig("start warmup when ready: "); aborted {
		// Ctrl-C at the ready-gate skips warmup entirely, like countdown aborts
		return nil
	}
	sh.countdown(sh.warmup, "Warmup -- open the hands, play lightly")

	return sh.runRounds(sess, 1, order, 0)
}

// runRounds drives the main timed loop for an already-prepared session.
// round is the round to start in, cur is that round's exact exercise order and
// startSeq is how many of its exercises were already completed (0 for a fresh
// session, >0 when resuming a paused one). A resume point is persisted after
// every recorded exercise, so an interrupted session can be picked up again
// later with `guitar-coach resume <session-id>`.
func (sh *Shell) runRounds(sess model.Session, round int, cur []model.Exercise, startSeq int) error {
	var err error
	cfg := sess.Config
	for {
		fmt.Printf("\n=== round %d ===\n", round)

		quit := false
		done := 0
		for i := startSeq; i < len(cur); i++ {
			ex := cur[i]
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
			fmt.Print("press Enter when you are ready to start the set (keys here are ignored/Ctrl-C skips): ")
			flushStdio()
			if _, aborted := sh.readLineSig("begin Doing countdown: "); aborted {
				quit = true
				break
			}
			sh.countdown(durationOf(cfg.DurationSec), "Doing: "+label)
			finishedAt := util.NowRFC()

			rest := sh.startRest(label, durationOf(cfg.RestSec))
			opt := promptOpts{status: rest.statusText}
			endBPM, aborted := sh.promptBPM("end BPM", startBPM, opt)
			notes := ""
			if !aborted {
				notes, aborted = sh.readPrompt("notes: ", promptOpts{status: rest.statusText, history: &sh.noteHistory})
			}
			if aborted {
				rest.signal()
				quit = true
				break
			}
			sh.retainNotes(notes)
			sh.waitRest(rest)

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
			if err := sh.api.SetResumePoint(sess.ID, round, idsOf(cur), seq); err != nil {
				return err
			}
			fmt.Printf("  recorded: %s %d -> %d bpm\n", ex.Name, startBPM, endBPM)
			done = seq
		}
		if quit {
			return sh.pauseOrEnd(sess, round, cur, done)
		}

		line, aborted := sh.readLineSig("\nstart another round? [Y/n]: ")
		if aborted {
			return sh.pauseOrEnd(sess, round, cur, len(cur))
		}
		if strings.EqualFold(line, "n") || strings.EqualFold(line, "no") {
			break
		}
		cur = api.ScrambleRotation(cur)
		sh.countdown(durationOf(cfg.BreakSec), "Break -- order scrambled, next round ready")
		round++
		startSeq = 0
	}

	return sh.finishSession(sess)
}

// pauseOrEnd handles an interrupted session. With recorded entries the user
// can choose to keep the session open for a later resume; otherwise the stop
// ends it (or discards it when nothing was recorded).
func (sh *Shell) pauseOrEnd(sess model.Session, round int, cur []model.Exercise, completed int) error {
	if len(sess.Entries) == 0 {
		if err := sh.api.DeleteSession(sess.ID); err != nil {
			return err
		}
		fmt.Println("\nsession discarded -- nothing was recorded")
		return nil
	}
	line, aborted := sh.readLineSig("\npause this session to resume later? [Y/n]: ")
	if !aborted && (strings.EqualFold(line, "n") || strings.EqualFold(line, "no")) {
		return sh.finishSession(sess)
	}
	if err := sh.api.SetResumePoint(sess.ID, round, idsOf(cur), completed); err != nil {
		return err
	}
	next := completed + 1
	if next > len(cur) {
		fmt.Printf("\nsession %s paused -- round %d complete\n", sess.ID, round)
	} else {
		fmt.Printf("\nsession %s paused at round %d, exercise %d/%d\n", sess.ID, round, next, len(cur))
	}
	fmt.Printf("resume anytime with:\n  guitar-coach resume %s\n", sess.ID)
	return nil
}

// finishSession ends a session: discarded when empty, ended and summarised
// otherwise, with an optional open-in-browser prompt.
func (sh *Shell) finishSession(sess model.Session) error {
	if len(sess.Entries) == 0 {
		if err := sh.api.DeleteSession(sess.ID); err != nil {
			return err
		}
		fmt.Println("\nsession discarded -- nothing was recorded")
		return nil
	}

	var err error
	sess, err = sh.api.EndSession(sess.ID)
	if err != nil {
		return err
	}

	total := util.ParseTime(sess.EndedAt).Sub(util.ParseTime(sess.StartedAt))
	fmt.Printf("\nsession %s finished -- %d rounds, %d exercises done, %s total\n",
		sess.ID, model.MaxRound(sess.Entries), len(sess.Entries), util.FormatDuration(total))

	line, aborted := sh.readLineSig("open this session in the browser for review? [y/N]: ")
	if !aborted && (strings.EqualFold(line, "y") || strings.EqualFold(line, "yes")) {
		fmt.Println("opening web UI\u2026")
		if err := sh.openSessionInBrowser(sess.ID); err != nil {
			fmt.Println("warning: could not open the web UI:", err)
		}
	}
	return nil
}

func idsOf(exs []model.Exercise) []string {
	ids := make([]string, 0, len(exs))
	for _, ex := range exs {
		ids = append(ids, ex.ID)
	}
	return ids
}

func durationOf(sec int) time.Duration {
	return time.Duration(sec) * time.Second
}
