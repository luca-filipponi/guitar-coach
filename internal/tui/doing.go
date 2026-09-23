package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// tickMsg marks one countdown tick (every Tick cadence).
type tickMsg struct{}

// gateMsg is the terminal message the parent program receives when the Doing
// phase ends. started=true means "begin actively" (a lone Enter or the timer
// running out on its own); skipped=true means Ctrl-C skipped it.
type gateMsg struct {
	started bool
	skipped bool
}

// Tick is the redraw cadence of the Doing countdown.
const Tick = 100 * time.Millisecond

func tickCmd() tea.Cmd {
	return tea.Tick(Tick, func(time.Time) tea.Msg { return tickMsg{} })
}

// startPhase Cmd tells the parent "do it now"; skipPhase Cmd tells it "skip".
func (m doingModel) startPhase() tea.Msg { return gateMsg{started: true} }
func (m doingModel) skipPhase() tea.Msg  { return gateMsg{skipped: true} }

// doingModel is the Doing/ready-gate slice of the Bubble Tea TUI. The model
// responds only to a lone Enter (start), Ctrl-C (skip) and its own ticks. A
// stray key is delivered as a KeyMsg the model simply ignores — it can never
// gate, auto-advance or echo, because the model's Update inspects keys it
// cares about and its View has no writer at all.
type doingModel struct {
	name      string
	total     time.Duration
	left      time.Duration
	startNote string // e.g. "start 120 bpm"
	started   bool   // lone Enter (or expiry) requested start
	skipped   bool   // Ctrl-C requested skip
}

func (m doingModel) Init() tea.Cmd { return tickCmd() }

// Update routes every incoming message through the Doing contract:
//
//   - stray keys: ignored (returned unchanged — no echo, no auto-gate);
//   - a lone Enter: started=true, phase ends;
//   - Ctrl-C: skipped=true, phase ends (never rendered as a literal ^C);
//   - a tick: countdown ticks; when it expires the phase ends on its own —
//     a tick finishes it, never a stray Enter.
func (m doingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.left -= Tick
		if m.left <= 0 {
			m.left = 0
			m.started = true
			return m, func() tea.Msg { return m.startPhase() }
		}
		return m, tickCmd()
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "\r", "\n":
			m.started = true
			return m, func() tea.Msg { return m.startPhase() }
		case "ctrl+c", "\x03":
			m.skipped = true
			return m, func() tea.Msg { return m.skipPhase() }
		}
		return m, nil
	}
	return m, nil
}

// View renders the live countdown and ready-gate as pure ANSI cells. There is
// no writer, so nothing typed can ever be echoed into this string.
func (m doingModel) View() string {
	leftSec := int(m.left / time.Second)
	bar := progressBar(m.total-m.left, m.total)
	prompt := "press Enter to begin, Ctrl-C to skip"
	if m.started {
		prompt = "starting…"
	} else if m.skipped {
		prompt = "skipped"
	}
	return fmt.Sprintf("\n  %s  %s\n  %s  %02d s\n  %s\n",
		m.name, m.startNote, bar, leftSec, prompt)
}

// progressBar renders a 16-cell fill proportional to elapsed time.
func progressBar(elapsed, total time.Duration) string {
	const w = 16
	if total <= 0 {
		total = 1
	}
	full := int(elapsed * w / total)
	if full > w {
		full = w
	}
	buf := make([]byte, w+2)
	buf[0] = '['
	for i := 1; i <= w; i++ {
		if i <= full {
			buf[i] = '#'
		} else {
			buf[i] = '.'
		}
	}
	buf[w+1] = ']'
	return string(buf)
}
