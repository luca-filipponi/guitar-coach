package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
)

func (sh *Shell) newAddCmd() *cobra.Command {
	var topic, description string
	cmd := &cobra.Command{
		Use:   "add <name>...",
		Short: "add one or more exercises",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range args {
				ex, err := sh.api.CreateExercise(name, topic, description)
				if errors.Is(err, api.ErrExists) {
					fmt.Printf("already exists: %s\n", name)
					continue
				}
				if err != nil {
					return err
				}
				fmt.Printf("added: %s (%s)\n", ex.Name, ex.ID)
			}
			return nil
		},
		ValidArgsFunction: sh.noCompletion,
	}
	cmd.Flags().StringVar(&topic, "topic", "", "topic of the exercises (e.g. alternate picking)")
	cmd.Flags().StringVar(&description, "description", "", "short description of the exercise")
	cmd.RegisterFlagCompletionFunc("topic", sh.completeTopics)
	return cmd
}

func (sh *Shell) newListCmd() *cobra.Command {
	var topic string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list exercises (optionally filter by topic)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exercises := sh.api.ListExercises()
			if len(exercises) == 0 {
				fmt.Println(`no exercises yet — add some: guitar-coach add "Warmup"`)
				return nil
			}
			if topic != "" {
				filtered := make([]model.Exercise, 0, len(exercises))
				for _, ex := range exercises {
					if ex.Topic == topic {
						filtered = append(filtered, ex)
					}
				}
				exercises = filtered
				if len(exercises) == 0 {
					fmt.Printf("no exercises with topic %q\n", topic)
					return nil
				}
			}
			fmt.Println("exercises:")
			for _, ex := range exercises {
				fmt.Printf("  %s  %s", ex.ID, ex.Name)
				if ex.Topic != "" {
					fmt.Printf("  [%s]", ex.Topic)
				}
				fmt.Println()
				if ex.Description != "" {
					fmt.Printf("      %s\n", ex.Description)
				}
			}
			return nil
		},
		ValidArgsFunction: sh.noCompletion,
	}
	cmd.Flags().StringVar(&topic, "topic", "", "only exercises with this topic")
	cmd.RegisterFlagCompletionFunc("topic", sh.completeTopics)
	return cmd
}

func (sh *Shell) newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name|id>",
		Short: "remove an exercise",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ex, err := sh.api.FindExercise(args[0])
			if err != nil {
				return err
			}
			if err := sh.api.DeleteExercise(ex.ID); err != nil {
				return err
			}
			fmt.Printf("removed: %s\n", ex.Name)
			return nil
		},
		ValidArgsFunction: sh.completeExerciseNames,
	}
}
