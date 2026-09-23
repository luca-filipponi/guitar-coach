package cli

import (
	"bytes"
	"testing"
)

// TestAwaitEnterRegression is hermetic and deterministic: it drives the real
// primitive awaitEnterBytes directly with a bytes.Reader — no pty, no tty, no
// timing, no writer (the primitive has none, so echoing is structurally
// impossible). It guards the Doing/warmup gate contract: junk keys typed
// during a timer can never auto-gate, a lone Enter always starts, Ctrl-C
// always skips, and EOF/closed-pipe can never wedge the session.
func TestAwaitEnterRegression(t *testing.T) {
	const junk = "s ye asdf asdf a sdf a df"
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"lone LF starts", "\n", true},
		{"lone CR starts", "\r", true},
		{"junk keys then lone LF starts (junk never gates)", junk + "\n", true},
		{"lone Ctrl-C skips", "\x03", false},
		{"stray keys then Ctrl-C still skips", junk + "\x03", false},
		{"junk + EOF starts (closed pipe cannot wedge)", junk, true},
		{"instant EOF starts", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := awaitEnterBytes(bytes.NewReader([]byte(c.in))); got != c.want {
				t.Fatalf("awaitEnterBytes(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
