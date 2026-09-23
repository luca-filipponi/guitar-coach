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
	"github.com/luca-filipponi/guitar-coach/internal/tui"
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
			prevRows := sessionRowsOf(last, -1, nil)
			keep, aborted := tui.RunY(fmt.Sprintf("keep previous rotation (%d exercises)?", len(last)), prevRows)
			if !aborted && !strings.EqualFold(keep, "n") && !strings.EqualFold(keep, "no") {
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
	items := make([]string, 0, len(pool))
	for _, ex := range pool {
		name := ex.Name
		if ex.Topic != "" {
			name = fmt.Sprintf("%s (%s)", ex.Name, ex.Topic)
		}
		items = append(items, name)
	}
	line, aborted := tui.RunPick(fmt.Sprintf("choose up to %d exercises", n), items)
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

	flow := tui.NewFlow()
	flow.Run()
	defer flow.Close()

	flow.Log("\nenter start BPM before each exercise. After the set, the rest timer runs while you record end BPM and notes. Ctrl-C skips a timer or aborts a prompt.")
	flow.Log("\nwarm up before the first set:")

	warmupPlan := planInfoOf(1, order, -1, nil, cfg)
	if res := flow.Next(tui.PhaseRequest{Kind: tui.PhaseCountdown, Label: "Warmup -- open the hands, play lightly", Total: sh.warmup, Rows: warmupPlan.Rows, Plan: warmupPlan}); res.Aborted {
		return nil
	}
	sh.playAlarm()

	return sh.runRounds(flow, sess, 1, order, 0)
}

// runRounds drives the main timed loop for an already-prepared session.
// round is the round to start in, cur is that round's exact exercise order and
// startSeq is how many of its exercises were already completed (0 for a fresh
// session, >0 when resuming a paused one). A resume point is persisted after
// every recorded exercise, so an interrupted session can be picked up again
// later with `guitar-coach resume <session-id>`.
func (sh *Shell) runRounds(flow *tui.Flow, sess model.Session, round int, cur []model.Exercise, startSeq int) error {
	var err error
	cfg := sess.Config
	for {
		flow.Log("\n=== round %d ===", round)

		quit := false
		done := 0
		endBPMs := map[int]int{}
		for i := startSeq; i < len(cur); i++ {
			ex := cur[i]
			seq := i + 1
			label := ex.Name
			if ex.Topic != "" {
				label = fmt.Sprintf("%s (%s)", ex.Name, ex.Topic)
			}
			flow.Log("\n[%d/%d] %s  (%s each)", seq, len(cur), label, util.FormatDuration(durationOf(cfg.DurationSec)))
			for _, l := range sh.historyLines(ex.ID) {
				flow.Log("%s", l)
			}

			suggest := sh.lastEndBPM(ex.ID)
			plan := planInfoOf(round, cur, i, endBPMs, cfg)

			sm := flow.Next(tui.PhaseRequest{
				Kind:      tui.PhaseSession,
				Label:     label,
				Total:     durationOf(cfg.DurationSec),
				Rows:      plan.Rows,
				Plan:      plan,
				Suggest:   suggest,
				NotesSeed: sh.lastNoteFor(ex.ID),
				Alarm:     sh.playAlarm,
			})
			if sm.Aborted {
				quit = true
				break
			}
			startBPM := sm.StartBPM
			endBPM := sm.EndBPM
			endBPMs[i] = endBPM
			notes := sm.Notes
			sh.retainNotes(notes)

			startedAt := util.NowRFC()
			if !sm.StartedAt.IsZero() {
				startedAt = sm.StartedAt.UTC().Format(time.RFC3339)
			}
			finishedAt := sm.FinishedAt.UTC().Format(time.RFC3339)
			last := seq == len(cur)
			if !last {
				if res := flow.Next(tui.PhaseRequest{Kind: tui.PhaseCountdown, Label: "Rest -- " + label, Total: durationOf(cfg.RestSec), Rows: plan.Rows, Plan: plan}); res.Aborted {
					quit = true
					break
				}
				sh.playAlarm()
			}

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
			flow.Log("  recorded: %s %d -> %d bpm", ex.Name, startBPM, endBPM)
			done = seq
		}
		if quit {
			return sh.pauseOrEnd(flow, sess, round, cur, done)
		}

		allRows := sessionRowsOf(cur, -1, endBPMs)
		res := flow.Next(tui.PhaseRequest{Kind: tui.PhaseY, Label: "start another round?", Rows: allRows})
		if res.Aborted {
			return sh.pauseOrEnd(flow, sess, round, cur, len(cur))
		}
		v := strings.ToLower(strings.TrimSpace(res.Value))
		if v == "n" || v == "no" {
			break
		}
		cur = api.ScrambleRotation(cur)
		if res := flow.Next(tui.PhaseRequest{Kind: tui.PhaseCountdown, Label: "Break -- order scrambled, next round ready", Total: durationOf(cfg.BreakSec), Rows: allRows}); res.Aborted {
			return sh.pauseOrEnd(flow, sess, round, cur, len(cur))
		}
		sh.playAlarm()
		round++
		startSeq = 0
	}

	return sh.finishSession(flow, sess)
}

