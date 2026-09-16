package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/luca-filipponi/guitar-coach/internal/api"
	"github.com/luca-filipponi/guitar-coach/internal/model"
	"github.com/luca-filipponi/guitar-coach/internal/util"
)

//go:embed static
var staticFS embed.FS

func Listen(addr string, a *api.API, stop <-chan os.Signal) error {
	mux := http.NewServeMux()
	h := &handlers{api: a}

	// JSON API — the single source of truth for the SPA.
	mux.HandleFunc("GET /api/exercises", h.listExercises)
	mux.HandleFunc("POST /api/exercises", h.createExercise)
	mux.HandleFunc("GET /api/exercises/{id}", h.getExercise)
	mux.HandleFunc("DELETE /api/exercises/{id}", h.deleteExercise)
	mux.HandleFunc("PUT /api/exercises/{id}/topic", h.setTopic)
	mux.HandleFunc("PUT /api/exercises/{id}/description", h.setDescription)
	mux.HandleFunc("GET /api/exercises/{id}/progress", h.exerciseProgress)
	mux.HandleFunc("GET /api/sessions", h.listSessions)
	mux.HandleFunc("POST /api/sessions", h.createSession)
	mux.HandleFunc("GET /api/sessions/{id}", h.getSession)
	mux.HandleFunc("POST /api/sessions/{id}/entries", h.addEntry)
	mux.HandleFunc("PUT /api/sessions/{id}/entries/{n}", h.updateEntry)
	mux.HandleFunc("POST /api/sessions/{id}/end", h.endSession)

	// Static assets (SPA shell + JS + CSS + vendored libs), then SPA fallback.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSub(staticFS, "static")))))
	mux.HandleFunc("GET /{$}", h.spa)
	mux.HandleFunc("GET /{path...}", h.spa)

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

// spa serves the single index.html shell for every non-API route; the client
// router owns navigation from there.
func (h *handlers) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	index, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

type handlers struct {
	api *api.API
}

func (h *handlers) listExercises(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.api.ListExercises())
}

func (h *handlers) getExercise(w http.ResponseWriter, r *http.Request) {
	ex, err := h.api.GetExercise(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, ex)
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
		fmt.Printf("writeJSON: %v\n", err)
	}
}