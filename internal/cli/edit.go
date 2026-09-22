package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/tui"
)

func (sh *Shell) newEditCmd() *cobra.Command {
	var startP, endP int
	cmd := &cobra.Command{
		Use:   "edit <session-id> <entry>",
		Short: "fix the BPM of a past session entry",
		Long: `edit lets you correct start/end BPM of a recorded entry.

<entry> is the number shown by ` + "`guitar-coach show <session-id>`" + ` (1 = first set).
Use --start and --end to set values non-interactively.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := sh.api.GetSession(args[0])
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 || n > len(sess.Entries) {
				return fmt.Errorf("entry number must be between 1 and %d (use `guitar-coach show %s`)", len(sess.Entries), sess.ID)
			}
			idx := n - 1
			e := sess.Entries[idx]
			name := e.Name
			if name == "" {
				name = "exercise " + e.ExerciseID
			}
			bpm := "n/a"
			if e.StartBPM > 0 || e.EndBPM > 0 {
				bpm = fmt.Sprintf("%d -> %d", e.StartBPM, e.EndBPM)
			}
			fmt.Printf("entry %d (round %d): %s  %s bpm\n", n, e.Round, name, bpm)
			if e.Notes != "" {
				fmt.Printf("  notes: %s\n", e.Notes)
			}

			startSet := cmd.Flags().Changed("start")
			endSet := cmd.Flags().Changed("end")
			if startP < 0 || endP < 0 {
				return errors.New("BPM values cannot be negative")
			}
			if !startSet && !endSet {
			sp, aborted := tui.RunBPM("new start BPM (Enter keeps current)", e.StartBPM)
			if aborted {
				return errors.New("aborted")
			}
			if sp != e.StartBPM {
				startP, startSet = sp, true
			}
			ep, aborted := tui.RunBPM("new end BPM (Enter keeps current)", e.EndBPM)
			if aborted {
				return errors.New("aborted")
			}
			if ep != e.EndBPM {
				endP, endSet = ep, true
			}
			if !startSet && !endSet {
				fmt.Println("nothing changed")
				return nil
			}
		}

			updated, err := sh.api.UpdateSessionEntry(sess.ID, idx, func(en *model.Entry) {
				if startSet {
					en.StartBPM = startP
				}
				if endSet {
					en.EndBPM = endP
				}
			})
			if err != nil {
				return err
			}
			ne := updated.Entries[idx]
			fmt.Printf("updated: %s  %d -> %d bpm\n", ne.Name, ne.StartBPM, ne.EndBPM)
			return nil
		},
		ValidArgsFunction: sh.completeSessionIDs,
	}
	cmd.Flags().IntVar(&startP, "start", 0, "new start BPM (0 keeps it)")
	cmd.Flags().IntVar(&endP, "end", 0, "new end BPM (0 keeps it)")
	return cmd
}
