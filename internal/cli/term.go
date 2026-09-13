package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// makeRaw switches the terminal into raw mode (no echo, no line buffering,
// terminal-generated signals disabled) and returns a function that restores
// it. It fails with an error when stdin is not a TTY.
func makeRaw() (func(), error) {
	fd := int(os.Stdin.Fd())
	orig, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return nil, err
	}
	raw := *orig
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Oflag &^= unix.OPOST
	raw.Iflag &^= unix.ICRNL
	raw.Cc[unix.VMIN] = 0
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw); err != nil {
		return nil, err
	}
	return func() {
		_ = unix.IoctlSetTermios(fd, unix.TIOCSETA, orig)
	}, nil
}

// promptOpts configures a prompt: an optional live status line (shown above
// the prompt, e.g. the ticking rest clock), an initial prefilled value, an
// up/down-arrow history list, and numeric mode where the up/down arrows
// nudge the number instead of cycling history.
type promptOpts struct {
	status  func() string
	history *[]string
	initial string
	numeric bool
}

// readPrompt reads a single line from the user. On a TTY it runs a small line
// editor in raw mode: backspace, ctrl-u/ctrl-w, arrow keys, up/down history,
// no echo of control bytes, and Ctrl-C aborts. On pipes/tests it falls back
// to a plain blocking line read.
func (sh *Shell) readPrompt(label string, opts promptOpts) (string, bool) {
	restore, err := makeRaw()
	if err != nil {
		fmt.Print(label)
		line, rerr := readPlainLine()
		if rerr != nil {
			return "", false
		}
		return strings.TrimSpace(line), false
	}
	defer restore()
	return sh.editLine(label, opts)
}

// readLineSig is the plain prompt used where no extra options are needed.
func (sh *Shell) readLineSig(label string) (string, bool) {
	return sh.readPrompt(label, promptOpts{})
}

// retainNotes records a submitted note so up/down-arrow can recall it.
func (sh *Shell) retainNotes(note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	if len(sh.noteHistory) > 0 && sh.noteHistory[len(sh.noteHistory)-1] == note {
		return
	}
	sh.noteHistory = append(sh.noteHistory, note)
}

// editLine implements the raw-mode line editor.
func (sh *Shell) editLine(label string, opts promptOpts) (string, bool) {
	showStatus := opts.status != nil && opts.status() != ""
	if showStatus {
		fmt.Print("\n")
	}

	var buf []rune
	if opts.initial != "" {
		buf = []rune(opts.initial)
	}
	pos := len(buf)
	histIdx := 0
	if opts.history != nil {
		histIdx = len(*opts.history)
	}
	lastStatus := ""
	if showStatus {
		lastStatus = opts.status()
	}
	dirty := true

	draw := func() {
		if showStatus {
			st := ""
			if opts.status != nil {
				st = opts.status()
			}
			if st != "" {
				fmt.Printf("\x1b[1A\x1b[2K%s", st)
				fmt.Printf("\x1b[1B")
			}
		}
		fmt.Printf("\r\x1b[K%s%s", label, string(buf))
		if pos < len(buf) {
			fmt.Printf("\x1b[%dG", utf8.RuneCountInString(label)+pos+1)
		}
	}

	var escState int
	var seq []byte
	var draft []rune
	draftSet := false

	for {
		if dirty {
			draw()
			dirty = false
		}

		var b [1]byte
		n, _ := os.Stdin.Read(b[:])
		if n == 0 {
			if showStatus {
				st := opts.status()
				if st != lastStatus {
					lastStatus = st
					dirty = true
				}
			}
			if !dirty {
				time.Sleep(20 * time.Millisecond)
			}
			continue
		}

		c := b[0]
		if escState != 0 {
			escState, dirty = handleEscape(escState, c, &seq, opts, &buf, &pos, &draft, &draftSet, &histIdx)
			continue
		}

		switch {
		case c == 27:
			escState = 1
		case c == 3 || c == 26: // Ctrl-C / Ctrl-Z
			flushStdio()
			clearPrompt(showStatus)
			return "", true
		case c == 4: // Ctrl-D
			if len(buf) == 0 {
				flushStdio()
				clearPrompt(showStatus)
				return "", true
			}
			fallthrough
		case c == 13 || c == 10: // Enter
			line := strings.TrimSpace(string(buf))
			clearPrompt(showStatus)
			fmt.Print("\n")
			return line, false
		case c == 8 || c == 127: // Backspace
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
			}
		case c == 21: // Ctrl-U: clear the line
			buf = nil
			pos = 0
		case c == 23: // Ctrl-W: delete word before the cursor
			pos = deleteWordBackward(buf, pos)
		case c == 1: // Ctrl-A
			pos = 0
		case c == 5: // Ctrl-E
			pos = len(buf)
		case c < 32: // ignore any other control byte, never echo it
		default:
			if opts.initial != "" && pos == len(buf) && string(buf) == opts.initial {
				// a fresh pre-filled value is an editable default: the first
				// keystroke replaces it instead of appending
				buf = nil
				pos = 0
				histIdx = 0
			}
			buf, pos = insertText(buf, pos, c)
		}
		dirty = true
	}
}

