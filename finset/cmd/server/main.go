package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/finset/app/internal/db"
	"github.com/finset/app/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	h := handlers.New(nil)
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.Health)
		r.Get("/debug", h.Debug)
		r.Get("/stats", h.GetStats)
		r.Get("/monthly-flow", h.GetMonthlyFlow)
		r.Get("/category-breakdown", h.GetCategoryBreakdown)
		r.Get("/transactions", h.ListTransactions)
		r.Post("/transactions", h.CreateTransaction)
		r.Delete("/transactions/all", h.DeleteAllTransactions)
		r.Get("/transactions/{id}", h.GetTransaction)
		r.Delete("/transactions/{id}", h.DeleteTransaction)
		r.Post("/import", h.ImportTransactions)
		r.Post("/quiz/attempts", h.SubmitQuizAttempt)
		r.Get("/quiz/dashboard", h.GetQuizDashboard)
	})

	// Serve frontend — all non-API routes return index.html
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		serveFrontend(w, req)
	})

	go func() {
		for {
			pool, err := db.Connect()
			if err != nil {
				log.Printf("DB connection deferred: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			if err := pool.Migrate(); err != nil {
				log.Printf("DB migration deferred: %v", err)
				pool.Close()
				time.Sleep(5 * time.Second)
				continue
			}
			h.SetDB(pool)
			log.Printf("Database ready")
			return
		}
	}()

	log.Printf("FinSet listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