// pauseOrEnd handles an interrupted session. With recorded entries the user
// can choose to keep the session open for a later resume; otherwise the stop
// ends it (or discards it when nothing was recorded).
func (sh *Shell) pauseOrEnd(flow *tui.Flow, sess model.Session, round int, cur []model.Exercise, completed int) error {
	if len(sess.Entries) == 0 {
		if err := sh.api.DeleteSession(sess.ID); err != nil {
			return err
		}
		flow.Log("\nsession discarded -- nothing was recorded")
		return nil
	}
	res := flow.Next(tui.PhaseRequest{Kind: tui.PhaseY, Label: "pause this session to resume later?"})
	if res.Aborted {
		return sh.finishSession(flow, sess)
	}
	v := strings.ToLower(strings.TrimSpace(res.Value))
	if v == "n" || v == "no" {
		return sh.finishSession(flow, sess)
	}
	if err := sh.api.SetResumePoint(sess.ID, round, idsOf(cur), completed); err != nil {
		return err
	}
	next := completed + 1
	if next > len(cur) {
		flow.Log("\nsession %s paused -- round %d complete", sess.ID, round)
	} else {
		flow.Log("\nsession %s paused at round %d, exercise %d/%d", sess.ID, round, next, len(cur))
	}
	flow.Log("resume anytime with:\n  guitar-coach resume %s", sess.ID)
	return nil
}

// finishSession ends a session: discarded when empty, ended and summarised
// otherwise, with an optional open-in-browser prompt.
func (sh *Shell) finishSession(flow *tui.Flow, sess model.Session) error {
	if len(sess.Entries) == 0 {
		if err := sh.api.DeleteSession(sess.ID); err != nil {
			return err
		}
		flow.Log("\nsession discarded -- nothing was recorded")
		return nil
	}

	var err error
	sess, err = sh.api.EndSession(sess.ID)
	if err != nil {
		return err
	}

	total := util.ParseTime(sess.EndedAt).Sub(util.ParseTime(sess.StartedAt))
	flow.Log("\nsession %s finished -- %d rounds, %d exercises done, %s total",
		sess.ID, model.MaxRound(sess.Entries), len(sess.Entries), util.FormatDuration(total))

	res := flow.Next(tui.PhaseRequest{Kind: tui.PhaseYN, Label: "open this session in the browser for review?"})
	v := strings.ToLower(strings.TrimSpace(res.Value))
	if !res.Aborted && (v == "y" || v == "yes") {
		flow.Log("opening web UI\u2026")
		if err := sh.openSessionInBrowser(sess.ID); err != nil {
			flow.Log("warning: could not open the web UI: %v", err)
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

// sessionRowsOf builds the k9s-style side-table rows for a round. doing is the
// index of the current exercise (-1 = none, all pending/done) and endBPMs maps
// a finished exercise's index to its recorded end BPM so done rows can show it.

// planInfoOf builds the always-on-top session plan box for the current round:
// the exercise order with the run state (pending / doing / done) and a footer
// line describing the scramble/break coming after the round.
func planInfoOf(round int, cur []model.Exercise, doing int, endBPMs map[int]int, cfg model.Config) *tui.PlanInfo {
	rows := make([]tui.SessionRow, 0, len(cur))
	for j, exj := range cur {
		st := "pending"
		if doing >= 0 && j == doing {
			st = "doing"
		} else if doing >= 0 && j < doing {
			st = "done"
		}
		tj := ""
		if exj.Topic != "" {
			tj = exj.Topic
		}
		rows = append(rows, tui.SessionRow{
			Seq:    j + 1,
			Name:   exj.Name,
			Topic:  tj,
			Status: st,
			BPM:    endBPMs[j],
			Detail: fmt.Sprintf("%s, then %s rest", util.FormatDuration(durationOf(cfg.DurationSec)), util.FormatDuration(durationOf(cfg.RestSec))),
		})
	}
	footer := fmt.Sprintf("after %d exercises: the order is scrambled for the next round (then %s break)",
		len(cur), util.FormatDuration(durationOf(cfg.BreakSec)))
	return &tui.PlanInfo{Round: round, Rows: rows, Footer: footer}
}

func sessionRowsOf(cur []model.Exercise, doing int, endBPMs map[int]int) []tui.SessionRow {
	rows := make([]tui.SessionRow, 0, len(cur))
	for j, exj := range cur {
		st := "pending"
		if j == doing {
			st = "doing"
		} else if j < doing {
			st = "done"
		}
		tj := ""
		if exj.Topic != "" {
			tj = exj.Topic
		}
		rows = append(rows, tui.SessionRow{Seq: j + 1, Name: exj.Name, Topic: tj, Status: st, BPM: endBPMs[j]})
	}
	return rows
}

func durationOf(sec int) time.Duration {
	return time.Duration(sec) * time.Second
}
