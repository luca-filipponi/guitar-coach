package cli

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/store"
)

type Shell struct {
	api         *api.API
	st          *store.Store
	sig         chan os.Signal
	noteHistory []string
	warmup      time.Duration
	alarmSound  string
	alarmVolume float64
	alarmCount  int
	alarmGap    time.Duration
}

func New(st *store.Store) *Shell {
	sh := &Shell{
		api:         api.New(st),
		st:          st,
		sig:         make(chan os.Signal, 1),
		warmup:      5 * time.Minute,
		alarmSound:  "/System/Library/Sounds/Ping.aiff",
		alarmVolume: 2.5,
		alarmCount:  3,
		alarmGap:    220 * time.Millisecond,
	}
	signal.Notify(sh.sig, os.Interrupt, syscall.SIGTERM)
	return sh
}

func (sh *Shell) Root() *cobra.Command {
	root := &cobra.Command{
		Use:     "guitar-coach",
		Short:   "Guitar practice session tracker",
		Version: version,
		Long: `guitar-coach is a CLI for tracking guitar practice sessions.

It helps you run timed practice sessions, records start/end BPM and notes
per exercise, and keeps a local history you can review from the terminal
or from the built-in web UI.

Data is stored in ~/.guitar-coach (override with --dir or
the GUITAR_COACH_DIR environment variable).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(sh.newAddCmd())
	root.AddCommand(sh.newListCmd())
	root.AddCommand(sh.newRemoveCmd())
	root.AddCommand(sh.newTopicCmd())
	root.AddCommand(sh.newDescriptionCmd())
	root.AddCommand(sh.newStartCmd())
	root.AddCommand(sh.newEditCmd())
	root.AddCommand(sh.newSeedCmd())
	root.AddCommand(sh.newHistoryCmd())
	root.AddCommand(sh.newShowCmd())
	root.AddCommand(sh.newServeCmd())
	root.AddCommand(sh.newStopCmd())
	root.AddCommand(versionCmd())

	return root
}

func (sh *Shell) completeExerciseNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var names []string
	for _, ex := range sh.api.ListExercises() {
		names = append(names, ex.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func (sh *Shell) completeSessionIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var ids []string
	for _, s := range sh.api.ListSessions() {
		ids = append(ids, s.ID)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

// completeTopics suggests topics already used by exercises. The directive
// still lets the user type a brand-new topic.
func (sh *Shell) completeTopics(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	seen := make(map[string]bool)
	var topics []string
	for _, ex := range sh.api.ListExercises() {
		if ex.Topic == "" || seen[ex.Topic] {
			continue
		}
		seen[ex.Topic] = true
		topics = append(topics, ex.Topic)
	}
	return topics, cobra.ShellCompDirectiveNoFileComp
}

func (sh *Shell) noCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
