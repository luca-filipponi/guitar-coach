package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptContract(t *testing.T) {
	subs := []struct {
		name   string
		maker  func() promptModel
		keys   []string
		verify func(t *testing.T, m promptModel)
	}{
		{
			"BPM: stray letters ignored, digits kept, backspace works", func() promptModel {
				return promptModel{kind: promptBPM, label: "end BPM", suggest: "62", numeric: true}
			},
			[]string{"a", "b", "1", "2", "backspace", "0"}, func(t *testing.T, m promptModel) {
				if m.current != "10" {
					t.Fatalf("stray letters must drop, backspace removes: wanted 10, got %q", m.current)
				}
			},
		},
		{
			"BPM: up/down nudges the suggestion", func() promptModel {
				return promptModel{kind: promptBPM, label: "start BPM", suggest: "62", numeric: true}
			},
			[]string{"up"}, func(t *testing.T, m promptModel) {
				if m.current != "63" {
					t.Fatalf("up from 62 wanted 63, got %q", m.current)
				}
			},
		},
		{
			"notes: space and punctuation are kept", func() promptModel {
				return promptModel{kind: promptText, label: "notes"}
			},
			[]string{"n", "i", "c", "e", " ", "i", "t", "!"}, func(t *testing.T, m promptModel) {
				if m.current != "nice it!" {
					t.Fatalf("space must be kept in notes: got %q", m.current)
				}
			},
		},
		{
			"countdown: first Enter readies, skip is Ctrl-C only", func() promptModel {
				return promptModel{kind: promptCountdown, label: "Warmup", total: time.Minute, left: time.Minute}
			},
			[]string{"enter", "enter"}, func(t *testing.T, m promptModel) {
				if !m.initStep || m.done {
					t.Fatalf("first enter readies; second enter must NOT complete: initStep=%v done=%v", m.initStep, m.done)
				}
			},
		},
		{
			"countdown: tick does not burn before Enter", func() promptModel {
				return promptModel{kind: promptCountdown, label: "Warmup", total: time.Minute, left: time.Minute}
			},
			[]string{}, func(t *testing.T, m promptModel) {
				n, cmd := m.Update(tickMsg{})
				nm := n.(promptModel)
				if nm.left != m.left {
					t.Fatalf("pre-Enter tick must not decrement: before=%v after=%v", m.left, nm.left)
				}
				if cmd == nil {
					t.Fatal("pre-Enter tick must re-arm the timer so the countdown actually runs after Enter")
				}
			},
		},
		{
			"countdown: tick self-advances after ready", func() promptModel {
				return promptModel{kind: promptCountdown, label: "Warmup", total: time.Minute, left: time.Minute, initStep: true}
			},
			[]string{}, func(t *testing.T, m promptModel) {
				n, _ := m.Update(tickMsg{})
				nm := n.(promptModel)
				if nm.left >= m.left {
					t.Fatalf("tick must lower remaining: before=%v after=%v", m.left, nm.left)
				}
			},
		},
		{
			"countdown: Esc skips, natural end is a clean finish", func() promptModel {
				return promptModel{kind: promptCountdown, label: "Break", total: 400 * time.Millisecond, left: 400 * time.Millisecond, initStep: true}
			},
			[]string{"esc"}, func(t *testing.T, m promptModel) {
				if !m.done || m.last == nil || !m.last.result.Skipped || m.last.result.Aborted {
					t.Fatalf("esc must skip+continue: done=%v last=%+v", m.done, m.last)
				}
			},
		},
		{
			"countdown: Ctrl-C aborts hard", func() promptModel {
				return promptModel{kind: promptCountdown, label: "Break", total: 400 * time.Millisecond, left: 400 * time.Millisecond, initStep: true}
			},
			[]string{"ctrl+c"}, func(t *testing.T, m promptModel) {
				if !m.done || m.last == nil || !m.last.result.Aborted || m.last.result.Skipped {
					t.Fatalf("ctrl-c must abort hard: done=%v last=%+v", m.done, m.last)
				}
			},
		},
		{
			"prompts pin the exercise side-table at the bottom", func() promptModel {
				return promptModel{kind: promptY, label: "keep rotation?", rows: sessionRows(3), defY: true}
			},
			[]string{}, func(t *testing.T, m promptModel) {
				v := m.View()
				if strings.Index(v, "exercises") < strings.Index(v, "Enter = y") {
					t.Fatalf("exercise table must render below the prompt: %q", v)
				}
				if !strings.Contains(v, "exercise 3") {
					t.Fatalf("side-table rows missing: %q", v)
				}
			},
		},
		{
			"Y: 'y' answers immediately without Enter", func() promptModel {
				return promptModel{kind: promptY, label: "keep rotation?", defY: true}
			},
			[]string{"y"}, func(t *testing.T, m promptModel) {
				if !m.done || m.last == nil || m.last.result.Value != "y" {
					t.Fatalf("typing y must answer at once: done=%v last=%+v", m.done, m.last)
				}
			},
		},
		{
			"Y: 'n' answers immediately and overrides the default", func() promptModel {
				return promptModel{kind: promptY, label: "keep rotation?", defY: true}
			},
			[]string{"n"}, func(t *testing.T, m promptModel) {
				if !m.done || m.last == nil || m.last.result.Value != "n" {
					t.Fatalf("typing n must answer at once: done=%v last=%+v", m.done, m.last)
				}
			},
		},
		{
			"Y: 'n' wins in a run of letters", func() promptModel {
				return promptModel{kind: promptY, label: "keep rotation?", defY: true}
			},
			[]string{"n", "o"}, func(t *testing.T, m promptModel) {
				if !m.done || m.last == nil || m.last.result.Value != "n" {
					t.Fatalf("first letter n must answer at once: done=%v last=%+v", m.done, m.last)
				}
			},
		},
	}
	for _, sub := range subs {
		t.Run(sub.name, func(t *testing.T) {
			m := sub.maker()
			for _, k := range sub.keys {
				next, _ := m.Update(key(k))
				m = next.(promptModel)
			}
			sub.verify(t, m)
		})
	}
}

// helper: ignored in the tui package test build — ensures KeyMsg paths compile.
var _ = tea.KeyMsg{}
