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
	var topic string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "list past sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions := sh.api.ListSessions()
			if len(sessions) == 0 {
				fmt.Println("no sessions yet — run: guitar-coach start")
				return nil
			}
			type line struct {
				id, when string
				rounds   int
				sets     int
				dur      string
			}
			var lines []line
			for i := len(sessions) - 1; i >= 0; i-- {
				s := sessions[i]
				entries := s.Entries
				if topic != "" {
					var matched []model.Entry
					for _, e := range entries {
						if ex, err := sh.api.GetExercise(e.ExerciseID); err == nil && ex.Topic == topic {
							matched = append(matched, e)
						}
					}
					if len(matched) == 0 {
						continue
					}
					entries = matched
				}
				lines = append(lines, line{
					id:     s.ID,
					when:   util.ParseTime(s.StartedAt).Local().Format("Jan _2 15:04"),
					rounds: model.MaxRound(entries),
					sets:   len(entries),
					dur:    util.FormatDuration(time.Duration(s.Config.DurationSec) * time.Second),
				})
			}
			if len(lines) == 0 {
				if topic != "" {
					fmt.Printf("no sessions for topic %q\n", topic)
				} else {
					fmt.Println("no sessions yet — run: guitar-coach start")
				}
				return nil
			}
			fmt.Println("sessions:")
			for _, l := range lines {
				fmt.Printf("  %s  %s  rounds=%d  exercises=%d  %s\n", l.id, l.when, l.rounds, l.sets, l.dur)
			}
			return nil
		},
		ValidArgsFunction: sh.noCompletion,
	}
	cmd.Flags().StringVar(&topic, "topic", "", "only sessions containing exercises with this topic")
	cmd.RegisterFlagCompletionFunc("topic", sh.completeTopics)
	return cmd
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
