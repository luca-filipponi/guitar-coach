package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

//go:embed app.js
var appJS []byte

const pageCSS = `<style>body{background:#0b0f14;color:#d7dee6;margin:2rem;font-family:system-ui,sans-serif}` +
	`table{border-collapse:collapse;margin-top:.5rem}td,th{border:1px solid #26313d;padding:.35rem .6rem;text-align:left}th{background:#141b24;color:#9aa7b3}` +
	`a{color:#5aa2e8}code{background:#141b24;padding:.1rem .25rem;border-radius:4px}select,input{background:#141b24;color:#d7dee6;border:1px solid #26313d;padding:.25rem .4rem;border-radius:4px}` +
	`button{background:#1d7af2;color:#fff;border:0;border-radius:6px;padding:.35rem .75rem;cursor:pointer}` +
	`form#log-form{display:flex;gap:.5rem;flex-wrap:wrap;margin:.5rem 0}` +
	`.stats{display:flex;gap:.75rem;flex-wrap:wrap;margin-bottom:1.5rem}.stat{border:1px solid #26313d;border-radius:8px;padding:.5rem .9rem;background:#121924;min-width:110px}` +
	`.stat b{display:block;font-size:1.5rem;color:#5aa2e8}.stat span{color:#93a2b3;font-size:.8rem}</style>`

func writePage(w http.ResponseWriter, title, body string, withScript bool) {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset='utf-8'>")
	fmt.Fprintf(&b, "<title>%s</title>", htmlEscape(title))
	b.WriteString(pageCSS)
	b.WriteString("</head><body>")
	b.WriteString(body)
	if withScript {
		b.WriteString("<script src='/app.js'></script>")
	}
	b.WriteString("</body></html>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, b.String())
}

func Listen(addr string, a *api.API, stop <-chan os.Signal) error {
	mux := http.NewServeMux()
	h := &handlers{api: a}

	mux.HandleFunc("GET /api/exercises", h.listExercises)
	mux.HandleFunc("POST /api/exercises", h.createExercise)
	mux.HandleFunc("DELETE /api/exercises/{id}", h.deleteExercise)
	mux.HandleFunc("PUT /api/exercises/{id}/topic", h.setTopic)
	mux.HandleFunc("GET /api/exercises/{id}/progress", h.exerciseProgress)
	mux.HandleFunc("GET /api/sessions", h.listSessions)
	mux.HandleFunc("POST /api/sessions", h.createSession)
	mux.HandleFunc("GET /api/sessions/{id}", h.getSession)
	mux.HandleFunc("GET /sessions/{id}", h.sessionDetail)
	mux.HandleFunc("POST /api/sessions/{id}/entries", h.addEntry)
	mux.HandleFunc("POST /api/sessions/{id}/end", h.endSession)
	mux.HandleFunc("GET /{$}", h.index)
	mux.HandleFunc("GET /app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(appJS)
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	fmt.Printf("guitar-coach UI on http://localhost%s (Ctrl-C to quit)\n", addr)
	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		fmt.Printf("\nreceived %s, shutting down...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

type handlers struct {
	api *api.API
}

func (h *handlers) listExercises(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.api.ListExercises())
}

func (h *handlers) createExercise(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Topic string `json:"topic"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ex, err := h.api.CreateExercise(req.Name, req.Topic)
	if errors.Is(err, api.ErrExists) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, ex)
}

func (h *handlers) deleteExercise(w http.ResponseWriter, r *http.Request) {
	if err := h.api.DeleteExercise(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) setTopic(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic string `json:"topic"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ex, err := h.api.SetTopic(r.PathValue("id"), req.Topic)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, ex)
}

func (h *handlers) exerciseProgress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.api.GetExercise(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, h.api.ExerciseProgress(id))
}

func (h *handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.api.ListSessions())
}

