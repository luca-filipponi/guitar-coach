package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

//go:embed app.js
var appJS []byte

//go:embed static
var staticFS embed.FS

const customCSS = `<style>
:root{--pico-background-color:#0a0e13;--pico-primary:#4f9cf9;--pico-card-background-color:#121826}
body{background:
 radial-gradient(1100px 520px at 85% -12%,#15233b 0%,transparent 60%),
 radial-gradient(900px 480px at -8% 112%,#1c1830 0%,transparent 55%),
 #0a0e13;background-attachment:fixed}
.wrap{max-width:1080px;margin:0 auto;padding:1.5rem 1.5rem 3rem}
header.top{display:flex;align-items:center;gap:.85rem;padding-bottom:1.1rem;border-bottom:1px solid var(--pico-muted-border-color);margin-bottom:1.6rem}
header.top .logo{width:46px;height:46px;flex:none;filter:drop-shadow(0 2px 6px rgba(79,156,249,.25))}
header.top h1{margin:0 0 .1rem;font-size:1.4rem}
header.top .tag{color:var(--pico-muted-color);font-size:.82rem}
h2{margin:2rem 0 .4rem}
.pill{display:inline-block;background:#1b2840;border:1px solid #2b3e5e;color:#9ec3ef;border-radius:999px;padding:.08rem .6rem;font-size:.77rem}
.hint{color:var(--pico-muted-color);font-size:.83rem}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:.75rem;margin:1.2rem 0 2rem}
.stat{background:var(--pico-card-background-color);border:1px solid var(--pico-muted-border-color);border-radius:12px;padding:.9rem 1rem}
.stat b{display:block;font-size:1.4rem}
.stat span{color:var(--pico-muted-color);font-size:.77rem}
#chart{margin-top:.7rem}
.chart-note{position:absolute;z-index:5;pointer-events:none;max-width:280px;background:rgba(10,14,19,.94);border:1px solid #31404f;border-radius:8px;padding:.45rem .65rem;font-size:.8rem;line-height:1.4;color:#e8edf4;box-shadow:0 4px 14px rgba(0,0,0,.45);display:none}
.chart-note b{color:var(--pico-primary);white-space:nowrap}
.chart-note span{color:#c2ccd9}
.desc{display:block;color:#c9d6e3;font-size:.86rem;margin-top:.12rem}
tr[data-href]{cursor:pointer}
tr[data-href]:hover td{text-decoration:underline}
.pager{display:flex;gap:1rem;align-items:center;justify-content:space-between;margin-top:.7rem;color:var(--pico-muted-color);font-size:.9rem}
.exblock{margin:0 0 1.8rem}
.exblock h3{margin:.5rem 0 .1rem}
.exchart{display:block;max-width:100%;height:auto;margin:.6rem 0 .2rem}
.exchart text{font-family:var(--pico-font-family)}
.spark{display:block;width:110px;height:auto}
</style>`

const logoSVG = `<svg viewBox='0 0 64 64' class='logo' aria-hidden='true' xmlns='http://www.w3.org/2000/svg'>
<defs><linearGradient id='gcoat' x1='0' y1='0' x2='1' y2='1'><stop offset='0' stop-color='#4f9cf9'/><stop offset='1' stop-color='#7a5cf0'/></linearGradient></defs>
<path d='M32 20.6 L27.2 20.6 C23.3 20.6 20.2 22.9 19 25 C16 27.4 14.4 30.8 15.6 34.2 C16.7 36.9 18.8 38.8 21 39.9 L18.8 44.2 C14.6 47.4 13.8 53 17.6 56.2 C21.2 59.2 28 60.8 32 60.8 C36 60.8 42.8 59.2 46.4 56.2 C50.2 53 49.4 47.4 45.2 44.2 L43 39.9 C45.2 38.8 47.3 36.9 48.4 34.2 C49.6 30.8 48 27.4 45 25 C43.8 22.9 40.7 20.6 36.8 20.6 Z' fill='url(#gcoat)'/>
<rect x='29.8' y='7' width='4.4' height='16' rx='1.2' fill='#e8edf4'/>
<rect x='28.2' y='2' width='7.6' height='7' rx='1.4' fill='#232d3f' stroke='#4f9cf9' stroke-width='.6'/>
<g stroke='#e8edf4' stroke-width='.5' opacity='.85'>
<line x1='30.3' y1='4.5' x2='30.3' y2='52'/>
<line x1='31.6' y1='4.5' x2='31.3' y2='52'/>
<line x1='32.9' y1='4.5' x2='33.6' y2='52'/>
<line x1='34.1' y1='4.5' x2='34.1' y2='52'/>
</g>
<circle cx='32' cy='34' r='1.6' fill='#0a0e13'/>
</svg>`

