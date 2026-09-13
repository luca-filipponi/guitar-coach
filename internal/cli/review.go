package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

func (sh *Shell) newHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "list past sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions := sh.api.ListSessions()
			if len(sessions) == 0 {
				fmt.Println("no sessions yet — run: guitar-coach start")
				return nil
			}
			fmt.Println("sessions:")
			for i := len(sessions) - 1; i >= 0; i-- {
				s := sessions[i]
				start := util.ParseTime(s.StartedAt)
				fmt.Printf("  %s  %s  rounds=%d  exercises=%d  %s\n",
					s.ID,
					start.Local().Format("Jan _2 15:04"),
					model.MaxRound(s.Entries),
					len(s.Entries),
					util.FormatDuration(time.Duration(s.Config.DurationSec)*time.Second),
				)
			}
			return nil
		},
		ValidArgsFunction: sh.noCompletion,
	}
}

func (sh *Shell) newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <session-id>",
		Short: "show details of one session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := sh.api.GetSession(args[0])
			if err != nil {
				return err
			}
			sh.printSession(sess)
			return nil
		},
		ValidArgsFunction: sh.completeSessionIDs,
	}
}

func (sh *Shell) printSession(s model.Session) {
	fmt.Printf("session: %s\n", s.ID)
	fmt.Printf("started: %s\n", formatTimeRFC(s.StartedAt))
	if s.EndedAt != "" {
		fmt.Printf("ended  : %s\n", formatTimeRFC(s.EndedAt))
	}
	fmt.Printf("plan   : %d exercises, %s each, %s rest, %s break\n",
		s.Config.ExercisesPerRound,
		util.FormatDuration(time.Duration(s.Config.DurationSec)*time.Second),
		util.FormatDuration(time.Duration(s.Config.RestSec)*time.Second),
		util.FormatDuration(time.Duration(s.Config.BreakSec)*time.Second),
	)
	names := make([]string, 0, len(s.Order))
	for _, id := range s.Order {
		if ex, err := sh.api.GetExercise(id); err == nil {
			names = append(names, ex.Name)
		}
	}
	fmt.Printf("order  : %s\n", strings.Join(names, " > "))

	round := 0
	for _, e := range s.Entries {
		if e.Round != round {
			round = e.Round
			fmt.Printf("\nround %d\n", round)
		}
		bpm := "n/a"
		if e.StartBPM > 0 || e.EndBPM > 0 {
			bpm = fmt.Sprintf("%d -> %d", e.StartBPM, e.EndBPM)
		}
		fmt.Printf("  %d. %s  %s bpm", e.Sequence, e.Name, bpm)
		if e.Notes != "" {
			fmt.Printf("  [%s]", e.Notes)
		}
		fmt.Printf("\n")
	}
}

func formatTimeRFC(s string) string {
	t := util.ParseTime(s)
	if t.IsZero() {
		return "n/a"
	}
	return t.Local().Format(time.RFC1123)
}