func (h *handlers) createSession(w http.ResponseWriter, r *http.Request) {
	var req api.StartSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sess, err := h.api.StartSession(req)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (h *handlers) getSession(w http.ResponseWriter, r *http.Request) {
	sess, err := h.api.GetSession(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (h *handlers) addEntry(w http.ResponseWriter, r *http.Request) {
	var entry model.Entry
	if err := decodeJSON(r, &entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if entry.StartedAt == "" {
		entry.StartedAt = util.NowRFC()
	}
	if entry.FinishedAt == "" {
		entry.FinishedAt = util.NowRFC()
	}
	sess, err := h.api.AddSessionEntry(r.PathValue("id"), entry)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (h *handlers) endSession(w http.ResponseWriter, r *http.Request) {
	sess, err := h.api.EndSession(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	exercises := h.api.ListExercises()
	sessions := h.api.ListSessions()

	var b strings.Builder
	b.WriteString("<h1>guitar-coach</h1>")
	b.WriteString(renderStats(computeStats(sessions, exercises)))

	b.WriteString("<h2>Quick log</h2>")
	b.WriteString("<form id='log-form'>")
	b.WriteString("<select id='log-exercise'>")
	for _, ex := range exercises {
		label := htmlEscape(ex.Name)
		if ex.Topic != "" {
			label += " (" + htmlEscape(ex.Topic) + ")"
		}
		fmt.Fprintf(&b, "<option value='%s'>%s</option>", ex.ID, label)
	}
	b.WriteString("</select>")
	b.WriteString("<input type='number' id='log-start' placeholder='start bpm' min='0' style='width:7rem'>")
	b.WriteString("<input type='number' id='log-end' placeholder='end bpm' min='0' style='width:7rem'>")
	b.WriteString("<input type='text' id='log-notes' placeholder='notes' style='width:16rem'>")
	b.WriteString("<button type='submit'>Log set</button>")
	b.WriteString("</form><p id='log-status'></p>")

	if len(exercises) == 0 {
		b.WriteString("<p>No exercises yet.</p>")
	} else {
		b.WriteString("<h2>Exercises</h2><table><tr><th>Name</th><th>Topic</th><th>Progress</th></tr>")
		for _, ex := range exercises {
			topic := ""
			if ex.Topic != "" {
				topic = htmlEscape(ex.Topic)
			}
			fmt.Fprintf(&b, "<tr data-topic='%s'><td>%s</td><td>%s</td><td><a href='/api/exercises/%s/progress'>data</a></td></tr>", htmlEscape(ex.Topic), htmlEscape(ex.Name), topic, ex.ID)
		}
		b.WriteString("</table>")

		b.WriteString("<h2>Progress</h2>")
		seenTopic := make(map[string]bool)
		var topics []string
		for _, ex := range exercises {
			if ex.Topic != "" && !seenTopic[ex.Topic] {
				seenTopic[ex.Topic] = true
				topics = append(topics, ex.Topic)
			}
		}
		if len(topics) > 0 {
			b.WriteString("<p><label>Topic: <select id='topic-filter'><option value=''>All topics</option>")
			for _, t := range topics {
				fmt.Fprintf(&b, "<option value='%s'>%s</option>", htmlEscape(t), htmlEscape(t))
			}
			b.WriteString("</select></label></p>")
		}
		b.WriteString("<p><label>Exercise: <select id='exercise-select'>")
		for _, ex := range exercises {
			label := htmlEscape(ex.Name)
			if ex.Topic != "" {
				label += " (" + htmlEscape(ex.Topic) + ")"
			}
			fmt.Fprintf(&b, "<option value='%s' data-topic='%s'>%s</option>", ex.ID, htmlEscape(ex.Topic), label)
		}
		b.WriteString("</select></label></p>")
		b.WriteString("<div id='chart'><p>Select an exercise to plot its progress.</p></div>")
	}

	b.WriteString("<h2>Sessions</h2>")
	if len(sessions) == 0 {
		b.WriteString("<p>No sessions yet.</p>")
	} else {
		b.WriteString("<table><tr><th>ID</th><th>Started</th><th>Rounds</th><th>Exercises</th><th>Detail</th></tr>")
		for i := len(sessions) - 1; i >= 0; i-- {
			s := sessions[i]
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td><a href='/sessions/%s'>view</a> <a href='/api/sessions/%s'>json</a></td></tr>",
				s.ID,
				util.ParseTime(s.StartedAt).Local().Format("Jan 2 15:04"),
				model.MaxRound(s.Entries),
				len(s.Entries),
				s.ID,
				s.ID,
			)
		}
		b.WriteString("</table>")
	}

	writePage(w, "guitar-coach", b.String(), true)
}

type uiStats struct {
	sessions, sets, minutes int
	avgEnd, bestBPM         int
	bestName                string
	streak                  int
}

func computeStats(sessions []model.Session, exercises []model.Exercise) uiStats {
	var st uiStats
	nameByID := make(map[string]string, len(exercises))
	for _, ex := range exercises {
		nameByID[ex.ID] = ex.Name
	}
	daySet := make(map[string]bool)
	st.sessions = len(sessions)
	var totalMinutes, totalEnd int
	endCount := 0
	for _, s := range sessions {
		daySet[util.ParseTime(s.StartedAt).Local().Format("2006-01-02")] = true
		for _, e := range s.Entries {
			st.sets++
			start := util.ParseTime(e.StartedAt)
			finish := util.ParseTime(e.FinishedAt)
			if finish.After(start) {
				totalMinutes += int(finish.Sub(start).Minutes())
			}
			if e.EndBPM > 0 {
				totalEnd += e.EndBPM
				endCount++
				if e.EndBPM > st.bestBPM {
					st.bestBPM = e.EndBPM
					st.bestName = nameByID[e.ExerciseID]
				}
			}
		}
	}
	st.minutes = totalMinutes
	if endCount > 0 {
		st.avgEnd = int(math.Round(float64(totalEnd) / float64(endCount)))
	}
	cursor := time.Now()
	if !daySet[cursor.Format("2006-01-02")] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for daySet[cursor.Format("2006-01-02")] {
		st.streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return st
}

func renderStats(st uiStats) string {
	var b strings.Builder
	card := func(v, label string) {
		fmt.Fprintf(&b, "<div class='stat'><b>%s</b><span>%s</span></div>", v, label)
	}
	b.WriteString("<div class='stats'>")
	card(strconv.Itoa(st.sessions), "sessions")
	card(strconv.Itoa(st.sets), "sets logged")
	card(strconv.Itoa(st.minutes), "min practiced")
	avg := "—"
	if st.avgEnd > 0 {
		avg = strconv.Itoa(st.avgEnd) + " bpm"
	}
	card(avg, "avg end bpm")
	best := "—"
	bestLabel := "best"
	if st.bestBPM > 0 {
		best = strconv.Itoa(st.bestBPM) + " bpm"
		bestLabel = "best: " + htmlEscape(st.bestName)
	}
	card(best, bestLabel)
	card(strconv.Itoa(st.streak), "day streak")
	b.WriteString("</div>")
	return b.String()
}

func (h *handlers) sessionDetail(w http.ResponseWriter, r *http.Request) {
	sess, err := h.api.GetSession(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	nameByID := make(map[string]string)
	for _, ex := range h.api.ListExercises() {
		nameByID[ex.ID] = ex.Name
	}
	var b strings.Builder
	b.WriteString("<p><a href='/'>← all sessions</a></p>")
	fmt.Fprintf(&b, "<h1>session %s</h1>", sess.ID)
	fmt.Fprintf(&b, "<p>started %s</p>", formatWebTime(sess.StartedAt))
	if sess.EndedAt != "" {
		fmt.Fprintf(&b, "<p>ended %s</p>", formatWebTime(sess.EndedAt))
	}
	fmt.Fprintf(&b, "<p>plan: %d exercises, %s each, %s rest, %s break</p>",
		sess.Config.ExercisesPerRound,
		util.FormatDuration(time.Duration(sess.Config.DurationSec)*time.Second),
		util.FormatDuration(time.Duration(sess.Config.RestSec)*time.Second),
		util.FormatDuration(time.Duration(sess.Config.BreakSec)*time.Second),
	)
	names := make([]string, 0, len(sess.Order))
	for _, id := range sess.Order {
		if n, ok := nameByID[id]; ok {
			names = append(names, n)
		}
	}
	if len(names) > 0 {
		fmt.Fprintf(&b, "<p>order: %s</p>", htmlEscape(strings.Join(names, " → ")))
	}
	round := 0
	for _, e := range sess.Entries {
		if e.Round != round {
			round = e.Round
			fmt.Fprintf(&b, "<h2>round %d</h2>", round)
		}
		name := e.Name
		if name == "" {
			name = nameByID[e.ExerciseID]
		}
		bpm := "n/a"
		if e.StartBPM > 0 || e.EndBPM > 0 {
			bpm = fmt.Sprintf("%d → %d", e.StartBPM, e.EndBPM)
		}
		fmt.Fprintf(&b, "<p>%d. %s <code>%s bpm</code>", e.Sequence, htmlEscape(name), bpm)
		if e.Notes != "" {
			fmt.Fprintf(&b, " <span style='color:#93a2b3'>%s</span>", htmlEscape(e.Notes))
		}
		b.WriteString("</p>")
	}
	writePage(w, "session "+sess.ID, b.String(), false)
}

func formatWebTime(rfc string) string {
	t := util.ParseTime(rfc)
	if t.IsZero() {
		return "n/a"
	}
	return t.Local().Format("Jan 2, 2006 15:04")
}

func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
