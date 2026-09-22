package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func sessionRows(n int) []sessionRow {
	rows := make([]sessionRow, n)
	for i := range rows {
		rows[i] = sessionRow{seq: i + 1, name: "exercise " + itoa(i+1), topic: "t", status: "pending"}
	}
	return rows
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	rn := []rune(s)
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: rn}
}

// startDoing opens a window and advances past the start-BPM field, returning
// the model in the Doing phase. startBPM empty keeps the suggestion.
func startDoing(t *testing.T, m sessionModel) sessionModel {
	if m.phase != phaseStartBPM {
		t.Fatalf("window must open on the start-BPM phase, got %v", m.phase)
	}
	m.phase = phaseDoing
	m.left = m.total
	m.startedAt = time.Now().UTC()
	return m
}

func TestDoingWindowStrayKeys(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", time.Minute, 120, "", nil))
	for _, k := range []string{"a", "b", "c"} {
		next, _ := m.Update(key(k))
		m = next.(sessionModel)
	}
	if m.aborted || m.phase != phaseDoing {
		t.Fatalf("stray keys must be no-ops in doing: aborted=%v phase=%v", m.aborted, m.phase)
	}
}

func TestDoingNoOpKeys(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", time.Minute, 120, "", nil))
	for _, k := range []string{"enter", "space", "tab", "\t", "1", "2", "0"} {
		next, _ := m.Update(key(k))
		m = next.(sessionModel)
	}
	if m.aborted || m.phase != phaseDoing {
		t.Fatalf("enter/space/tab/digits must not quit or skip: aborted=%v phase=%v", m.aborted, m.phase)
	}
}

func TestEscFinishesEarlyRecords(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", time.Minute, 120, "", nil))
	next, _ := m.Update(key("esc"))
	m = next.(sessionModel)
	if m.aborted {
		t.Fatalf("esc must finish early, not abort: aborted=%v", m.aborted)
	}
	if m.phase != phaseEndBPM {
		t.Fatalf("esc in doing must move to end-BPM phase, got %v", m.phase)
	}
	if strings.Contains(m.View(), "\x03") {
		t.Fatal("screen must not echo a literal ^C")
	}
}

func TestCtrlCAborts(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", time.Minute, 120, "", nil))
	next, _ := m.Update(key("ctrl+c"))
	m = next.(sessionModel)
	if !m.aborted {
		t.Fatalf("ctrl-c must abort: aborted=%v", m.aborted)
	}
}

func TestTickSelfAdvancesDuringDoing(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", time.Minute, 120, "", nil))
	next, _ := m.Update(tickMsg{})
	m2 := next.(sessionModel)
	if m2.left >= m.left {
		t.Fatalf("tick must decrease left: before=%v after=%v", m.left, m2.left)
	}
	if next == nil {
		t.Fatal("pre-expiry tick must re-arm the countdown, got nil cmd")
	}
}

func TestStartBPMFlow(t *testing.T) {
	m := newSessionModel(sessionRows(3), "exercise 1", time.Minute, 120, "seed note", nil)
	if m.startBPM != 0 {
		t.Fatalf("window must open with start BPM uncommitted, got %d", m.startBPM)
	}
	// Type an explicit start BPM.
	next, _ := m.Update(key("9"))
	m = next.(sessionModel)
	next, _ = m.Update(key("9"))
	m = next.(sessionModel)
	// Enter begins doing with the typed value.
	next, _ = m.Update(key("enter"))
	m = next.(sessionModel)
	if m.phase != phaseDoing || m.startBPM != 99 {
		t.Fatalf("expected doing with start BPM 99, got phase=%v startBPM=%d", m.phase, m.startBPM)
	}
}

func TestEscOnStartBPMKeepsSuggestion(t *testing.T) {
	m := newSessionModel(sessionRows(3), "exercise 1", time.Minute, 120, "", nil)
	next, _ := m.Update(key("esc"))
	m = next.(sessionModel)
	if m.phase != phaseDoing || m.startBPM != 120 {
		t.Fatalf("esc on start BPM must accept suggestion and begin doing: phase=%v startBPM=%d", m.phase, m.startBPM)
	}
}

func TestEndBPMAndNotesFlow(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", time.Minute, 120, "seed note", nil))
	// Finish the set early.
	next, _ := m.Update(key("esc"))
	m = next.(sessionModel)
	if m.phase != phaseEndBPM {
		t.Fatalf("expected end-BPM phase, got %v", m.phase)
	}
	// Type an end BPM below the start suggestion.
	next, _ = m.Update(key("1"))
	m = next.(sessionModel)
	next, _ = m.Update(key("1"))
	m = next.(sessionModel)
	next, _ = m.Update(key("0"))
	m = next.(sessionModel)
	// Enter accepts the end BPM and moves to notes.
	next, _ = m.Update(key("enter"))
	m = next.(sessionModel)
	if m.phase != phaseNotes {
		t.Fatalf("expected notes phase, got %v", m.phase)
	}
	if m.endBPM != 110 {
		t.Fatalf("end BPM must be the typed 110, got %d", m.endBPM)
	}
	// Seed is pre-filled.
	if !strings.Contains(m.View(), "seed note") {
		t.Fatal("notes field must be pre-seeded from history")
	}
	// Enter on notes closes the window; the exercise is recorded.
	next, _ = m.Update(key("enter"))
	m = next.(sessionModel)
	if m.aborted {
		t.Fatal("completing notes must record, not abort")
	}
}

func TestTimerExpiryMovesToEndBPM(t *testing.T) {
	m := startDoing(t, newSessionModel(sessionRows(3), "", Tick, 120, "", nil))
	next, _ := m.Update(tickMsg{})
	m = next.(sessionModel)
	if m.phase != phaseEndBPM {
		t.Fatalf("timer expiry must move to end-BPM phase, got %v", m.phase)
	}
	if m.left != 0 {
		t.Fatalf("left must clamp to 0, got %v", m.left)
	}
}