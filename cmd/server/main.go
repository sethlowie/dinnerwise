package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/sethlowie/dinnerwise/internal/agent"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/meal/v1/mealv1connect"
	"github.com/sethlowie/dinnerwise/internal/recipe"
	"github.com/sethlowie/dinnerwise/internal/recipe/v1/recipev1connect"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbPath := os.Getenv("DINNERWISE_DB")
	if dbPath == "" {
		dbPath = "dinnerwise.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("server: open db: %v", err)
	}
	defer database.Close()

	if err := recipe.Migrate(database); err != nil {
		log.Fatalf("server: migrate: %v", err)
	}
	if err := recipe.SeedIfEmpty(database); err != nil {
		log.Fatalf("server: seed: %v", err)
	}

	if err := meal.Migrate(database); err != nil {
		log.Fatalf("server: meal migrate: %v", err)
	}
	if err := meal.SeedIfEmpty(database); err != nil {
		log.Fatalf("server: meal seed: %v", err)
	}

	repo := recipe.NewRepo(database)
	recipes, err := repo.List(context.Background())
	if err != nil {
		log.Fatalf("server: list recipes: %v", err)
	}
	log.Printf("server: %d recipes loaded from %s", len(recipes), dbPath)

	mux := http.NewServeMux()
	mux.Handle(recipev1connect.NewRecipeServiceHandler(recipe.NewService(repo)))
	mux.Handle(agentv1connect.NewAgentServiceHandler(
		agent.NewService(recipe.NewRepo(database), meal.NewRepo(database)),
	))
	mux.Handle(mealv1connect.NewMealServiceHandler(meal.NewService(meal.NewRepo(database))))

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
