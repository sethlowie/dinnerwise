package main

import (
	"log"
	"net/http"
	"os"

	"github.com/sethlowie/dinnerwise/internal/foo"
	"github.com/sethlowie/dinnerwise/internal/foo/v1/foov1connect"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.Handle(foov1connect.NewFooServiceHandler(foo.NewService()))

	log.Printf("server: listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// withCORS allows the Vite dev server (different origin) to call the Connect
// API directly. Permissive on purpose — this is a local dev convenience.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, "+
				"X-Grpc-Web, X-User-Agent")
		w.Header().Set("Access-Control-Expose-Headers",
			"Content-Type, Connect-Protocol-Version, Grpc-Status, Grpc-Message")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
