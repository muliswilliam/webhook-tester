package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/config"
	"webhook-tester/internal/api"
	"webhook-tester/internal/db"
	"webhook-tester/internal/web"
	"webhook-tester/internal/web/sessions"
	"webhook-tester/internal/webhook"
)

type Server struct {
	Router       *chi.Mux
	DB           *gorm.DB
	SessionStore *gormstore.Store
	Logger       *log.Logger
	Srv          *http.Server
}

func (srv *Server) MountHandlers() {
	r := srv.Router

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

	// Static file server for /static/*
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Mount("/", web.Router(srv.DB, srv.SessionStore, srv.Logger))
	r.Mount("/api", api.Router(srv.DB, srv.SessionStore, srv.Logger))
	r.Mount("/webhooks", webhook.NewRouter(srv.DB, srv.SessionStore, srv.Logger))
}

func NewServer() *Server {
	config.LoadEnv()
	conn := db.Connect()
	db.AutoMigrate(conn)

	r := chi.NewRouter()
	srv := http.Server{
		Addr:        ":3000",
		Handler:     r,
		IdleTimeout: time.Minute,
	}

	return &Server{
		Router:       r,
		DB:           conn,
		SessionStore: sessions.CreateSessionStore(conn),
		Logger:       log.New(os.Stdout, "[server] ", log.LstdFlags),
		Srv:          &srv,
	}
}