func pageHeader(title, tagline string) string {
	return "<header class='top'>" + logoSVG +
		"<div><h1>" + htmlEscape(title) + "</h1><span class='tag'>" + htmlEscape(tagline) + "</span></div></header>"
}

func writePage(w http.ResponseWriter, title, body string, withScript bool) {
	var b strings.Builder
	b.WriteString("<!doctype html><html lang='en' data-theme='dark'><head><meta charset='utf-8'>")
	b.WriteString("<meta name='viewport' content='width=device-width,initial-scale=1'>")
	fmt.Fprintf(&b, "<title>%s</title>", htmlEscape(title))
	b.WriteString("<link rel='stylesheet' href='/static/pico.min.css'>")
	b.WriteString("<link rel='stylesheet' href='/static/uplot.min.css'>")
	b.WriteString(customCSS)
	b.WriteString("</head><body>")
	b.WriteString(body)
	if withScript {
		b.WriteString("<script src='/static/uplot.iife.min.js'></script>")
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
	mux.HandleFunc("PUT /api/exercises/{id}/description", h.setDescription)
	mux.HandleFunc("GET /api/exercises/{id}/progress", h.exerciseProgress)
	mux.HandleFunc("GET /api/sessions", h.listSessions)
	mux.HandleFunc("POST /api/sessions", h.createSession)
	mux.HandleFunc("GET /api/sessions/{id}", h.getSession)
	mux.HandleFunc("GET /sessions/{id}", h.sessionDetail)
	mux.HandleFunc("POST /api/sessions/{id}/entries", h.addEntry)
	mux.HandleFunc("POST /api/sessions/{id}/end", h.endSession)
	mux.HandleFunc("GET /{$}", h.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSub(staticFS, "static")))))
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

func mustSub(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

type handlers struct {
	api *api.API
}

func (h *handlers) listExercises(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.api.ListExercises())
}

