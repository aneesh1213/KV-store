package api

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/aneesh1213/kvstore/backend/internals/store"
)

type Server struct {
	store *store.Store
	http  *http.Server
}


func New(s *store.Store, addr string) *Server {
	srv := &Server{
		store: s,
	}

	mux := http.NewServeMux();
	mux.HandleFunc("PUT /kv/{key}", srv.handleSet)
	mux.HandleFunc("GET /kv/{key}", srv.handleGet)
	mux.HandleFunc("DELETE /kv/{key}", srv.handleDelete)
	mux.HandleFunc("GET /health", srv.handleHealth)

	srv.http = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return srv
}


// ListenAndServe starts the HTTP server. It blocks until the server
// is shut down.
func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}


// shutdown gracefully
func (s *Server) ShutDown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// hadnlers


// 1. handle set
func (s *Server) handleSet(w http.ResponseWriter, r *http.Request){
	key := r.PathValue("key");

	// read body
	body, err := io.ReadAll(r.Body);
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return;
	}

	if err := s.store.Set(key, string(body)); err != nil {
		http.Error(w, "store error: "+err.Error(), http.StatusInternalServerError)
	    return
	}

	w.WriteHeader(http.StatusOK)
}


// handle Get

func (s *Server) handleGet (w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	v, err := s.store.Get(key)
	if errors.Is(err, store.NotFound) {
	    http.Error(w, "not found", http.StatusNotFound)
	    return
	}
	if err != nil {
	    http.Error(w, "store error", http.StatusInternalServerError)
	    return
	}
	w.Write([]byte(v))
}


// handle Delete

func (s *Server) handleDelete (w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	if err := s.store.Delete(key); err != nil {
	    http.Error(w, "store error", http.StatusInternalServerError)
	    return
	}
	w.WriteHeader(http.StatusNoContent)
}


// handle health 

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}