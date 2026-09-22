package cli

import "os"

// color wraps s in ANSI SGR codes only when stdout is a real terminal, so
// piped or redirected output (e.g. `make test-session > file`) stays plain.
func color(code, s string) string {
	fi, err := os.Stdout.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// accent is the warm orange used for exercise names.
const accent = "38;5;209"

// gold is used for bpm values, echoing the chart's PB gold.
const gold = "38;5;220"

func accentText(s string) string { return color(accent, s) }
func bpmText(s string) string    { return color(gold, s) }
