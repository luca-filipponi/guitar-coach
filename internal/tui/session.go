package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// sessionRow is one line of the k9s-style side table. Status is one of
// pending / doing / done (done rows carry the recorded end BPM).
type sessionRow struct {
	seq    int
	name   string
	topic  string
	status string
	bpm    int
}

// sessionPhase is where inside the single per-exercise window we are.
type sessionPhase int

const (
	phaseStartBPM sessionPhase = iota
	phaseDoing
	phaseEndBPM
	phaseNotes
)

// sessionModel owns the whole per-exercise window. Instead of shipping four
// short-lived programs at the CLI (start BPM, Doing, end BPM, notes) it runs
// all four phases in one alt-screen window so the terminal never tears/clears
// between them. The contract is locked by the hermetic regression test: the
// window opens on the start-BPM field, Enter (or Esc) begins the Doing count
// down immediately, Enter/Space/Tab are harmless no-ops while Doing, Esc
// finishes the set early, Ctrl-C aborts the session without echoing ^C, and
// the countdown ticks on its own until it expires. The exercise is always
// recorded when the window closes (natural end, early finish, notes done);
// only Ctrl-C skips recording.
type sessionModel struct {
	rows    []sessionRow
	label   string
	total   time.Duration
	left    time.Duration
	phase   sessionPhase
	aborted bool

	bpm       string // editing buffer for start/end BPM
	suggest   int    // suggested start BPM, pre-seeded at open
	notes     string // notes editing buffer, pre-seeded from history
	blink     bool
	alarm     func()
	startedAt time.Time
	endedAt   time.Time

	startBPM int
	endBPM   int
}

func newSessionModel(rows []sessionRow, label string, total time.Duration, startBPM int, notesSeed string, alarm func()) sessionModel {
	return sessionModel{rows: rows, label: label, total: total, left: total, phase: phaseStartBPM, suggest: startBPM, notes: notesSeed, alarm: alarm}
}

func (sessionModel) Init() tea.Cmd { return tea.Batch(tickCmd(), blinkCmd()) }

// beginDoing kicks off the Doing countdown: records when the set started,
// resets the left budget and re-arms the tick chain.
func (m *sessionModel) beginDoing() tea.Cmd {
	m.phase = phaseDoing
	m.left = m.total
	m.startedAt = time.Now().UTC()
	return tickCmd()
}

// finishDoing runs when the Doing phase ends for any reason: marks the set as
// over, fires the alarm, and moves on to the end-BPM field.
func (m *sessionModel) finishDoing() {
	m.endedAt = time.Now().UTC()
	m.phase = phaseEndBPM
	if m.alarm != nil {
		m.alarm()
	}
}

func (m sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case blinkMsg:
		m.blink = !m.blink
		return m, blinkCmd()
	case tickMsg:
		if m.phase == phaseDoing {
			m.left -= Tick
			if m.left <= 0 {
				m.left = 0
				m.finishDoing()
				return m, nil
			}
			return m, tickCmd()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "\r", "\n":
			switch m.phase {
			case phaseStartBPM:
				m.startBPM = m.acceptedBPM()
				m.bpm = ""
				return m, m.beginDoing()
			case phaseDoing:
				return m, nil
			case phaseEndBPM:
				m.endBPM = m.acceptedBPM()
				m.bpm = ""
				m.phase = phaseNotes
				return m, nil
			default: // phaseNotes
				m.aborted = false
				return m, tea.Quit
			}
		case "esc", "delete":
			switch m.phase {
			case phaseStartBPM:
				m.startBPM = m.suggest
				m.bpm = ""
				return m, m.beginDoing()
			case phaseDoing:
				m.finishDoing()
				return m, nil
			case phaseEndBPM:
				m.endBPM = m.acceptedBPM()
				m.bpm = ""
				m.phase = phaseNotes
				return m, nil
			default: // phaseNotes
				m.aborted = false
				return m, tea.Quit
			}
		case "ctrl+c", "\x03":
			m.aborted = true
			return m, tea.Quit
		case "backspace":
			if len(m.bpm) > 0 && (m.phase == phaseStartBPM || m.phase == phaseEndBPM) {
				m.bpm = m.bpm[:len(m.bpm)-1]
			} else if len(m.notes) > 0 && m.phase == phaseNotes {
				m.notes = m.notes[:len(m.notes)-1]
			}
			return m, nil
		case "up", "down":
			if m.phase == phaseStartBPM || m.phase == phaseEndBPM {
				d := 1
				if msg.String() == "down" {
					d = -1
				}
				n := atoiDefault(m.bpm)
				if n == 0 && m.suggest != 0 {
					n = m.suggest
				}
				m.bpm = itoa(n + d)
			}
			return m, nil
		}
		if m.phase == phaseStartBPM || m.phase == phaseEndBPM {
			c := msg.String()
			if len(c) == 1 && c >= "0" && c <= "9" {
				m.bpm += c
			}
			return m, nil
		}
		if m.phase == phaseNotes {
			if len(msg.String()) != 1 {
				return m, nil
			}
			m.notes += msg.String()
		}
		return m, nil
	default:
		return m, nil
	}
}

// acceptedBPM returns the validated value of the current numeric field: the
// typed number when non-zero, the suggestion otherwise. The end-BPM field
// suggests the BPM the set actually started at.
func (m sessionModel) acceptedBPM() int {
	n := atoiDefault(m.bpm)
	if m.phase == phaseEndBPM && m.startBPM != 0 {
		return m.startBPM
	}
	if n == 0 && m.suggest != 0 {
		return m.suggest
	}
	return n
}

