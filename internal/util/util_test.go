package util

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{1 * time.Second, "1s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1:00"},
		{5*time.Minute + 3*time.Second, "5:03"},
		{2 * time.Hour, "120:00"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.in); got != c.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"300", 300 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"1m30s", 90 * time.Second, false},
		{"0", 0, true},
		{"-5", 0, true},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if c.wantErr != (err != nil) {
			t.Errorf("ParseDuration(%q) err = %v, wantErr = %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
