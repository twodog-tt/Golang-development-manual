package lab

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

//go:embed schema.sql
var Schema string

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	wrap := func(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
			w.Header().Set("Content-Type", "application/json")
			if err := f(w, r); err != nil {
				status := http.StatusUnprocessableEntity
				if errors.Is(err, ErrConflict) {
					status = 409
				}
				if errors.Is(err, pgx.ErrNoRows) {
					status = 404
				}
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			}
		}
	}
	reply := func(w http.ResponseWriter, v any) error { return json.NewEncoder(w).Encode(v) }
	mux.HandleFunc("GET /health", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var height, epoch, quality int64
		var hash string
		if err := a.DB.QueryRow(r.Context(), "SELECT height,hash,epoch,(SELECT count(*) FROM quality_errors) FROM index_meta WHERE id=1").Scan(&height, &hash, &epoch, &quality); err != nil {
			return err
		}
		head, err := a.RPC.HeaderByNumber(r.Context(), nil)
		if err != nil {
			return err
		}
		return reply(w, map[string]any{"chain": 31337, "indexed": height, "indexed_hash": hash, "head": head.Number.Int64(), "lag": head.Number.Int64() - height, "epoch": epoch, "quality_errors_total": quality})
	}))
	mux.HandleFunc("POST /intents/{id}", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var request Request
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		d.DisallowUnknownFields()
		if err := d.Decode(&request); err != nil {
			return err
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return errors.New("one JSON object required")
		}
		if err := a.Create(r.Context(), r.PathValue("id"), request); err != nil {
			return err
		}
		return reply(w, map[string]string{"id": r.PathValue("id"), "state": "persisted; broadcast separately"})
	}))
	mux.HandleFunc("GET /intents/{id}", wrap(func(w http.ResponseWriter, r *http.Request) error {
		out, err := a.Reconcile(r.Context(), r.PathValue("id"))
		if err != nil {
			return err
		}
		return reply(w, out)
	}))
	mux.HandleFunc("POST /intents/{id}/broadcast", wrap(func(w http.ResponseWriter, r *http.Request) error {
		if err := a.Broadcast(r.Context(), r.PathValue("id")); err != nil {
			return err
		}
		return reply(w, map[string]bool{"submitted": true})
	}))
	mux.HandleFunc("POST /intents/{id}/bump", wrap(func(w http.ResponseWriter, r *http.Request) error {
		if err := a.Bump(r.Context(), r.PathValue("id")); err != nil {
			return err
		}
		return reply(w, map[string]bool{"persisted": true})
	}))
	mux.HandleFunc("POST /index/sync", wrap(func(w http.ResponseWriter, r *http.Request) error {
		if err := a.Sync(r.Context()); err != nil {
			return err
		}
		return reply(w, map[string]bool{"indexed": true})
	}))
	mux.HandleFunc("GET /events", wrap(func(w http.ResponseWriter, r *http.Request) error {
		limit := 20
		if s := r.URL.Query().Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			limit = n
		}
		out, err := a.Events(r.Context(), r.URL.Query().Get("cursor"), limit)
		if err != nil {
			return err
		}
		return reply(w, out)
	}))
	return mux
}