func (m sessionModel) View() string {
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(GuitarArt, "\n", "\n  ") + "\n")
	switch m.phase {
	case phaseStartBPM:
		b.WriteString("  " + m.bpmLabel() + "\n")
		b.WriteString("  start BPM: ")
		b.WriteString(m.field())
		b.WriteString("\n  Enter to begin the countdown | up/down to adjust | Ctrl-C quit\n")
	case phaseDoing:
		b.WriteString("  Doing -- " + durText(m.left) + "\n")
		b.WriteString("  " + progressBar(m.total-m.left, m.total) + " " + durText(m.left) + "\n")
		b.WriteString("  start BPM: " + itoa(m.startBPM) + "\n")
		b.WriteString("\n  exercises\n")
		b.WriteString(sideTable(m.rows))
		b.WriteString("\n  Esc to finish early (always recorded) | Ctrl-C quit\n")
	case phaseEndBPM:
		b.WriteString("  " + m.bpmLabel() + "\n")
		b.WriteString("  end BPM: ")
		b.WriteString(m.field())
		b.WriteString("\n  Enter to record, then notes | Esc to keep " + itoa(m.acceptedBPM()) + " | Ctrl-C quit\n")
	default: // phaseNotes
		b.WriteString("  " + m.bpmLabel() + "\n")
		b.WriteString("  notes: " + m.notes + cursor(m.blink) + "\n")
		b.WriteString("  Enter to finish (always recorded) | Esc to skip notes | Ctrl-C quit\n")
	}
	return b.String()
}

// bpmLabel is the exercise heading shared by every phase of the window.
func (m sessionModel) bpmLabel() string {
	return m.label
}

// field renders the numeric field with its suggestion and a blinking cursor.
func (m sessionModel) field() string {
	s := m.phaseSuggestion()
	if m.bpm == "" && s > 0 {
		return itoa(s) + " (suggested, up/down to adjust, Enter) " + cursor(m.blink)
	}
	return m.bpm + cursor(m.blink)
}

// phaseSuggestion is the value to show/accept when the field is left empty:
// the original suggestion for the start-BPM field, or the BPM the set actually
// started at for the end-BPM field.
func (m sessionModel) phaseSuggestion() int {
	if m.phase == phaseEndBPM && m.startBPM != 0 {
		return m.startBPM
	}
	return m.suggest
}

// sideTable renders the k9s-style exercise list with per-row status markers:
// green ▶ on the current (doing) exercise, ✓ on done ones, plain elsewhere.
func sideTable(rows []sessionRow) string {
	var b strings.Builder
	for _, r := range rows {
		switch r.status {
		case "done":
			mark := "done - " + strconv.Itoa(r.bpm) + " bpm"
			name := r.name
			if r.topic != "" {
				name += " (" + r.topic + ")"
			}
			b.WriteString(fmt.Sprintf("  \u2713  %02d. %-34s %s\n", r.seq, name, mark))
		case "doing":
			mark := "doing"
			name := r.name
			if r.topic != "" {
				name += " (" + r.topic + ")"
			}
			line := fmt.Sprintf("  \u25b6  %02d. %-34s %s", r.seq, name, mark)
			b.WriteString(green(line) + "\n")
		default:
			name := r.name
			if r.topic != "" {
				name += " (" + r.topic + ")"
			}
			b.WriteString(fmt.Sprintf("       %02d. %-34s pending\n", r.seq, name))
		}
	}
	return b.String()
}

// durText renders a duration as m:ss, falling back to seconds below a minute.
func durText(d time.Duration) string {
	t := int(d / time.Second)
	if t < 60 {
		return itoa(t) + "s"
	}
	return itoa(t/60) + "m" + zero2(t%60) + "s"
}

// green wraps s in ANSI green so the row the user is actually on pops.
func green(s string) string {
	return "\x1b[32m" + s + "\x1b[0m"
}

func zero2(n int) string {
	if n < 0 {
		n = 0
	}
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func atoiDefault(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// itoa renders n as a decimal string with no leading zeros (0 -> "0").
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// SessionRequest is what the CLI hands the full per-exercise window. StartBPM
// is the suggested value the user last recorded for this exercise; NotesSeed is
// the previous session's note text for quick reuse; Alarm is fired the moment
// the Doing countdown ends (never blocking the window).
type SessionRequest struct {
	Label     string
	Total     time.Duration
	Rows      []SessionRow
	StartBPM  int
	NotesSeed string
	Alarm     func()
}

// SessionRow is the CLI-facing copy of the k9s side-table row.
type SessionRow struct {
	Seq    int
	Name   string
	Topic  string
	Status string // pending | doing | done
	BPM    int    // end BPM once done
}

// SessionResult is what the window hands back. Aborted means Ctrl-C quit and
// the exercise must not be recorded; otherwise the exercise is recorded, with
// the exact BPMs and notes the user entered plus when the set ran.
type SessionResult struct {
	Aborted    bool
	StartBPM   int
	EndBPM     int
	Notes      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// RunSession runs the per-exercise window as a one-shot Bubble Tea program
// against the caller's real terminal and returns its gate result.
func RunSession(req SessionRequest) SessionResult {
	rows := make([]sessionRow, 0, len(req.Rows))
	for _, r := range req.Rows {
		rows = append(rows, sessionRow{seq: r.Seq, name: r.Name, topic: r.Topic, status: r.Status, bpm: r.BPM})
	}
	m := newSessionModel(rows, req.Label, req.Total, req.StartBPM, req.NotesSeed, req.Alarm)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	done, err := p.Run()
	if err != nil {
		return SessionResult{Aborted: true}
	}
	rm, ok := done.(sessionModel)
	if !ok {
		return SessionResult{Aborted: true}
	}
	return SessionResult{Aborted: rm.aborted, StartBPM: rm.startBPM, EndBPM: rm.endBPM, Notes: rm.notes, StartedAt: rm.startedAt, FinishedAt: rm.endedAt}
}