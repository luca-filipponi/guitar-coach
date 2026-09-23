package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// PhaseKind selects which sub-window the persistent session program runs next.
type PhaseKind int

const (
	// PhaseCountdown is a warmup/rest/break countdown (Enter starts it, Esc
	// skips, Ctrl-C aborts the session).
	PhaseCountdown PhaseKind = iota
	// PhaseSession is the merged per-exercise window: start BPM -> Doing ->
	// end BPM -> notes, all inside one window.
	PhaseSession
	// PhaseY is a yes/no prompt.
	PhaseY
	// PhaseYN is a yes/no prompt defaulting to no.
	PhaseYN
)

// PlanInfo is the persistent session-plan box pinned under the guitar across
// every window of a session: the round's exercise order with a pointer on the
// current exercise and done markers, plus a footer line.
type PlanInfo struct {
	Round  int
	Rows   []SessionRow
	Footer string
}

// PhaseRequest asks the persistent program to run one phase.
type PhaseRequest struct {
	Kind      PhaseKind
	Label     string
	Total     time.Duration
	Rows      []SessionRow // exercise side table, pinned under the window
	Plan      *PlanInfo    // always-on-top session plan box (nil for standalone)
	Suggest   int          // suggested start BPM for PhaseSession
	NotesSeed string       // previous note text for PhaseSession
	Alarm     func()       // fired the moment a Doing countdown ends
}

// PhaseResult is what the persistent program reports when a phase finished.
// Skipped/Aborted mirror the CLI gate contract (Esc/Ctrl-C); Value carries a
// Y/n answer; StartBPM/EndBPM/Notes/StartedAt/FinishedAt come from PhaseSession.
type PhaseResult struct {
	Skipped    bool
	Aborted    bool
	Value      string
	StartBPM   int
	EndBPM     int
	Notes      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// Flow drives one persistent alt-screen program for the whole session. The
// player enters the alt screen once and phases (countdowns, the exercise
// window, prompts) are swapped in place by the driver model, so the terminal
// never restores the primary buffer mid-session: no flicker between windows.
type Flow struct {
	p     *tea.Program
	resCh chan PhaseResult
	done  chan struct{}
	err   error
}

// NewFlow creates the persistent program without running it yet.
func NewFlow() *Flow {
	f := &Flow{
		resCh: make(chan PhaseResult, 4),
		done:  make(chan struct{}),
	}
	f.p = tea.NewProgram(startFlowModel{resCh: f.resCh}, tea.WithAltScreen(), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	return f
}

// Run starts the program on its own goroutine.
func (f *Flow) Run() {
	go func() {
		_, f.err = f.p.Run()
		close(f.done)
	}()
}

// Next starts a phase and blocks until it finishes.
func (f *Flow) Next(req PhaseRequest) PhaseResult {
	f.p.Send(startPhaseMsg{req})
	return <-f.resCh
}

// Log renders a formatted line into the in-window session feed, so nothing
// scrolls the normal terminal. An empty format with no args is ignored.
func (f *Flow) Log(format string, args ...any) {
	if format == "" && len(args) == 0 {
		return
	}
	f.p.Send(flowLogMsg{lines: []string{fmt.Sprintf(format, args...)}})
}

// Close quits the persistent program and waits for it to exit.
func (f *Flow) Close() {
	f.p.Quit()
	<-f.done
}

// startFlowModel hosts one child sub-window at a time plus the session-text
// feed. It forwards every message to the child and intercepts the child's
// tea.Quit, converting it into a flowResultMsg so the program survives the gap
// between phases.
type startFlowModel struct {
	resCh chan<- PhaseResult
	child tea.Model
	logs  []string
}

const maxLogLines = 24

func (m startFlowModel) Init() tea.Cmd { return nil }

func (m startFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.QuitMsg:
		return m, tea.Quit
	case startPhaseMsg:
		m.child = buildChild(msg.req)
		return m, m.child.Init()
	case flowLogMsg:
		m.logs = append(m.logs, msg.lines...)
		if len(m.logs) > maxLogLines {
			m.logs = m.logs[len(m.logs)-maxLogLines:]
		}
		return m, nil
	case flowResultMsg:
		m.resCh <- msg.result
		return m, nil
	default:
		if m.child == nil {
			return m, nil
		}
		cm, cmd := m.child.Update(msg)
		doneState := cm
		m.child = cm
		if cmd != nil {
			return m, func() tea.Msg {
				got := cmd()
				if _, isQuit := got.(tea.QuitMsg); isQuit {
					return flowResultMsg{result: extractResult(doneState)}
				}
				return got
			}
		}
		return m, nil
	}
}

func (m startFlowModel) View() string {
	var b strings.Builder
	if m.child != nil {
		b.WriteString(m.child.View())
	}
	if len(m.logs) > 0 {
		b.WriteString("\n")
		b.WriteString(strings.Join(m.logs, "\n"))
	}
	return b.String()
}

// buildChild turns a phase request into the sub-window model.
func buildChild(req PhaseRequest) tea.Model {
	switch req.Kind {
	case PhaseSession:
		m := newSessionModel(nil, req.Label, req.Total, req.Suggest, req.NotesSeed, req.Alarm)
		m.plan = req.Plan
		if len(req.Rows) > 0 {
			m.rows = toSessionRows(req.Rows)
		}
		return m
	case PhaseY:
		pm := promptModel{kind: promptY, label: req.Label, defY: true, plan: req.Plan}
		if len(req.Rows) > 0 {
			pm.rows = toSessionRows(req.Rows)
		}
		return pm
	case PhaseYN:
		pm := promptModel{kind: promptY, label: req.Label, defY: false, plan: req.Plan}
		if len(req.Rows) > 0 {
			pm.rows = toSessionRows(req.Rows)
		}
		return pm
	default: // PhaseCountdown
		pm := promptModel{kind: promptCountdown, label: req.Label, total: req.Total, left: req.Total, plan: req.Plan}
		if len(req.Rows) > 0 {
			pm.rows = toSessionRows(req.Rows)
		}
		return pm
	}
}

// extractResult reads the final state of a finished child into a PhaseResult.
func extractResult(child tea.Model) PhaseResult {
	switch c := child.(type) {
	case sessionModel:
		return PhaseResult{
			Aborted:    c.aborted,
			StartBPM:   c.startBPM,
			EndBPM:     c.endBPM,
			Notes:      c.notes,
			StartedAt:  c.startedAt,
			FinishedAt: c.endedAt,
		}
	case promptModel:
		var pr PhaseResult
		if c.last != nil {
			pr.Skipped = c.last.result.Skipped
			pr.Aborted = c.last.result.Aborted
			pr.Value = c.last.result.Value
		}
		return pr
	}
	return PhaseResult{Aborted: true}
}

// startPhaseMsg swaps the child to a new phase.
type startPhaseMsg struct {
	req PhaseRequest
}

// flowLogMsg appends session text to the in-window feed.
type flowLogMsg struct {
	lines []string
}

// flowResultMsg is emitted by the driver when a child finishes; the model then
// forwards it to the CLI through the result channel.
type flowResultMsg struct {
	result PhaseResult
}
