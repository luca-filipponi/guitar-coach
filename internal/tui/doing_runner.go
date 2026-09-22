package tui

import (
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// doingRunner wraps doingModel as a one-shot Bubble Tea program so the CLI can
// run the Doing phase as its own screen and get a plain bool back
// (true=start, false=Ctrl-C skipped / EOF / error). The wrapper owns only
// "when is this screen done" (a gateMsg ends the program); every key decision
// lives in doingModel.Update, whose contract is locked by the hermetic
// regression test. View is doingModel.View — a pure string with no writer, so
// nothing typed can ever echo into it.
type doingRunner struct {
	wrap doingModel
}

func (m doingRunner) Init() tea.Cmd { return m.wrap.Init() }

func (m doingRunner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.wrap.Update(msg)
	m.wrap = model.(doingModel)
	return m, tea.Batch(cmd, gateQuitCmd(m.wrap))
}

// gateQuitCmd ends the program as soon as the Doing phase gates (started or
// skipped). Without it the model would happily tick forever in a program.
func gateQuitCmd(w doingModel) tea.Cmd {
	if w.started || w.skipped {
		return tea.Quit
	}
	return nil
}

// View delegates: the doing model renders the countdown + ready-gate only.
func (m doingRunner) View() string { return m.wrap.View() }

// RunDoing runs the Doing phase as a one-shot Bubble Tea program against the
// caller's real terminal (r=osStdin, w=real stdout — the same fds awaitEnter
// used). It returns true when the phase starts (lone Enter or the Doing timer
// expiring on its own) and false when Ctrl-C skipped it, EOF arrived, or the
// program errored. No other key can end it: doingModel.Update ignores stray
// keys, and the wrapper only quits on a real gateMsg.
func RunDoing(name string, total time.Duration, startNote string, r io.Reader, w io.Writer) bool {
	m := doingRunner{wrap: doingModel{
		name:      name,
		total:     total,
		left:      total,
		startNote: startNote,
	}}
	p := tea.NewProgram(m, tea.WithInput(r), tea.WithOutput(w))
	done, err := p.Run()
	if err != nil {
		return false
	}
	rm, ok := done.(doingRunner)
	if !ok {
		return false
	}
	return rm.wrap.started
}
