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
	"sync/atomic"
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
:root{
 --pico-background-color:#0a0e13;
 --pico-primary:#4f9cf9;
 --pico-primary-hover:#6db0ff;
 --pico-card-background-color:#111827;
 --pico-muted-border-color:#223052;
 --pico-muted-color:#93a3bb;
 --pico-font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Inter",ui-sans-serif,"Helvetica Neue",Arial,sans-serif;
}
html{scrollbar-color:#2b3b5c transparent}
*{scrollbar-width:thin}
::-webkit-scrollbar{width:11px;height:11px}
::-webkit-scrollbar-thumb{background:#2b3b5c;border-radius:8px;border:3px solid #0a0e13}
::-webkit-scrollbar-track{background:transparent}
::selection{background:rgba(79,156,249,.35);color:#fff}
body{
 background:
  radial-gradient(1000px 500px at 85% -10%,#15233b 0%,transparent 60%),
  radial-gradient(800px 460px at -10% 108%,#1c1830 0%,transparent 55%),
  #0a0e13;
 background-attachment:fixed;
 -webkit-font-smoothing:antialiased;
}
.wrap{max-width:1080px;margin:0 auto;padding:1rem 1.5rem 4rem}
header.top{
 position:sticky;top:0;z-index:20;
 display:flex;align-items:center;gap:.9rem;
 padding:.6rem 0 .9rem;margin-bottom:1.4rem;
 background:linear-gradient(180deg,rgba(10,14,19,.96) 55%,rgba(10,14,19,0));
 -webkit-backdrop-filter:blur(10px);backdrop-filter:blur(10px);
}
header.top .logo{width:42px;height:42px;flex:none;filter:drop-shadow(0 3px 8px rgba(79,156,249,.35))}
header.top h1{margin:0 0 .05rem;font-size:1.35rem;font-weight:700;letter-spacing:-.01em}
header.top .tag{color:var(--pico-muted-color);font-size:.82rem}
h2{margin:2.2rem 0 .6rem;font-size:1.02rem;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:#c8d4e5}
.pill{display:inline-block;background:linear-gradient(180deg,#16233d,#101a30);border:1px solid #2c4370;color:#9dc2f0;border-radius:999px;padding:.1rem .65rem;font-size:.76rem;font-weight:500;box-shadow:inset 0 1px 0 rgba(255,255,255,.05)}
.hint{color:var(--pico-muted-color);font-size:.84rem}
.legend{display:flex;flex-wrap:wrap;gap:.6rem 1.3rem;color:var(--pico-muted-color);font-size:.8rem;margin-top:.55rem}
.legend span{display:inline-flex;align-items:center;gap:.4rem}
.legend i{display:inline-block;width:18px;height:3px;border-radius:2px;background:var(--c,#ccc)}
.legend .gold{width:0;height:0;border-left:5px solid transparent;border-right:5px solid transparent;border-bottom:7px solid #ffd45e;background:none}
.legend .dot{width:7px;height:7px;border-radius:50%;background:var(--c,#ccc)}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:.8rem;margin:1.2rem 0 2rem}
.stat{position:relative;overflow:hidden;background:linear-gradient(160deg,#141d2f,#0e1523);border:1px solid #223052;border-radius:14px;padding:1rem 1.05rem;box-shadow:0 6px 20px rgba(0,0,0,.3);transition:transform .18s ease,border-color .18s ease}
.stat:hover{transform:translateY(-2px);border-color:#33507f}
.stat::before{content:'';position:absolute;inset:0 0 auto 0;height:2px;background:linear-gradient(90deg,var(--sc,#4f9cf9),transparent)}
.stat::after{content:'';position:absolute;right:-18px;top:-18px;width:56px;height:56px;border-radius:50%;background:radial-gradient(closest-side,var(--sc,#4f9cf9) 0%,transparent 70%);opacity:.14;pointer-events:none}
.stat b{display:block;font-size:1.5rem;font-weight:700;letter-spacing:-.01em;font-variant-numeric:tabular-nums}
.stat span{color:var(--pico-muted-color);font-size:.76rem;text-transform:uppercase;letter-spacing:.04em}
.stat:nth-child(1){--sc:#4f9cf9}.stat:nth-child(2){--sc:#7a5cf0}.stat:nth-child(3){--sc:#4bd484}.stat:nth-child(4){--sc:#f0875a}.stat:nth-child(5){--sc:#ffd45e}.stat:nth-child(6){--sc:#e85f9c}
table{width:100%;border-collapse:separate;border-spacing:0;background:rgba(17,24,39,.7);border:1px solid #1d2a44;border-radius:12px;overflow:hidden;box-shadow:0 8px 28px rgba(0,0,0,.28)}
table th{text-align:left;font-size:.74rem;letter-spacing:.05em;text-transform:uppercase;color:var(--pico-muted-color);background:rgba(23,32,52,.9);padding:.7rem .95rem;border-bottom:1px solid #1d2a44;font-weight:600;white-space:nowrap}
table td{padding:.7rem .95rem;border-bottom:1px solid rgba(29,42,68,.55);font-size:.92rem;line-height:1.45}
table tbody tr:last-child td{border-bottom:0}
table tbody tr{transition:background .15s ease}
tr[data-href],tr[data-topic]{cursor:pointer}
tbody tr[data-href]:hover td,tbody tr[data-topic]:hover td{background:rgba(79,156,249,.08)}
tbody tr[data-href]:hover td:first-child,tbody tr[data-topic]:hover td:first-child{box-shadow:inset 2px 0 0 var(--pico-primary)}
#chart{margin-top:.7rem}
.u-legend{color:#c2ccd9;font-size:.8rem;padding:.35rem .6rem;background:rgba(10,14,19,.65);border:1px solid #1d2a44;border-radius:8px;margin-bottom:.55rem}
.u-legend th{font-weight:600}
.chart-note{position:absolute;z-index:5;pointer-events:none;max-width:280px;background:rgba(10,14,19,.96);border:1px solid #31404f;border-radius:10px;padding:.5rem .7rem;font-size:.8rem;line-height:1.45;color:#e8edf4;box-shadow:0 8px 24px rgba(0,0,0,.55);display:none}
.chart-note b{color:var(--pico-primary);white-space:nowrap}
.chart-note span{color:#c2ccd9}
.desc{display:block;color:#c9d6e3;font-size:.86rem;margin-top:.12rem}
.pager{display:flex;gap:.5rem;align-items:center;justify-content:space-between;margin-top:.9rem;color:var(--pico-muted-color);font-size:.9rem}
.pager a{text-decoration:none;font-weight:600}
.exblock{margin:0 0 1.8rem}
.exblock h3{margin:.6rem 0 .15rem;font-size:1.05rem}
.exchart{display:block;max-width:100%;height:auto;margin:.7rem 0 .25rem;filter:drop-shadow(0 10px 26px rgba(0,0,0,.4))}
.exchart text{font-family:var(--pico-font-family)}
.spark{display:block;width:110px;height:auto}
.entry{display:flex;align-items:center;gap:.6rem;padding:.55rem .8rem;border:1px solid #1d2a44;border-radius:10px;background:rgba(17,24,39,.55);margin-bottom:.5rem;flex-wrap:wrap;transition:border-color .15s ease}
.entry:hover{border-color:#2b3f6b}
.entry .exname{color:var(--pico-primary);font-weight:600;min-width:8rem}
.entry input[type=number],.entry input[type=text],.entry select{background:#0a0e13;border:1px solid #223052;border-radius:8px;padding:.32rem .5rem;color:#e8edf4}
.entry input[type=number]{width:4.6rem}
.entry input[type=text]{flex:1;min-width:12rem}
.entry input:focus,.entry select:focus{outline:2px solid var(--pico-primary);outline-offset:1px;border-color:var(--pico-primary)}
.entry .saved{opacity:0;color:#4bd484;font-size:.8rem;transition:opacity .25s}
.entry.saving .saved{opacity:1}
button,.btn{transition:filter .15s ease,transform .1s ease}
button:hover,.btn:hover{filter:brightness(1.08)}
button:active,.btn:active{transform:translateY(1px)}
:is(input,select,button):focus-visible{outline:2px solid var(--pico-primary);outline-offset:2px}
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
	mux.HandleFunc("GET /exercises/{id}", h.exerciseProgressPage)
	mux.HandleFunc("GET /sessions/{id}", h.sessionDetail)
	mux.HandleFunc("POST /api/sessions/{id}/entries", h.addEntry)
	mux.HandleFunc("PUT /api/sessions/{id}/entries/{n}", h.updateEntry)
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

// exerciseProgressPage renders an exercise's history as an HTML table instead
// of the raw JSON the API endpoint returns.
func (h *handlers) exerciseProgressPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ex, err := h.api.GetExercise(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	pts := h.api.ExerciseProgress(id)

	var b strings.Builder
	b.WriteString("<div class='wrap'>")
	b.WriteString("<p><a href='/?ex_page=1'>← dashboard</a></p>")
	fmt.Fprintf(&b, "<h1>%s</h1>", htmlEscape(ex.Name))
	if ex.Topic != "" {
		fmt.Fprintf(&b, "<p><span class='pill'>%s</span></p>", htmlEscape(ex.Topic))
	}
	if ex.Description != "" {
		fmt.Fprintf(&b, "<p class='desc'>%s</p>", htmlEscape(ex.Description))
	}
	if len(pts) == 0 {
		b.WriteString("<p>No history yet for this exercise.</p>")
	} else {
		b.WriteString(renderMiniChart(pts))
		fmt.Fprintf(&b, "<p class='hint'>%d recorded sets</p>", len(pts))
		b.WriteString("<table><tr><th>When</th><th>Session</th><th>Round</th><th>Start</th><th>End</th><th>Notes</th></tr>")
		for i := len(pts) - 1; i >= 0; i-- {
			p := pts[i]
			start := "—"
			if p.StartBPM > 0 {
				start = strconv.Itoa(p.StartBPM)
			}
			end := "—"
			if p.EndBPM > 0 {
				end = strconv.Itoa(p.EndBPM)
			}
			notes := ""
			if p.Notes != "" {
				notes = htmlEscape(p.Notes)
			}
			fmt.Fprintf(&b, "<tr><td>%s</td><td><a href='/sessions/%s'>%s</a></td><td>r%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
				formatWebTime(p.Date), p.SessionID, p.SessionID, p.Round, start, end, notes)
		}
		b.WriteString("</table>")
	}
	b.WriteString("</div>")
	writePage(w, ex.Name, b.String(), false)
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

func (h *handlers) updateEntry(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		http.Error(w, "entry number must be an integer", http.StatusBadRequest)
		return
	}
	var req struct {
		Notes    *string `json:"notes"`
		StartBPM *int    `json:"start_bpm"`
		EndBPM   *int    `json:"end_bpm"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sess, err := h.api.UpdateSessionEntry(r.PathValue("id"), idx, func(e *model.Entry) {
		if req.Notes != nil {
			e.Notes = *req.Notes
		}
		if req.StartBPM != nil {
			e.StartBPM = *req.StartBPM
		}
		if req.EndBPM != nil {
			e.EndBPM = *req.EndBPM
		}
	})
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, sess)
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
	b.WriteString("<p class='hint'>Timed practice sessions run from the terminal: <code>guitar-coach start</code>. Open a session to fix entries.</p>")

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
				"<tr data-topic='%s'><td>%s%s</td><td>%s</td><td>%s</td><td><a href='/exercises/%s'>history</a></td></tr>",
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
		b.WriteString("<table><tr><th>ID</th><th>Started</th><th>Rounds</th><th>Exercises</th><th>Length</th></tr>")
		hi := len(sessions) - 1 - (page-1)*sessionsPerPage
		lo := hi - sessionsPerPage + 1
		if lo < 0 {
			lo = 0
		}
		for i := hi; i >= lo; i-- {
			s := sessions[i]
			fmt.Fprintf(&b, "<tr data-href='/sessions/%s'><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
				s.ID,
				s.ID,
				util.ParseTime(s.StartedAt).Local().Format("Jan 2 15:04"),
				model.MaxRound(s.Entries),
				len(s.Entries),
				util.FormatDuration(serverSessionLength(s)),
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

// fpt is a float point used to build smooth SVG paths.
type fpt struct{ x, y float64 }

// smoothPath converts sample points into a catmull-rom cubic bézier path, so
// charts render as curvy lines instead of hard polyline segments.
func smoothPath(pts []fpt) string {
	if len(pts) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "M%.2f %.2f", pts[0].x, pts[0].y)
	for i := 0; i < len(pts)-1; i++ {
		p0 := pts[max(i-1, 0)]
		p1 := pts[i]
		p2 := pts[i+1]
		p3 := pts[min(i+2, len(pts)-1)]
		c1x := p1.x + (p2.x-p0.x)/6
		c1y := p1.y + (p2.y-p0.y)/6
		c2x := p2.x - (p3.x-p1.x)/6
		c2y := p2.y - (p3.y-p1.y)/6
		fmt.Fprintf(&b, " C%.2f %.2f %.2f %.2f %.2f %.2f", c1x, c1y, c2x, c2y, p2.x, p2.y)
	}
	return b.String()
}

// fillPath closes a smooth path down to a baseline so the chart can be filled
// with a gradient under the line. base is the y coordinate of the floor.
func fillPath(pts []fpt, base float64) string {
	if len(pts) == 0 {
		return ""
	}
	return smoothPath(pts) + fmt.Sprintf(" L%.2f %.2f L%.2f %.2f Z", pts[len(pts)-1].x, base, pts[0].x, base)
}

var idCounter atomic.Uint64

func nextID() uint64 { return idCounter.Add(1) }

func renderSparkline(pts []api.ProgressPoint) string {
	cp := chartPoints(pts)
	if len(cp) == 0 {
		return "<span class='hint'>no data</span>"
	}
	const W, H = 110, 26
	lo, hi := cp[0].end, cp[0].end
	for _, p := range cp {
		if p.end < lo {
			lo = p.end
		}
		if p.end > hi {
			hi = p.end
		}
	}
	if lo == hi {
		lo, hi = lo-1, hi+1
	}
	x := func(i int) float64 { return 2 + float64(i)/float64(len(cp)-1)*(W-4) }
	y := func(b int) float64 { return 3 + float64(hi-b)/float64(hi-lo)*(H-6) }
	ptsF := make([]fpt, len(cp))
	for i, p := range cp {
		ptsF[i] = fpt{x(i), y(p.end)}
	}
	gid := fmt.Sprintf("sp%d", nextID())
	var b strings.Builder
	b.WriteString("<svg class='spark' viewBox='0 0 110 26' role='img' aria-label='trend' xmlns='http://www.w3.org/2000/svg'>")
	fmt.Fprintf(&b, "<defs><linearGradient id='%s' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#5aa2e8' stop-opacity='.55'/><stop offset='1' stop-color='#5aa2e8' stop-opacity='.02'/></linearGradient></defs>", gid)
	fmt.Fprintf(&b, "<path d='%s' fill='url(#%s)'/>", fillPath(ptsF, H-3), gid)
	fmt.Fprintf(&b, "<path d='%s' fill='none' stroke='#5aa2e8' stroke-width='1.6' stroke-linecap='round' stroke-linejoin='round'/>", smoothPath(ptsF))
	b.WriteString("</svg>")
	return b.String()
}

func formatDelta(d int) string {
	if d >= 0 {
		return fmt.Sprintf("+%d bpm all time", d)
	}
	return fmt.Sprintf("%d bpm all time", d)
}

func deltaColor(d int) string {
	if d > 0 {
		return "#4bd484"
	}
	if d < 0 {
		return "#f0875a"
	}
	return "#93a3bb"
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

	startPts := make([]fpt, 0, len(cp))
	endPts := make([]fpt, 0, len(cp))
	hasStart := false
	for _, p := range cp {
		ex := x(p.t)
		endPts = append(endPts, fpt{ex, y(p.end)})
		if p.start > 0 {
			hasStart = true
		}
		if p.start > 0 {
			startPts = append(startPts, fpt{ex, y(p.start)})
		} else {
			startPts = append(startPts, fpt{ex, y(p.end)})
		}
	}
	base := float64(H - PB)
	endID := fmt.Sprintf("endg%d", nextID())
	startID := fmt.Sprintf("startg%d", nextID())

	var b strings.Builder
	b.WriteString("<svg class='exchart' viewBox='0 0 520 150' role='img' aria-label='exercise BPM history' xmlns='http://www.w3.org/2000/svg'>")
	b.WriteString("<defs>")
	fmt.Fprintf(&b,
		"<linearGradient id='%s' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#f0875a' stop-opacity='.32'/><stop offset='1' stop-color='#f0875a' stop-opacity='.02'/></linearGradient>", endID)
	fmt.Fprintf(&b,
		"<linearGradient id='%s' x1='0' y1='0' x2='0' y2='1'><stop offset='0' stop-color='#5aa2e8' stop-opacity='.25'/><stop offset='1' stop-color='#5aa2e8' stop-opacity='0'/></linearGradient>", startID)
	b.WriteString("</defs>")
	fmt.Fprintf(&b, "<rect width='520' height='150' rx='10' fill='#0f141c'/>")
	// horizontal gridlines (dashed) with labels
	for v := ((lo + step - 1) / step) * step; v <= hi; v += step {
		yy := y(v)
		fmt.Fprintf(&b, "<line x1='%d' y1='%.1f' x2='%d' y2='%.1f' stroke='#1d2940' stroke-dasharray='3 5'/>", PL, yy, W-PR, yy)
		fmt.Fprintf(&b, "<text x='%d' y='%.1f' text-anchor='end' fill='#8fa3b8' font-size='9'>%d</text>", PL-6, yy+3, v)
	}
	// date labels
	fmt.Fprintf(&b, "<text x='%d' y='%d' text-anchor='start' fill='#8fa3b8' font-size='9'>%s</text>", PL, H-7, t0.Format("Jan 2"))
	fmt.Fprintf(&b, "<text x='%d' y='%d' text-anchor='end' fill='#8fa3b8' font-size='9'>%s</text>", W-PR, H-7, t1.Format("Jan 2"))
	// gradient fills under each line
	fmt.Fprintf(&b, "<path d='%s' fill='url(#%s)'/>", fillPath(endPts, base), endID)
	if hasStart {
		fmt.Fprintf(&b, "<path d='%s' fill='url(#%s)'/>", fillPath(startPts, base), startID)
	}
	// start line (dashed, so it reads as a secondary series)
	if hasStart {
		fmt.Fprintf(&b, "<path d='%s' fill='none' stroke='#5aa2e8' stroke-width='1.4' stroke-linecap='round' stroke-linejoin='round' stroke-dasharray='1 4'/>", smoothPath(startPts))
	}
	// end line (the primary series)
	fmt.Fprintf(&b, "<path d='%s' fill='none' stroke='#f0875a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'/>", smoothPath(endPts))
	// per-point dots, gold on record values
	for _, p := range cp {
		if p.end == best {
			fmt.Fprintf(&b, "<path d='M%.1f %.1f l2 3.4 l-4 0 z' fill='#ffd45e'/>", x(p.t), y(p.end)-2.6)
		} else {
			fmt.Fprintf(&b, "<circle cx='%.1f' cy='%.1f' r='2.2' fill='#f0875a'/>", x(p.t), y(p.end))
		}
	}
	if len(cp) == 1 {
		fmt.Fprintf(&b, "<circle cx='%.1f' cy='%.1f' r='3' fill='#f0875a'/>", x(cp[0].t), y(cp[0].end))
	}
	b.WriteString("</svg>")
	delta := cp[len(cp)-1].end - cp[0].end
	fmt.Fprintf(&b, "<div class='legend'><span><i style='--c:#f0875a'></i>end bpm</span><span><i style='--c:#5aa2e8'></i>start bpm</span>")
	if best > 0 {
		fmt.Fprintf(&b, "<span><i class='gold'></i>record %d bpm</span>", best)
	}
	fmt.Fprintf(&b, "<span><i class='dot' style='--c:%s'></i>%s</span>", deltaColor(delta), formatDelta(delta))
	fmt.Fprintf(&b, "<span>%d sets</span></div>", len(cp))
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
	b.WriteString("<div class='wrap' data-session='" + sess.ID + "'>")
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
	for i, e := range sess.Entries {
		if e.Round != round {
			round = e.Round
			fmt.Fprintf(&b, "<h2>round %d</h2>", round)
		}
		name := e.Name
		if name == "" {
			name = nameByID[e.ExerciseID]
		}
		fmt.Fprintf(&b, "<div class='entry'><span>%d.</span> <span class='exname'>%s</span>", e.Sequence, htmlEscape(name))
		fmt.Fprintf(&b, "<input type='number' data-idx='%d' data-field='start_bpm' value='%d' title='start bpm'>", i, e.StartBPM)
		b.WriteString(" → ")
		fmt.Fprintf(&b, "<input type='number' data-idx='%d' data-field='end_bpm' value='%d' title='end bpm'>", i, e.EndBPM)
		b.WriteString(" bpm")
		fmt.Fprintf(&b, "<input type='text' data-idx='%d' data-field='notes' value='%s' placeholder='notes'>", i, htmlEscape(e.Notes))
		b.WriteString("<span class='saved'>saved</span></div>")
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
	writePage(w, "session "+sess.ID, b.String(), true)
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
