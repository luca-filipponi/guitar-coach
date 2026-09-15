package cli

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/util"
)

// promptBPM asks for a BPM value, pre-filling the suggested value so the
// user can accept it with Enter or nudge it with the up/down arrows. An
// emptied answer also accepts the suggestion. opts can carry the live
// rest-clock status line.
func (sh *Shell) promptBPM(label string, suggest int, opts ...promptOpts) (int, bool) {
	opt := promptOpts{}
	if len(opts) > 0 {
		opt = opts[0]
	}
	if suggest > 0 {
		opt.initial = strconv.Itoa(suggest)
	}
	opt.numeric = true
	for {
		line, aborted := sh.readPrompt(label+": ", opt)
		if aborted {
			return suggest, true
		}
		if line == "" {
			return suggest, false
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 0 {
			return n, false
		}
		fmt.Println("  please enter a number (or empty to skip)")
	}
}

func (sh *Shell) countdown(dur time.Duration, label string) bool {
	end := time.Now().Add(dur)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	fmt.Printf("  %s\n", label)
	for {
		remaining := time.Until(end)
		if remaining <= 0 {
			fmt.Printf("\r\033[K  %s -- done\n", label)
			sh.playAlarm()
			return false
		}
		fmt.Printf("\r\033[K  %s -- %s remaining (Ctrl-C to skip)", label, util.FormatDuration(remaining))
		select {
		case <-ticker.C:
		case <-sh.sig:
			fmt.Printf("\r\033[K  %s -- skipped\n", label)
			sh.playAlarm()
			return true
		}
	}
}

// restClock drives the rest timer. It keeps the end time so the prompts can
// show a live "rest X left" line, and a done channel that fires when the rest
// elapses (or the user skips it). done is closed (never sent to) so waiters
// never lose the wakeup.
type restClock struct {
	end   time.Time
	label string
	done  chan struct{}
	once  sync.Once
}

func (sh *Shell) startRest(label string, dur time.Duration) *restClock {
	rc := &restClock{end: time.Now().Add(dur), label: label, done: make(chan struct{})}
	go func() {
		select {
		case <-time.After(dur):
			sh.playAlarm()
			rc.signal()
		case <-rc.done:
		}
	}()
	return rc
}

// signal marks the rest as done (from the timer or a skip); the first call
// closes done, waking every waiter.
func (rc *restClock) signal() {
	rc.once.Do(func() { close(rc.done) })
}

// statusText is fed to the prompt editor's status line so the user sees the
// rest time ticking while they record BPM and notes.
func (rc *restClock) statusText() string {
	rem := time.Until(rc.end)
	if rem <= 0 {
		return fmt.Sprintf("rest over for %s", rc.label)
	}
	return fmt.Sprintf("%s: rest %s left", rc.label, util.FormatDuration(rem))
}

// waitRest blocks until the rest elapses (or Ctrl-C skips it), redrawing a
// live countdown on a single line.
func (sh *Shell) waitRest(rc *restClock) {
	for {
		select {
		case <-rc.done:
			fmt.Print("\r\033[K")
			return
		case <-time.After(time.Second):
			fmt.Printf("\r\033[K  %s (Ctrl-C to skip)", rc.statusText())
			select {
			case <-sh.sig:
				rc.signal()
			default:
			}
		}
	}
}

func (sh *Shell) lastEndBPM(exID string) int {
	points := sh.api.ExerciseProgress(exID)
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].EndBPM > 0 {
			return points[i].EndBPM
		}
	}
	return 0
}

func (sh *Shell) showHistory(exID string) {
	points := sh.api.ExerciseProgress(exID)
	if len(points) == 0 {
		fmt.Println("  first time -- no history yet")
		return
	}

	lastBPM, lastWhen := 0, ""
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].EndBPM > 0 {
			lastBPM = points[i].EndBPM
			lastWhen = formatPointTime(points[i].Date)
			break
		}
	}
	lastNotes := ""
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Notes != "" {
			lastNotes = points[i].Notes
			break
		}
	}
	line := "  last: "
	if lastBPM > 0 {
		line += fmt.Sprintf("%d bpm on %s", lastBPM, lastWhen)
	} else {
		line += "no bpm yet"
	}
	if lastNotes != "" {
		line += fmt.Sprintf(" -- notes: %s", lastNotes)
	}
	fmt.Println(line)

	from := len(points) - 3
	if from < 0 {
		from = 0
	}
	fmt.Println("  recent history:")
	for i := len(points) - 1; i >= from; i-- {
		p := points[i]
		bpm := fmt.Sprintf("%d -> %d", p.StartBPM, p.EndBPM)
		if p.StartBPM == 0 && p.EndBPM == 0 {
			bpm = "n/a"
		}
		fmt.Printf("    %s  %s  %s bpm  r%d  %s\n", formatPointTime(p.Date), p.SessionID, bpm, p.Round, p.Notes)
	}
}

// formatPointTime renders a progress point's timestamp as a local date+time,
// so the same exercise recorded in different sessions (or rounds) is
// distinguishable at a glance.
func formatPointTime(rfc string) string {
	t := util.ParseTime(rfc)
	if t.IsZero() {
		return rfc
	}
	return t.Local().Format("2006-01-02 15:04")
}