// handleEscape consumes the bytes of an escape sequence. Escape sequences
// we do not understand are fully consumed and discarded so no stray bytes
// leak into the typed line.
func handleEscape(state int, c byte, seq *[]byte, opts promptOpts, buf *[]rune, pos *int, draft *[]rune, draftSet *bool, histIdx *int) (int, bool) {
	switch state {
	case 1: // saw ESC alone
		if c == '[' {
			*seq = nil
			return 2, false
		}
		return 0, false // ESC followed by something else: ignore the whole thing
	case 2: // ESC [ ...: accumulate until a final byte (0x40-0x7E)
		if c < 0x40 || c > 0x7E {
			*seq = append(*seq, c)
			return 2, false
		}
		s := string(append(*seq, c))
		*seq = nil
		switch s {
		case "A":
			nudgeNumeric(opts, buf, pos, 1, histIdx, draft, draftSet)
		case "B":
			nudgeNumeric(opts, buf, pos, -1, histIdx, draft, draftSet)
		case "C":
			if *pos < len(*buf) {
				*pos++
			}
		case "D":
			if *pos > 0 {
				*pos--
			}
		case "H":
			*pos = 0
		case "F":
			*pos = len(*buf)
		case "3~": // Delete
			if *pos < len(*buf) {
				*buf = append((*buf)[:*pos], (*buf)[*pos+1:]...)
			}
		}
		return 0, true
	}
	return 0, false
}

// nudgeNumeric adjusts a numeric field with the up/down arrows; for plain
// text fields it cycles the prompt history (saving any draft the user typed).
func nudgeNumeric(opts promptOpts, buf *[]rune, pos *int, dir int, histIdx *int, draft *[]rune, draftSet *bool) {
	if opts.numeric {
		n, ok := parseBufInt(*buf)
		if !ok {
			return
		}
		n += dir
		if n < 0 {
			n = 0
		}
		*buf = []rune(strconv.Itoa(n))
		*pos = len(*buf)
		return
	}
	if opts.history == nil || len(*opts.history) == 0 {
		return
	}
	if dir < 0 { // up: older entry
		if *histIdx == len(*opts.history) && !*draftSet && len(*buf) > 0 {
			*draft = append([]rune(nil), (*buf)...)
			*draftSet = true
		}
		if *histIdx > 0 {
			*histIdx--
		}
		*buf = []rune((*opts.history)[*histIdx])
		*pos = len(*buf)
	} else { // down: newer entry
		if *histIdx+1 < len(*opts.history) {
			*histIdx++
			*buf = []rune((*opts.history)[*histIdx])
			*pos = len(*buf)
		} else if *draftSet {
			*buf = append([]rune(nil), (*draft)...)
			*pos = len(*buf)
			*histIdx = len(*opts.history)
			*draftSet = false
		}
	}
}

func parseBufInt(buf []rune) (int, bool) {
	s := strings.TrimSpace(string(buf))
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// clearPrompt erases the status line (when present) and the prompt line the
// editor was drawing on.
func clearPrompt(showStatus bool) {
	if showStatus {
		fmt.Printf("\x1b[1A\x1b[2K")
		fmt.Printf("\x1b[1B")
	}
	fmt.Print("\r\x1b[K")
}

// flushStdio discards any pending standard-input bytes so a struck key or a
// stray newline does not bleed into the next prompt (non-blocking raw mode).
func flushStdio() {
	for i := 0; i < 100; i++ {
		var b [64]byte
		n, _ := os.Stdin.Read(b[:])
		if n == 0 {
			return
		}
	}
}

// deleteWordBackward removes the word before the cursor.
func deleteWordBackward(buf []rune, pos int) int {
	if pos == 0 {
		return 0
	}
	i := pos
	for i > 0 && buf[i-1] == ' ' {
		i--
	}
	for i > 0 && buf[i-1] != ' ' {
		i--
	}
	return i
}

func insertRune(buf []rune, pos int, r rune) ([]rune, int) {
	if pos < len(buf) {
		buf = append(buf[:pos+1], buf[pos:]...)
		buf[pos] = r
		return buf, pos + 1
	}
	return append(buf, r), len(buf) + 1
}

// insertText inserts a typed byte into the buffer, assembling incomplete UTF-8
// sequences (multi-byte runes) from following input bytes.
func insertText(buf []rune, pos int, first byte) ([]rune, int) {
	if first < 0x80 {
		return insertRune(buf, pos, rune(first))
	}
	size := 0
	switch {
	case first >= 0xF0:
		size = 4
	case first >= 0xE0:
		size = 3
	case first >= 0xC0:
		size = 2
	default:
		return buf, pos // stray continuation byte: drop it
	}
	acc := []byte{first}
	for len(acc) < size {
		var bb [1]byte
		n, _ := os.Stdin.Read(bb[:])
		if n == 0 {
			break
		}
		if bb[0]&0xC0 != 0x80 {
			return buf, pos
		}
		acc = append(acc, bb[0])
	}
	if len(acc) < size {
		return buf, pos
	}
	r, _ := utf8.DecodeRune(acc)
	if r == utf8.RuneError || r == 0 {
		return buf, pos
	}
	return insertRune(buf, pos, r)
}

// readPlainLine reads a single line from stdin for non-TTY (piped) use.
var plainReader *bufio.Reader

func readPlainLine() (string, error) {
	if plainReader == nil {
		plainReader = bufio.NewReader(os.Stdin)
	}
	line, err := plainReader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// playAlarm rings an audible alarm when a timer finishes. On macOS it plays a
// system sound several times in the background (never blocking a prompt);
// elsewhere it falls back to the terminal bell. Volume/count/gap/sound are
// tunable via start flags (see sh.alarm*).
func (sh *Shell) playAlarm() {
	go func() {
		if runtime.GOOS != "darwin" {
			fmt.Print("\a\a\a")
			return
		}
		for i := 0; i < sh.alarmCount; i++ {
			cmd := exec.Command("afplay", "-v", strconv.FormatFloat(sh.alarmVolume, 'f', -1, 64), sh.alarmSound)
			if err := cmd.Run(); err != nil {
				fmt.Print("\a\a\a")
				return
			}
			time.Sleep(sh.alarmGap)
		}
	}()
}