func (h *handlers) createExercise(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Topic       string `json:"topic"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ex, err := h.api.CreateExercise(req.Name, req.Topic, req.Description)
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

func (h *handlers) setDescription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ex, err := h.api.SetDescription(r.PathValue("id"), req.Description)
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

const sessionsPerPage = 15
const exercisesPerPage = 15

func pageURL(topic string, exPage, sessPage int) string {
	var q []string
	if topic != "" {
		q = append(q, "topic="+url.QueryEscape(topic))
	}
	if exPage > 1 {
		q = append(q, fmt.Sprintf("ex_page=%d", exPage))
	}
	if sessPage > 1 {
		q = append(q, fmt.Sprintf("page=%d", sessPage))
	}
	if len(q) == 0 {
		return "/"
	}
	return "/?" + strings.Join(q, "&")
}

func (h *handlers) index(w http.ResponseWriter, r *http.Request) {
	exercises := h.api.ListExercises()
	sessions := h.api.ListSessions()

	topic := strings.TrimSpace(r.URL.Query().Get("topic"))
	topicSet := make(map[string]bool)
	for _, ex := range exercises {
		if ex.Topic != "" {
			topicSet[ex.Topic] = true
		}
	}
	if topic != "" && !topicSet[topic] {
		topic = ""
	}

	filtered := make([]model.Exercise, 0, len(exercises))
	for _, ex := range exercises {
		if topic == "" || ex.Topic == topic {
			filtered = append(filtered, ex)
		}
	}

	exPage := 1
	if v := r.URL.Query().Get("ex_page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p >= 1 {
			exPage = p
		}
	}
	exTotal := 1
	if len(filtered) > 0 {
		exTotal = (len(filtered) + exercisesPerPage - 1) / exercisesPerPage
	}
	if exPage > exTotal {
		exPage = exTotal
	}

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p >= 1 {
			page = p
		}
	}
	totalPages := 1
	if len(sessions) > 0 {
		totalPages = (len(sessions) + sessionsPerPage - 1) / sessionsPerPage
	}
	if page > totalPages {
		page = totalPages
	}

	var b strings.Builder
	b.WriteString("<div class='wrap'>")
	b.WriteString(pageHeader("guitar-coach", "deliberate practice, tracked"))
	b.WriteString(renderStats(computeStats(sessions, exercises)))
	b.WriteString("<p class='hint'>Timed practice sessions run from the terminal: <code>guitar-coach start</code>. This dashboard is read-only for now.</p>")

	b.WriteString("<h2>Exercises</h2>")
	if len(filtered) == 0 {
		if topic != "" {
			b.WriteString("<p>No exercises for this topic.</p>")
		} else {
			b.WriteString("<p>No exercises yet.</p>")
		}
	} else {
		fmt.Fprintf(&b, "<p class='hint'>%d exercises", len(filtered))
		if topic != "" {
			fmt.Fprintf(&b, " (topic: %s)", htmlEscape(topic))
		}
		if len(filtered) > exercisesPerPage {
			fmt.Fprintf(&b, ", showing %d per page", exercisesPerPage)
		}
		b.WriteString("</p>")
		b.WriteString("<table><tr><th>Name</th><th>Topic</th><th>Trend</th><th>Detail</th></tr>")
		exHi := len(filtered) - 1 - (exPage-1)*exercisesPerPage
		exLo := exHi - exercisesPerPage + 1
		if exLo < 0 {
			exLo = 0
		}
		for i := exHi; i >= exLo; i-- {
			ex := filtered[i]
			topicHTML := ""
			if ex.Topic != "" {
				topicHTML = "<span class='pill'>" + htmlEscape(ex.Topic) + "</span>"
			}
			desc := ""
			if ex.Description != "" {
				desc = "<span class='desc' title='" + htmlEscape(ex.Description) + "'>" + htmlEscape(ex.Description) + "</span>"
			}
			fmt.Fprintf(&b,
				"<tr data-topic='%s'><td>%s%s</td><td>%s</td><td>%s</td><td><a href='/api/exercises/%s/progress'>data</a></td></tr>",
				htmlEscape(ex.Topic),
				htmlEscape(ex.Name), desc,
				topicHTML,
				renderSparkline(h.api.ExerciseProgress(ex.ID)),
				ex.ID)
		}
		b.WriteString("</table>")
		writeExercisePager(&b, exPage, exTotal, topic, page)

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
				sel := ""
				if t == topic {
					sel = " selected"
				}
				fmt.Fprintf(&b, "<option value='%s'%s>%s</option>", htmlEscape(t), sel, htmlEscape(t))
			}
			b.WriteString("</select></label></p>")
		}
		b.WriteString("<p><label>Exercise: <select id='exercise-select'>")
		for _, ex := range filtered {
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
		fmt.Fprintf(&b, "<p class='hint'>%d sessions, showing %d per page — click a row for the full history</p>", len(sessions), sessionsPerPage)
		b.WriteString("<table><tr><th>ID</th><th>Started</th><th>Rounds</th><th>Exercises</th><th>Length</th><th>Detail</th></tr>")
		hi := len(sessions) - 1 - (page-1)*sessionsPerPage
		lo := hi - sessionsPerPage + 1
		if lo < 0 {
			lo = 0
		}
		for i := hi; i >= lo; i-- {
			s := sessions[i]
			fmt.Fprintf(&b, "<tr data-href='/sessions/%s'><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td><td><a href='/api/sessions/%s'>json</a></td></tr>",
				s.ID,
				s.ID,
				util.ParseTime(s.StartedAt).Local().Format("Jan 2 15:04"),
				model.MaxRound(s.Entries),
				len(s.Entries),
				util.FormatDuration(serverSessionLength(s)),
				s.ID,
			)
		}
		b.WriteString("</table>")
		writeSessionPager(&b, page, totalPages, topic, exPage)
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

type chartPoint struct {
	t          time.Time
	start, end int
}

func chartPoints(pts []api.ProgressPoint) []chartPoint {
	out := make([]chartPoint, 0, len(pts))
	for _, p := range pts {
		if p.EndBPM <= 0 {
			continue
		}
		out = append(out, chartPoint{t: util.ParseTime(p.Date), start: p.StartBPM, end: p.EndBPM})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].t.Before(out[j].t) })
	return out
}

func renderSparkline(pts []api.ProgressPoint) string {
	cp := chartPoints(pts)
	if len(cp) == 0 {
		return "<span class='hint'>no data</span>"
	}
	const W, H = 110, 26
	lo, hi := cp[0].end, cp[0].end
	best := 0
	for _, p := range cp {
		if p.end < lo {
			lo = p.end
		}
		if p.end > hi {
			hi = p.end
		}
		if p.end > best {
			best = p.end
		}
	}
	if lo == hi {
		lo, hi = lo-1, hi+1
	}
	x := func(i int) float64 { return 3 + float64(i)/float64(len(cp)-1)*(W-6) }
	y := func(b int) float64 { return H - 4 - float64(b-lo)/float64(hi-lo)*(H-8) }
	var b strings.Builder
	b.WriteString("<svg class='spark' viewBox='0 0 110 26' role='img' aria-label='trend' xmlns='http://www.w3.org/2000/svg'>")
	b.WriteString("<polyline fill='none' stroke='#5aa2e8' stroke-width='1.5' stroke-linejoin='round' points='")
	for i, p := range cp {
		fmt.Fprintf(&b, "%.1f,%.1f ", x(i), y(p.end))
	}
	b.WriteString("'/>")
	for i, p := range cp {
		fill := "#5aa2e8"
		if p.end == best {
			fill = "#ffd45e"
		}
		fmt.Fprintf(&b, "<circle cx='%.1f' cy='%.1f' r='1.8' fill='%s'/>", x(i), y(p.end), fill)
	}
	b.WriteString("</svg>")
	return b.String()
}

func renderMiniChart(pts []api.ProgressPoint) string {
	cp := chartPoints(pts)
	if len(cp) == 0 {
		return "<p class='hint'>no history yet</p>"
	}
	const W, H = 520, 150
	const PL, PR, PT, PB = 40, 8, 16, 24
	iw, ih := W-PL-PR, H-PT-PB
	lo, hi := cp[0].end, cp[0].end
	best := 0
	for _, p := range cp {
		if p.end < lo {
			lo = p.end
		}
		if p.end > hi {
			hi = p.end
		}
		if p.start > 0 && p.start < lo {
			lo = p.start
		}
		if p.start > hi {
			hi = p.start
		}
		if p.end > best {
			best = p.end
		}
	}
	pad := (hi - lo) / 4
	if pad < 5 {
		pad = 5
	}
	lo = ((lo - pad) / 10) * 10
	hi = ((hi + pad) / 10) * 10
	if hi <= lo {
		hi = lo + 10
	}
	t0, t1 := cp[0].t, cp[len(cp)-1].t
	span := t1.Sub(t0).Seconds()
	if span <= 0 {
		span = 1
	}
	x := func(t time.Time) float64 { return PL + float64(t.Sub(t0).Seconds())/span*float64(iw) }
	y := func(v int) float64 { return PT + float64(ih) - float64(v-lo)/float64(hi-lo)*float64(ih) }
	step := 10
	if hi-lo > 80 {
		step = 20
	}
	if hi-lo > 200 {
		step = 50
	}
	var b strings.Builder
	b.WriteString("<svg class='exchart' viewBox='0 0 520 150' role='img' aria-label='exercise BPM history' xmlns='http://www.w3.org/2000/svg'>")
	b.WriteString("<rect width='520' height='150' rx='8' fill='#0f141c'/>")
	for v := ((lo + step - 1) / step) * step; v <= hi; v += step {
		yy := y(v)
		fmt.Fprintf(&b, "<line x1='%d' y1='%.1f' x2='%d' y2='%.1f' stroke='#1b2634'/>", PL, yy, W-PR, yy)
		fmt.Fprintf(&b, "<text x='%d' y='%.1f' text-anchor='end' fill='#9fb2c6'>%d</text>", PL-5, yy+3, v)
	}
	fmt.Fprintf(&b, "<text x='%d' y='%d' text-anchor='start' fill='#9fb2c6'>%s</text>", PL, H-7, t0.Format("Jan 2"))
	fmt.Fprintf(&b, "<text x='%d' y='%d' text-anchor='end' fill='#9fb2c6'>%s</text>", W-PR, H-7, t1.Format("Jan 2"))
	b.WriteString("<polyline fill='none' stroke='#5aa2e8' stroke-opacity='.45' stroke-width='1' stroke-linejoin='round' points='")
	hasStart := false
	for _, p := range cp {
		if p.start > 0 {
			hasStart = true
		}
	}
	if hasStart {
		for _, p := range cp {
			if p.start > 0 {
				fmt.Fprintf(&b, "%.1f,%.1f ", x(p.t), y(p.start))
			} else {
				fmt.Fprintf(&b, "%.1f,%.1f ", x(p.t), y(p.end))
			}
		}
	}
	b.WriteString("'/>")
	b.WriteString("<polyline fill='none' stroke='#f0875a' stroke-width='1.6' stroke-linejoin='round' points='")
	for _, p := range cp {
		fmt.Fprintf(&b, "%.1f,%.1f ", x(p.t), y(p.end))
	}
	b.WriteString("'/>")
	for _, p := range cp {
		if p.end == best {
			fmt.Fprintf(&b, "<path d='M%.1f %.1f l2 3.2 l-4 0 z' fill='#ffd45e'/>", x(p.t), y(p.end)-2.2)
		} else {
			fmt.Fprintf(&b, "<circle cx='%.1f' cy='%.1f' r='2.2' fill='#f0875a'/>", x(p.t), y(p.end))
		}
	}
	b.WriteString("</svg>")
	fmt.Fprintf(&b, "<div class='hint'>best %d bpm · %d sets</div>", best, len(cp))
	return b.String()
}

func writeExercisePager(b *strings.Builder, exPage, exTotal int, topic string, sessPage int) {
	if exTotal <= 1 {
		return
	}
	b.WriteString("<div class='pager'>")
	if exPage > 1 {
		fmt.Fprintf(b, "<a href='%s'>← newer</a>", htmlEscape(pageURL(topic, exPage-1, sessPage)))
	} else {
		b.WriteString("<span></span>")
	}
	fmt.Fprintf(b, "<span>exercises page %d / %d</span>", exPage, exTotal)
	if exPage < exTotal {
		fmt.Fprintf(b, "<a href='%s'>older →</a>", htmlEscape(pageURL(topic, exPage+1, sessPage)))
	} else {
		b.WriteString("<span></span>")
	}
	b.WriteString("</div>")
}

func writeSessionPager(b *strings.Builder, sessPage, sessTotal int, topic string, exPage int) {
	if sessTotal <= 1 {
		return
	}
	b.WriteString("<div class='pager'>")
	if sessPage > 1 {
		fmt.Fprintf(b, "<a href='%s'>← newer</a>", htmlEscape(pageURL(topic, exPage, sessPage-1)))
	} else {
		b.WriteString("<span></span>")
	}
	fmt.Fprintf(b, "<span>page %d / %d</span>", sessPage, sessTotal)
	if sessPage < sessTotal {
		fmt.Fprintf(b, "<a href='%s'>older →</a>", htmlEscape(pageURL(topic, exPage, sessPage+1)))
	} else {
		b.WriteString("<span></span>")
	}
	b.WriteString("</div>")
}

func (h *handlers) sessionDetail(w http.ResponseWriter, r *http.Request) {
	sess, err := h.api.GetSession(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	nameByID := make(map[string]string)
	exByID := make(map[string]model.Exercise)
	for _, ex := range h.api.ListExercises() {
		nameByID[ex.ID] = ex.Name
		exByID[ex.ID] = ex
	}
	var b strings.Builder
	b.WriteString("<div class='wrap'>")
	b.WriteString("<p><a href='/?page=1'>← dashboard</a></p>")
	fmt.Fprintf(&b, "<h1>session %s</h1>", sess.ID)
	fmt.Fprintf(&b, "<p>started %s", formatWebTime(sess.StartedAt))
	if sess.EndedAt != "" {
		fmt.Fprintf(&b, " · ended %s", formatWebTime(sess.EndedAt))
	}
	if d := serverSessionLength(sess); d > 0 {
		fmt.Fprintf(&b, " · length %s", util.FormatDuration(d))
	}
	fmt.Fprintf(&b, "</p>")
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
	seen := make(map[string]bool)
	var uniq []string
	for _, id := range sess.Order {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, ok := exByID[id]; ok {
			uniq = append(uniq, id)
		}
	}
	if len(uniq) > 0 {
		b.WriteString("<h2>Exercise history</h2>")
		b.WriteString("<p class='hint'>Progress of each exercise across all its sessions, not just this one.</p>")
		for _, id := range uniq {
			ex := exByID[id]
			fmt.Fprintf(&b, "<div class='exblock'><h3>%s", htmlEscape(ex.Name))
			if ex.Topic != "" {
				fmt.Fprintf(&b, " <span class='pill'>%s</span>", htmlEscape(ex.Topic))
			}
			b.WriteString("</h3>")
			if ex.Description != "" {
				fmt.Fprintf(&b, "<p class='desc'>%s</p>", htmlEscape(ex.Description))
			}
			b.WriteString(renderMiniChart(h.api.ExerciseProgress(id)))
			b.WriteString("</div>")
		}
	}
	b.WriteString("</div>")
	writePage(w, "session "+sess.ID, b.String(), false)
}

func formatWebTime(rfc string) string {
	t := util.ParseTime(rfc)
	if t.IsZero() {
		return "n/a"
	}
	return t.Local().Format("Jan 2, 2006 15:04")
}

// serverSessionLength is the wall-clock time from a session start to its end
// (or the last recorded entry if there is no end timestamp).
func serverSessionLength(s model.Session) time.Duration {
	start := util.ParseTime(s.StartedAt)
	end := util.ParseTime(s.EndedAt)
	for _, e := range s.Entries {
		if t := util.ParseTime(e.FinishedAt); t.After(end) {
			end = t
		}
	}
	if end.After(start) {
		return end.Sub(start)
	}
	return 0
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
