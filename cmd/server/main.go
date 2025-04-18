package main

import (
	"fmt"
	"log"
	"net/http"
	"webhook-tester/config"
	"webhook-tester/internal/api"
	"webhook-tester/internal/db"
	"webhook-tester/internal/web"
	"webhook-tester/internal/web/sessions"
	"webhook-tester/internal/webhook"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/health"))

	return r
}

func main() {
	config.LoadEnv()

	db.Connect()
	db.AutoMigrate()

	r := NewRouter()
	sessions.CreateSessionStore()

	// Static file server for /static/*
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Mount("/", web.NewRouter())
	r.Mount("/api", api.NewRouter())
	r.Mount("/webhooks", webhook.NewRouter())

	fmt.Println("Server running on http://localhost:3000")
	err := http.ListenAndServe(":3000", r)
	if err != nil {
		log.Fatal("Failed to start server", err)
	}
}
