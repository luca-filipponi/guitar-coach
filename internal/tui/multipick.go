package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pickCheckModel is a bare checklist: up/down moves a cursor, Space or Enter
// toggles a ✓ on the highlighted item, and the trailing "confirm" row finishes
// the prompt. Esc skips. It is deliberately separate from promptModel (which
// owns the countdown/BPM/Y/pick prompt machinery) so adding a second picker
// style cannot destabilise the existing prompts.
type pickCheckModel struct {
	label  string
	items  []string
	plan   *PlanInfo
	cursor int
	marks  []bool
	done   bool
	blink  bool
	last   *promptDoneMsg
}

// pickCheckDoneMsg is a pencil-local alias so the checklist can reuse the same
// promptDoneMsg wiring without importing another package's message.
type pickCheckDoneMsg struct{ *promptDoneMsg }

func (m pickCheckModel) Init() tea.Cmd { return tea.Batch(tickCmd(), blinkCmd()) }

func (m pickCheckModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case blinkMsg:
		if m.done {
			return m, nil
		}
		m.blink = !m.blink
		return m, blinkCmd()
	case tea.KeyMsg:
		if m.done {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			m.last = &promptDoneMsg{promptResult{Aborted: true}}
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.items) { // trailing confirm row
				m.cursor++
			}
			return m, nil
		case " ", "space", "enter":
			if m.cursor == len(m.items) { // confirm row
				m.done = true
				m.last = &promptDoneMsg{promptResult{Value: indicesOf(m.marks)}}
				return m, tea.Quit
			}
			m.marks[m.cursor] = !m.marks[m.cursor]
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
			return m, nil
		}
	}
	return m, nil
}

func (m pickCheckModel) View() string {
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(GuitarArt, "\n", "\n  ") + "\n")
	if m.plan != nil {
		b.WriteString(planBox(m.plan))
	}
	b.WriteString("  " + m.label + "\n")
	for i, it := range m.items {
		mark := "  "
		if m.marks[i] {
			mark = green("✓") + " "
		}
		cur := " "
		if m.cursor == i {
			cur = cursor(m.blink)
		}
		b.WriteString(fmt.Sprintf("  %s%2d. %s%s\n", mark, i+1, it, cur))
	}
	// trailing confirm row
	cur := " "
	if m.cursor == len(m.items) {
		cur = cursor(m.blink)
	}
	nChecked := 0
	for _, c := range m.marks {
		if c {
			nChecked++
		}
	}
	b.WriteString(fmt.Sprintf("  %s%d checked%s  [confirm rotation]\n", green("✓"), nChecked, cur))
	b.WriteString("  up/down to move, space/enter to toggle, enter on confirm to accept, Esc to skip\n")
	return b.String()
}

// indicesOf renders the checked rows as the comma-separated 1-based list the
// rest of the code expects from a picker (same shape RunPick / --pick parse).
func indicesOf(marks []bool) string {
	var parts []string
	for i, c := range marks {
		if c {
			parts = append(parts, itoa(i+1))
		}
	}
	return strings.Join(parts, ",")
}

// RunPickCheck runs the ✓-checklist. It returns the comma-separated 1-based
// indices of the checked rows, plus whether the prompt was aborted.
func RunPickCheck(label string, items []string, plan *PlanInfo) (string, bool) {
	if len(items) == 0 {
		return "", true
	}
	m := pickCheckModel{
		label: label,
		items: items,
		plan:  plan,
		marks: make([]bool, len(items)),
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	done, err := p.Run()
	if err != nil {
		return "", true
	}
	if mm, ok := done.(pickCheckModel); ok && mm.last != nil {
		return mm.last.result.Value, mm.last.result.Aborted
	}
	return "", true
}

var _ = os.Stdout
