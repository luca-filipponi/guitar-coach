package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/util"
)

type input struct {
	lines chan string
}

func newInput() *input {
	it := &input{lines: make(chan string, 16)}
	sc := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 65536)
	sc.Buffer(buf, 1024*1024)
	go func() {
		for sc.Scan() {
			it.lines <- sc.Text()
		}
	}()
	return it
}

func (sh *Shell) readLineSig(label string) (string, bool) {
	fmt.Print(label)
	select {
	case line := <-sh.in.lines:
		return strings.TrimSpace(line), false
	case <-sh.sig:
		return "", true
	}
}

func (sh *Shell) promptBPM(label string, suggest int) (int, bool) {
	hint := ""
	if suggest > 0 {
		hint = fmt.Sprintf(" [suggest: %d]", suggest)
	}
	for {
		line, aborted := sh.readLineSig(fmt.Sprintf("%s%s: ", label, hint))
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
			bell()
			return false
		}
		fmt.Printf("\r\033[K  %s -- %s remaining (Ctrl-C to skip)", label, util.FormatDuration(remaining))
		select {
		case <-ticker.C:
		case <-sh.sig:
			fmt.Printf("\r\033[K  %s -- skipped\n", label)
			bell()
			return true
		}
	}
}

// restWindow announces the rest and returns a channel that fires once the rest
// has elapsed (or Ctrl-C skips it). It runs in the background so the BPM/notes
// prompts can be answered during the rest without blocking the timer.
func (sh *Shell) restWindow(label string, dur time.Duration) <-chan struct{} {
	fmt.Printf("\n  Rest %s -- record end BPM and notes for %s (Ctrl-C to skip rest)\n",
		util.FormatDuration(dur), label)
	done := make(chan struct{}, 1)
	go func() {
		select {
		case <-time.After(dur):
			bell()
		case <-sh.sig:
		}
		done <- struct{}{}
	}()
	return done
}

func bell() {
	fmt.Print("\a\a\a")
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

	lastBPM, lastDate := 0, ""
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].EndBPM > 0 {
			lastBPM = points[i].EndBPM
			lastDate = points[i].Date[:10]
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
		line += fmt.Sprintf("%d bpm on %s", lastBPM, lastDate)
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
		note := p.Notes
		if len(note) > 40 {
			note = note[:40] + "..."
		}
		bpm := fmt.Sprintf("%d -> %d", p.StartBPM, p.EndBPM)
		if p.StartBPM == 0 && p.EndBPM == 0 {
			bpm = "n/a"
		}
		fmt.Printf("    %s  %s bpm  r%d  %s\n", p.Date[:10], bpm, p.Round, note)
	}
}
