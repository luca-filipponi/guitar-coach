package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

//go:embed app.js
var appJS []byte

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
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset='utf-8'>")
	b.WriteString("<title>guitar-coach</title>")
	b.WriteString("<style>body{font-family:system-ui,sans-serif;margin:2rem;color:#222}table{border-collapse:collapse;margin-top:.5rem}td,th{border:1px solid #ccc;padding:.35rem .6rem;text-align:left}th{background:#f5f5f5}</style>")
	b.WriteString("</head><body>")
	b.WriteString("<h1>guitar-coach</h1>")

	exercises := h.api.ListExercises()
	if len(exercises) == 0 {
		b.WriteString("<p>No exercises yet.</p>")
	} else {
		b.WriteString("<h2>Exercises</h2><table><tr><th>Name</th><th>Topic</th><th>Progress</th></tr>")
		for _, ex := range exercises {
			topic := ""
			if ex.Topic != "" {
				topic = htmlEscape(ex.Topic)
			}
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td><a href='/api/exercises/%s/progress'>data</a></td></tr>", htmlEscape(ex.Name), topic, ex.ID)
		}
		b.WriteString("</table>")

		b.WriteString("<h2>Progress</h2>")
		b.WriteString("<p><label>Exercise: <select id='exercise-select'>")
		for _, ex := range exercises {
			label := htmlEscape(ex.Name)
			if ex.Topic != "" {
				label += " (" + htmlEscape(ex.Topic) + ")"
			}
			fmt.Fprintf(&b, "<option value='%s'>%s</option>", ex.ID, label)
		}
		b.WriteString("</select></label></p>")
		b.WriteString("<div id='chart'><p>Select an exercise to plot its progress.</p></div>")
	}

	sessions := h.api.ListSessions()
	b.WriteString("<h2>Sessions</h2>")
	if len(sessions) == 0 {
		b.WriteString("<p>No sessions yet.</p>")
	} else {
		b.WriteString("<table><tr><th>ID</th><th>Started</th><th>Rounds</th><th>Exercises</th><th>Detail</th></tr>")
		for i := len(sessions) - 1; i >= 0; i-- {
			s := sessions[i]
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td><a href='/api/sessions'>JSON</a></td></tr>",
				s.ID,
				util.ParseTime(s.StartedAt).Local().Format("Jan 2 15:04"),
				model.MaxRound(s.Entries),
				len(s.Entries),
			)
		}
		b.WriteString("</table>")
		b.WriteString("<p>View sessions as JSON at <code>/api/sessions</code> or a specific one at <code>/api/sessions/{id}</code>.</p>")
	}
	b.WriteString("<script src='/app.js'></script>")
	b.WriteString("</body></html>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, b.String())
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
