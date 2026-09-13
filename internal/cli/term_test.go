package cli

import (
	"strings"
	"testing"
)

func TestInsertText(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"a", "a"},
		{"100", "100"},
	}
	for _, c := range cases {
		var buf []rune
		pos := 0
		for i := 0; i < len(c.in); i++ {
			buf, pos = insertText(buf, pos, c.in[i])
		}
		if got := string(buf); got != c.want {
			t.Errorf("insertText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDeleteWordBackward(t *testing.T) {
	buf := []rune("80 was too fast")
	got := deleteWordBackward(buf, len(buf))
	if string(buf[:got]) != "80 was too " {
		t.Errorf("deleteWordBackward = %q, want %q", string(buf[:got]), "80 was too ")
	}
	got = deleteWordBackward(buf, got)
	if string(buf[:got]) != "80 was " {
		t.Errorf("deleteWordBackward(2) = %q, want %q", string(buf[:got]), "80 was ")
	}
}

func TestParseBufInt(t *testing.T) {
	for _, good := range []string{"0", "65", "185", " 42 "} {
		if n, ok := parseBufInt([]rune(good)); !ok || n < 0 {
			t.Errorf("parseBufInt(%q) = %d, ok=%v", good, n, ok)
		}
	}
	for _, bad := range []string{"", "abc", "65bpm"} {
		if _, ok := parseBufInt([]rune(bad)); ok {
			t.Errorf("parseBufInt(%q) unexpectedly ok", bad)
		}
	}
}

func TestNudgeNumeric(t *testing.T) {
	buf := []rune("65")
	pos := len(buf)
	opts := promptOpts{numeric: true}
	hist := 0
	nudgeNumeric(opts, &buf, &pos, 1, &hist, nil, nil)
	if got := string(buf); got != "66" {
		t.Errorf("after +1 = %q, want %q", got, "66")
	}
	nudgeNumeric(opts, &buf, &pos, -1, &hist, nil, nil)
	nudgeNumeric(opts, &buf, &pos, -1, &hist, nil, nil)
	if got := string(buf); got != "64" {
		t.Errorf("after -2 from 66 = %q, want %q", got, "64")
	}
}

func TestHandleEscapeIgnoresUnknownSequences(t *testing.T) {
	buf := []rune("abc")
	pos := 2
	var seq []byte
	hist := 0
	var draft []rune

	// ESC [ 1 ; 2 A (shift+arrow) must be fully consumed, buffer unchanged.
	st, dirty := handleEscape(1, '[', &seq, promptOpts{}, &buf, &pos, &draft, new(bool), &hist)
	if st != 2 {
		t.Fatalf("expected state 2, got %d", st)
	}
	for _, b := range []byte{'1', ';', '2'} {
		st, dirty = handleEscape(st, b, &seq, promptOpts{}, &buf, &pos, &draft, new(bool), &hist)
		if st != 2 || dirty {
			t.Fatalf("mid-sequence state = %d dirty=%v", st, dirty)
		}
	}
	st, dirty = handleEscape(st, 'A', &seq, promptOpts{}, &buf, &pos, &draft, new(bool), &hist)
	if st != 0 {
		t.Fatalf("final state = %d", st)
	}
	_ = dirty
	if string(buf) != "abc" {
		t.Fatalf("unknown sequence leaked into buffer: %q", string(buf))
	}
}

func TestHandleEscapeNumberArrows(t *testing.T) {
	buf := []rune("65")
	pos := 2
	var seq []byte
	hist := 0
	st, _ := handleEscape(1, '[', &seq, promptOpts{numeric: true}, &buf, &pos, nil, nil, &hist)
	st, _ = handleEscape(st, 'A', &seq, promptOpts{numeric: true}, &buf, &pos, nil, nil, &hist)
	if string(buf) != "66" {
		t.Fatalf("up arrow did not nudge: %q", string(buf))
	}
	if st != 0 {
		t.Fatalf("state not reset: %d", st)
	}
}

func TestDeleteKey(t *testing.T) {
	buf := []rune("ab cd")
	pos := 2 // cursor between "ab" and " cd"
	var seq []byte
	hist := 0
	st, dirty := handleEscape(1, '[', &seq, promptOpts{}, &buf, &pos, nil, nil, &hist)
	if st != 2 {
		t.Fatalf("expected state 2, got %d", st)
	}
	st, _ = handleEscape(st, '3', &seq, promptOpts{}, &buf, &pos, nil, nil, &hist)
	st, dirty = handleEscape(st, '~', &seq, promptOpts{}, &buf, &pos, nil, nil, &hist)
	if string(buf) != "abcd" {
		t.Fatalf("Delete key: got %q, want %q", string(buf), "abcd")
	}
	if st != 0 || !dirty {
		t.Fatalf("final state=%d dirty=%v", st, dirty)
	}
}

func TestRetainNotesDedup(t *testing.T) {
	sh := &Shell{}
	sh.retainNotes("felt good")
	sh.retainNotes("felt good")
	sh.retainNotes("sloppy")
	if len(sh.noteHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d: %v", len(sh.noteHistory), sh.noteHistory)
	}
	if strings.Join(sh.noteHistory, "|") != "felt good|sloppy" {
		t.Fatalf("unexpected history: %v", sh.noteHistory)
	}
}
