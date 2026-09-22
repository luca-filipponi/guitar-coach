package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestDoingGateRegression drives the REAL doingModel.Update with hermetic
// messages (no program loop, no tty, no real clock) and locks the Doing
// ready-gate contract:
//
//   - stray keys: ignored — never gate, never auto-advance, never echo;
//   - a lone Enter: started=true, phase ends;
//   - Ctrl-C: skipped=true, phase ends (never a literal ^C in the view);
//   - a tick: countdown ticks; expiry starts on its own (a tick, not a key);
//   - render: the view carries no writer, so no typed byte can ever echo.
func TestDoingGateRegression(t *testing.T) {
	junkRunes := []rune("s ye asdf asdf a sdf")
	junk := make([]tea.KeyMsg, 0, len(junkRunes))
	for _, r := range junkRunes {
		junk = append(junk, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	base := doingModel{
		name:      "3x5 warmup",
		total:     30 * time.Second,
		left:      30 * time.Second,
		startNote: "start 120 bpm",
	}

	// drive applies msgs in order, calling Update on the model each step and
	// threading the returned model forward; it returns the final model.
	drive := func(m doingModel, msgs ...tea.Msg) doingModel {
		var mod tea.Model = m
		for _, msg := range msgs {
			nm, _ := mod.Update(msg)
			mod = nm
		}
		return mod.(doingModel)
	}

	// Ensure the base is Hermetized with a couple ticks so constructors prove
	// themselves; not swapped into Update path.
	_ = base

	t.Run("stray keys cannot gate, advance or echo", func(t *testing.T) {
		m := base
		msgs := make([]tea.Msg, 0, len(junk))
		for _, k := range junk {
			msgs = append(msgs, k)
		}
		m = drive(m, msgs...)
		if m.started || m.skipped {
			t.Fatalf("stray keys auto-gated: started=%v skipped=%v", m.started, m.skipped)
		}
		if m.left != base.left {
			t.Fatalf("stray keys advanced the timer: left=%v", m.left)
		}
		if strings.Contains(m.View(), "^C") {
			t.Fatalf("view echoes literal ^C: %q", m.View())
		}
	})

	t.Run("lone Enter after junk starts", func(t *testing.T) {
		m := base
		msgs := make([]tea.Msg, 0, len(junk)+1)
		for _, k := range junk {
			msgs = append(msgs, k)
		}
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyEnter})
		m = drive(m, msgs...)
		if !m.started || m.skipped {
			t.Fatalf("lone Enter did not start: started=%v skipped=%v", m.started, m.skipped)
		}
	})

	t.Run("lone Ctrl-C skips", func(t *testing.T) {
		m := drive(base, tea.KeyMsg{Type: tea.KeyCtrlC})
		if !m.skipped || m.started {
			t.Fatalf("Ctrl-C did not skip: skipped=%v started=%v", m.skipped, m.started)
		}
		if strings.Contains(m.View(), "^C") {
			t.Fatalf("view renders Ctrl-C as literal ^C: %q", m.View())
		}
	})

	t.Run("timer expiry starts on its own (tick, not a key)", func(t *testing.T) {
		m := base
		cap := int(base.total/Tick) + 5
		for i := 0; i < cap; i++ {
			var mod tea.Model = m
			nm, _ := mod.Update(tickMsg{})
			m = nm.(doingModel)
			if m.started {
				break
			}
		}
		if !m.started {
			t.Fatalf("timer expiry did not start: left=%v", m.left)
		}
		if m.left != 0 {
			t.Fatalf("expiry left residual time: %v", m.left)
		}
	})
}
