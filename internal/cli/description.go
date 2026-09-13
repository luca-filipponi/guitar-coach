package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (sh *Shell) newDescriptionCmd() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "description <exercise> [text...]",
		Short: "show or set an exercise's description",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("requires an exercise name or id")
			}
			ex, err := sh.api.FindExercise(args[0])
			if err != nil {
				return err
			}

			if len(args) >= 2 {
				if clear {
					return errors.New("--clear cannot be combined with a description value")
				}
				if _, err := sh.api.SetDescription(ex.ID, strings.Join(args[1:], " ")); err != nil {
					return err
				}
				fmt.Printf("description set: %s\n", ex.Name)
				return nil
			}

			if clear {
				if _, err := sh.api.SetDescription(ex.ID, ""); err != nil {
					return err
				}
				fmt.Printf("description cleared: %s\n", ex.Name)
				return nil
			}

			if ex.Description == "" {
				fmt.Printf("%s: no description\n", ex.Name)
			} else {
				fmt.Printf("%s: %s\n", ex.Name, ex.Description)
			}
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return sh.completeExerciseNames(cmd, args, toComplete)
			}
			return sh.noCompletion(cmd, args, toComplete)
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the exercise's description")
	return cmd
}
