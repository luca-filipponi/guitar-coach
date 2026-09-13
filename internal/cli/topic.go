package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func (sh *Shell) newTopicCmd() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "topic <exercise> [topic]",
		Short: "show or set an exercise's topic",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ex, err := sh.api.FindExercise(args[0])
			if err != nil {
				return err
			}

			if len(args) == 2 {
				if clear {
					return errors.New("--clear cannot be combined with a topic value")
				}
				if _, err := sh.api.SetTopic(ex.ID, args[1]); err != nil {
					return err
				}
				fmt.Printf("topic set: %s -> %s\n", ex.Name, args[1])
				return nil
			}

			if clear {
				if _, err := sh.api.SetTopic(ex.ID, ""); err != nil {
					return err
				}
				fmt.Printf("topic cleared: %s\n", ex.Name)
				return nil
			}

			if ex.Topic == "" {
				fmt.Printf("%s: no topic\n", ex.Name)
			} else {
				fmt.Printf("%s: %s\n", ex.Name, ex.Topic)
			}
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return sh.completeExerciseNames(cmd, args, toComplete)
			}
			return sh.completeTopics(cmd, args, toComplete)
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the exercise's topic")
	return cmd
}
