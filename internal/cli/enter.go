package cli

import (
	"io"
	"os"
)

// awaitEnterBytes blocks until a lone Enter arrives on r. Every other byte is
// consumed and ignored, and never echoed — the primitive has no writer, so
// echo is structurally impossible — meaning keys typed while a Doing timer runs
// can never auto-gate. It returns true when a lone Enter starts the phase and
// false when 0x03 (Ctrl-C) skips it. EOF is treated as a start so a closed or
// wedged stdin can never hang the session.
func awaitEnterBytes(r io.Reader) bool {
	for {
		var b [1]byte
		n, _ := r.Read(b[:])
		if n == 0 {
			return true
		}
		switch b[0] {
		case '\r', '\n':
			return true
		case 0x03:
			return false
		}
	}
}

// awaitEnter re-enters raw mode (so keys are ignored rather than echoed),
// drains any keys buffered during the previous timer, and awaits a lone
// Enter. Returns false when 0x03 (Ctrl-C) skips.
func (sh *Shell) awaitEnter() bool {
	restore, err := makeRaw()
	if err != nil {
		return awaitEnterBytes(os.Stdin)
	}
	defer restore()
	flushStdio()
	return awaitEnterBytes(os.Stdin)
}
