package app

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"net/http"
	"ohmymc/internal/safefile"
	"strings"
)

// Handler is a development-only loopback bridge. It uses an unpredictable session
// token, strict Host/Origin checking, no CORS, and a JSON-only API to reject CSRF.
func Handler(service *Service, frontend fs.FS, host string) http.Handler {
	token := safefile.ID() + safefile.ID()
	static := http.FileServer(http.FS(frontend))
	index, _ := fs.ReadFile(frontend, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if r.Host != host {
			http.Error(w, "Host rejected", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/api" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" && origin != "http://"+host {
				http.Error(w, "Origin rejected", 403)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-OhMyMC-Token")), []byte(token)) != 1 {
				http.Error(w, "Session token required", 403)
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "JSON required", 415)
				return
			}
			var req struct {
				Method  string         `json:"method"`
				Payload map[string]any `json:"payload"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
			if err := dec.Decode(&req); err != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}
			result, err := service.Invoke(r.Context(), req.Method, req.Payload)
			response := map[string]any{"result": result}
			if err != nil {
				response["error"] = err.Error()
			}
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "GET required", 405)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.Method != "HEAD" {
				_, _ = w.Write([]byte(strings.ReplaceAll(string(index), "__API_TOKEN__", token)))
			}
			return
		}
		static.ServeHTTP(w, r)
	})
}
