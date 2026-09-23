package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// simulate pushes a message through the driver model, runs the returned
// command if any, and returns the resulting message.
func simulate(t *testing.T, m *startFlowModel, msg tea.Msg) tea.Msg {
	t.Helper()
	nm, cmd := m.Update(msg)
	*m = nm.(startFlowModel)
	if cmd == nil {
		return nil
	}
	return cmd()
}

func newTestFlowModel() startFlowModel {
	return startFlowModel{resCh: make(chan PhaseResult, 8)}
}

func TestFlowCountdownSkip(t *testing.T) {
	m := newTestFlowModel()
	simulate(t, &m, startPhaseMsg{req: PhaseRequest{Kind: PhaseCountdown, Label: "rest", Total: Tick}})
	got := simulate(t, &m, tea.KeyMsg{Type: tea.KeyEnter})
	if got != nil {
		t.Fatalf("enter before first tick must not finish: got %#v", got)
	}
	got = simulate(t, &m, tea.KeyMsg{Type: tea.KeyEsc})
	res, ok := got.(flowResultMsg)
	if !ok {
		t.Fatalf("expected flowResultMsg, got %#v", got)
	}
	if !res.result.Skipped {
		t.Fatalf("expected Skipped=true, got %#v", res.result)
	}
}

func TestFlowCountdownExpiry(t *testing.T) {
	m := newTestFlowModel()
	simulate(t, &m, startPhaseMsg{req: PhaseRequest{Kind: PhaseCountdown, Label: "rest", Total: Tick}})
	simulate(t, &m, tea.KeyMsg{Type: tea.KeyEnter})
	got := simulate(t, &m, tickMsg{})
	if _, ok := got.(flowResultMsg); !ok {
		t.Fatalf("expected flowResultMsg after expiry, got %#v", got)
	}
	res := got.(flowResultMsg)
	if res.result.Skipped || res.result.Aborted {
		t.Fatalf("natural expiry is neither skip nor abort, got %#v", res.result)
	}
}

func TestFlowCountdownAbort(t *testing.T) {
	m := newTestFlowModel()
	simulate(t, &m, startPhaseMsg{req: PhaseRequest{Kind: PhaseCountdown, Label: "rest"}})
	got := simulate(t, &m, tea.KeyMsg{Type: tea.KeyCtrlC})
	res, ok := got.(flowResultMsg)
	if !ok {
		t.Fatalf("expected flowResultMsg, got %#v", got)
	}
	if !res.result.Aborted {
		t.Fatalf("expected Aborted=true, got %#v", res.result)
	}
}

func TestFlowSessionAbort(t *testing.T) {
	// A session phase aborted with Ctrl-C must surface Aborted through the
	// driver, and the driver must still be alive for the next phase.
	m := newTestFlowModel()
	simulate(t, &m, startPhaseMsg{req: PhaseRequest{Kind: PhaseSession, Label: "chords", Total: Tick, Suggest: 60}})
	if sm, ok := m.child.(sessionModel); !ok || sm.label != "chords" || sm.suggest != 60 {
		t.Fatalf("session child not seeded: %#v", m.child)
	}
	res := simulate(t, &m, tea.KeyMsg{Type: tea.KeyCtrlC}).(flowResultMsg)
	if !res.result.Aborted {
		t.Fatalf("expected Aborted=true, got %#v", res.result)
	}
}

func TestFlowLogFeed(t *testing.T) {
	m := newTestFlowModel()
	simulate(t, &m, flowLogMsg{lines: []string{"hello"}})
	simulate(t, &m, flowLogMsg{lines: []string{"world"}})
	if len(m.logs) != 2 || m.logs[0] != "hello" || m.logs[1] != "world" {
		t.Fatalf("feed mismatch: %#v", m.logs)
	}
	view := m.View()
	for _, want := range []string{"hello", "world"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view should contain %q, got:\n%s", want, view)
		}
	}
}

func TestFlowQuitPassthrough(t *testing.T) {
	m := newTestFlowModel()
	got := simulate(t, &m, tea.QuitMsg{})
	if _, ok := got.(tea.QuitMsg); !ok {
		t.Fatalf("expected outer tea.QuitMsg, got %#v", got)
	}
}

func TestBuildChildKinds(t *testing.T) {
	if c := buildChild(PhaseRequest{Kind: PhaseSession, Total: time.Second}); c.(sessionModel).total != time.Second {
		t.Fatalf("session child total wrong")
	}
	if c := buildChild(PhaseRequest{Kind: PhaseY}); c.(promptModel).defY != true {
		t.Fatalf("PhaseY should default to yes")
	}
	if c := buildChild(PhaseRequest{Kind: PhaseYN}); c.(promptModel).defY != false {
		t.Fatalf("PhaseYN should default to no")
	}
	if c := buildChild(PhaseRequest{Kind: PhaseCountdown}); c.(promptModel).kind != promptCountdown {
		t.Fatalf("countdown child wrong")
	}
}
