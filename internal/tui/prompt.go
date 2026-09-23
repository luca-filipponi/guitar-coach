package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// promptKind selects which in-window prompt is running.
type promptKind int

const (
	promptCountdown promptKind = iota
	promptBPM
	promptY
	promptText
	promptPick
)

// promptResult is the window's answer, mirroring the CLI contract exactly:
// Aborted is the "Ctrl-C / skip" bool the cli checks first; for promptBPM the
// Value is the accepted BPM (empty = accept suggestion); for promptY it is the
// normalized first letter.
type promptResult struct {
	Aborted bool
	Skipped bool
	Value   string
}

type promptModel struct {
	kind     promptKind
	label    string
	suggest  string
	defY     bool
	numeric  bool
	initStep bool
	current  string
	items    []string
	rows     []sessionRow
	plan     *PlanInfo
	left     time.Duration
	total    time.Duration
	done     bool
	blink    bool
	last     *promptDoneMsg
}

type promptDoneMsg struct{ result promptResult }

type blinkMsg struct{}

func blinkCmd() tea.Cmd {
	return tea.Tick(time.Second/2, func(time.Time) tea.Msg { return blinkMsg{} })
}

func (m promptModel) Init() tea.Cmd { return tea.Batch(tickCmd(), blinkCmd()) }

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case blinkMsg:
		if m.done {
			return m, nil
		}
		m.blink = !m.blink
		return m, blinkCmd()
	case tickMsg:
		if m.kind == promptCountdown {
			if !m.initStep {
				return m, tickCmd()
			}
			m.left -= Tick
			if m.left <= 0 {
				m.left = 0
				m.done = true
				m.last = &promptDoneMsg{}
				return m, tea.Quit
			}
			return m, tickCmd()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "\r", "\n":
			switch m.kind {
			case promptCountdown:
				if !m.initStep {
					m.initStep = true
					return m, nil
				}
				return m, nil
			case promptBPM:
				m.done = true
				m.last = &promptDoneMsg{promptResult{Value: m.current}}
				return m, tea.Quit
			case promptY:
				m.done = true
				v := "y"
				if !m.defY {
					v = "n"
				}
				if c := strings.TrimSpace(m.current); c != "" {
					v = string(c[0])
				}
				m.last = &promptDoneMsg{promptResult{Value: v}}
				return m, tea.Quit
			case promptPick:
				m.done = true
				m.last = &promptDoneMsg{promptResult{Value: m.current}}
				return m, tea.Quit
			default:
				m.done = true
				m.last = &promptDoneMsg{promptResult{Value: m.current}}
				return m, tea.Quit
			}
		case "esc", "delete":
			m.done = true
			m.last = &promptDoneMsg{promptResult{Skipped: true}}
			return m, tea.Quit
		case "ctrl+c", "\x03":
			m.done = true
			m.last = &promptDoneMsg{promptResult{Aborted: true}}
			return m, tea.Quit
		case "backspace":
			if len(m.current) > 0 {
				m.current = m.current[:len(m.current)-1]
			}
			return m, nil
		case "up", "down":
			if m.numeric && m.suggest != "" {
				d := 1
				if msg.String() == "down" {
					d = -1
				}
				n := atoiDefault(m.current)
				if n == 0 && m.suggest != "" {
					n = atoiDefault(m.suggest)
				}
				m.current = itoa(n + d)
			}
			return m, nil
		}
		if !m.done {
			c := msg.String()
			if m.kind == promptY {
				c = strings.TrimSpace(c)
				if c == "" {
					return m, nil
				}
				if l := strings.ToLower(c[0:1]); l == "y" || l == "n" {
					m.done = true
					m.last = &promptDoneMsg{promptResult{Value: l}}
					return m, tea.Quit
				}
				return m, nil
			}
			if m.kind == promptPick {
				if len(c) == 1 && (c >= "0" && c <= "9" || c == "," || c == " ") {
					m.current += c
				}
				return m, nil
			}
			if m.numeric {
				if len(c) == 1 && c >= "0" && c <= "9" {
					m.current += c
				}
				return m, nil
			}
			if len(c) != 1 {
				return m, nil
			}
			m.current += c
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m promptModel) View() string {
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(GuitarArt, "\n", "\n  ") + "\n")
	if m.plan != nil {
		b.WriteString(planBox(m.plan))
	}
	switch m.kind {
	case promptCountdown:
		b.WriteString("  " + m.label + "\n")
		if m.initStep {
			b.WriteString("  " + progressBar(m.total-m.left, m.total) + " " + durText(m.left) + "\n")
			b.WriteString("  Esc to skip\n")
		} else {
			b.WriteString("  press Enter to begin, Esc to skip\n")
		}
	case promptPick:
		b.WriteString("  " + m.label + "\n")
		for i, it := range m.items {
			b.WriteString(fmt.Sprintf("  %2d. %s\n", i+1, it))
		}
		b.WriteString("  choose indices, comma-separated: " + m.current + cursor(m.blink) + "\n")
		b.WriteString("  Enter to accept | Esc to skip\n")
	case promptY:
		b.WriteString("  " + m.label + "\n")
		b.WriteString("\n  (Enter = y, type y/n, Esc to skip)\n")
	default:
		b.WriteString("  " + m.label + ": ")
		cur := cursor(m.blink)
		if m.numeric && m.current == "" && m.suggest != "" {
			b.WriteString(m.suggest + " (suggested, up/down to adjust, Enter) " + cur + "\n")
		} else {
			b.WriteString(m.current + cur + "\n")
		}
	}
	if m.plan == nil && len(m.rows) > 0 {
		b.WriteString("\n  exercises\n" + sideTable(m.rows))
	}
	return b.String()
}

