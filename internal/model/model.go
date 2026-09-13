package model

type Exercise struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Topic     string `json:"topic,omitempty"`
	CreatedAt string `json:"created_at"`
}

type Config struct {
	ExercisesPerRound int `json:"exercises_per_round"`
	DurationSec       int `json:"duration_sec"`
	RestSec           int `json:"rest_sec"`
	BreakSec          int `json:"break_sec"`
}

type Entry struct {
	ExerciseID string `json:"exercise_id"`
	Name       string `json:"name"`
	Round      int    `json:"round"`
	Sequence   int    `json:"sequence"`
	StartBPM   int    `json:"start_bpm,omitempty"`
	EndBPM     int    `json:"end_bpm,omitempty"`
	Notes      string `json:"notes,omitempty"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
}

type Session struct {
	ID        string   `json:"id"`
	StartedAt string   `json:"started_at"`
	EndedAt   string   `json:"ended_at,omitempty"`
	Config    Config   `json:"config"`
	Order     []string `json:"order"`
	Entries   []Entry  `json:"entries"`
}

func MaxRound(entries []Entry) int {
	m := 0
	for _, e := range entries {
		if e.Round > m {
			m = e.Round
		}
	}
	return m
}
