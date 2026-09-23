package cli

import (
	"fmt"
	"strconv"

	"github.com/luca-filipponi/guitar-coach/internal/util"
)

// promptBPM asks for a BPM value, pre-filling the suggested value so the
// user can accept it with Enter or nudge it with the up/down arrows. An
// emptied answer also accepts the suggestion.
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

func (sh *Shell) lastEndBPM(exID string) int {
	points := sh.api.ExerciseProgress(exID)
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].EndBPM > 0 {
			return points[i].EndBPM
		}
	}
	return 0
}

func (sh *Shell) lastNoteFor(exID string) string {
	points := sh.api.ExerciseProgress(exID)
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Notes != "" {
			return points[i].Notes
		}
	}
	return ""
}

func (sh *Shell) historyLines(exID string) []string {
	points := sh.api.ExerciseProgress(exID)
	if len(points) == 0 {
		return []string{"  first time -- no history yet"}
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

	from := len(points) - 3
	if from < 0 {
		from = 0
	}
	lines := []string{line, "  recent history:"}
	for i := len(points) - 1; i >= from; i-- {
		p := points[i]
		bpm := fmt.Sprintf("%d -> %d", p.StartBPM, p.EndBPM)
		if p.StartBPM == 0 && p.EndBPM == 0 {
			bpm = "n/a"
		}
		lines = append(lines, fmt.Sprintf("    %s  %s  %s bpm  r%d  %s", formatPointTime(p.Date), p.SessionID, bpm, p.Round, p.Notes))
	}
	return lines
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
