package api

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder wraps http.ResponseWriter so we can capture the status
// code the handler wrote. If the handler never calls WriteHeader, Go
// implicitly uses 200 — so we default to that.

type statusRecorder struct {
	http.ResponseWriter
	status int
}


// writeheader funtion

func (r *statusRecorder) WriteHeader(code int){
	r.status = code;
	r.ResponseWriter.WriteHeader(code)
}


// withLogging wraps h so every request is logged with method, path,
// status, and latency.

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		start := time.Now();
		rec := &statusRecorder{
			ResponseWriter: w,
			status: http.StatusOK,
		}

		next.ServeHTTP(rec, r);

		log.Printf("%s %s %d %s",
			r.Method,
			r.URL.Path,
			rec.status,
			time.Since(start),
		)
	})
}