// cursor renders a fake blinking text-cursor: a solid block while "on", a
// space while "off", so the prompt reads as an editable field.
func cursor(on bool) string {
	if on {
		return "\u2588"
	}
	return " "
}

// runPrompt drives one in-window prompt and returns its result. It mirrors the
// cli prompt opt contract without touching the plain readPrompt/readLineSig.
func runPrompt(k promptKind, label, suggest string, total time.Duration, numeric bool, defY bool, rows ...[]SessionRow) promptResult {
	m := promptModel{kind: k, label: label, suggest: suggest, numeric: numeric, defY: defY, total: total, left: total}
	if len(rows) > 0 {
		m.rows = toSessionRows(rows[0])
	}
	return runPromptModel(m)
}

// toSessionRows converts the CLI's exported SessionRow list into the internal
// side-table rows every window can pin at the bottom.
func toSessionRows(rows []SessionRow) []sessionRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]sessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, sessionRow{seq: r.Seq, name: r.Name, topic: r.Topic, status: r.Status, bpm: r.BPM})
	}
	return out
}

func runPromptModel(m promptModel) promptResult {
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	done, err := p.Run()
	if err != nil {
		return promptResult{Aborted: true}
	}
	if mm, ok := done.(promptModel); ok {
		if mm.last != nil {
			return mm.last.result
		}
	}
	return promptResult{Aborted: true}
}

// RunBPM in-window BPM prompt: mirrors cli.promptBPM (suggest pre-fill, empty
// answer accepts the suggestion, up/down nudge, Ctrl-C aborts). Optional rows
// pin the exercise side-table at the bottom of the window.
func RunBPM(label string, suggest int, rows ...[]SessionRow) (int, bool) {
	r := runPrompt(promptBPM, label, itoa(suggest), 0, true, false, rows...)
	if r.Aborted {
		return suggest, true
	}
	n := atoiDefault(r.Value)
	if n == 0 && suggest > 0 {
		n = suggest
	}
	return n, r.Aborted
}

// RunY in-window yes/no: returns the normalized first letter (y/n) or empty on
// Ctrl-C, mirroring readLineSig's "not aborted and matches y/yes" semantics.
// defY true shows "Yes is the default (Y/n)", false shows "y/N".
func RunY(label string, rows ...[]SessionRow) (string, bool) {
	return runY(label, true, rows...)
}

// RunYN in-window yes/no defaulting to No (y/N).
func RunYN(label string, rows ...[]SessionRow) (string, bool) {
	return runY(label, false, rows...)
}

func runY(label string, defY bool, rows ...[]SessionRow) (string, bool) {
	r := runPrompt(promptY, label, "", 0, false, defY, rows...)
	return r.Value, r.Aborted
}

// RunCountdown in-window countdown: Enter to begin once, then the bar runs and
// Ctrl-C skips. Returns true when skipped (mirrors cli.countdown).
// RunCountdown returns (skipped, aborted). Esc/Delete skips the phase but the
// session continues; Ctrl-C aborts hard. Natural expiry is neither.
func RunCountdown(label string, total time.Duration, rows ...[]SessionRow) (skipped bool, aborted bool) {
	r := runPrompt(promptCountdown, label, "", total, false, false, rows...)
	return r.Skipped, r.Aborted
}

// RunNotes captures free-text notes in-window, seeded with the previous entry's
// text for quick reuse. Returns the accepted text and whether Ctrl-C aborted.
func RunNotes(label string, history *[]string, rows ...[]SessionRow) (string, bool) {
	seed := ""
	if history != nil && len(*history) > 0 {
		seed = (*history)[len(*history)-1]
	}
	r := runPrompt(promptText, label, seed, 0, false, false, rows...)
	return r.Value, r.Aborted
}

// RunPick shows a numbered list and asks for comma-separated indices in-window.
func RunPick(label string, items []string) (string, bool) {
	r := runPromptModel(promptModel{kind: promptPick, label: label, items: items})
	if r.Aborted {
		return "", true
	}
	return r.Value, false
}
