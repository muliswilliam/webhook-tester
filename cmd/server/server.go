package server

import (
	"fmt"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
	"webhook-tester/internal/utils"

	"github.com/MarceloPetrucio/go-scalar-api-reference"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	metricsMiddleware "github.com/slok/go-http-metrics/middleware"

	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/wader/gormstore/v2"

	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/config"
	_ "webhook-tester/docs"
	"webhook-tester/internal/db"
	appMetrics "webhook-tester/internal/metrics"

	"gorm.io/gorm"
)

type Server struct {
	Router    *chi.Mux
	DB        *gorm.DB
	GormStore *gormstore.Store
	Logger    *log.Logger
	Srv       *http.Server
}

func (srv *Server) MountHandlers() {
	r := srv.Router
	repo := store.NewGormWebookRepo(srv.DB, srv.Logger)
	userRepo := store.NewGormUserRepo(srv.DB, srv.Logger)
	webhookReqRepo := store.NewGormWebhookRequestRepo(srv.DB, srv.Logger)
	webhookSvc := service.NewWebhookService(repo)
	webhookReqSvc := service.NewWebhookRequestService(webhookReqRepo)
	sessionStore := service.NewSessionStore(srv.GormStore)
	authSvc := service.NewAuthService(userRepo, sessionStore, utils.NewPasswordHasher(), utils.NewPasswordValidator())
	metricsRec := appMetrics.PrometheusRecorder{}
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

	mdlw := metricsMiddleware.New(metricsMiddleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})

	// Instrument all routes
	r.Use(std.HandlerProvider("", mdlw))

	// Static file server for /static/*
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Mount("/", routers.NewWebRouter(webhookReqSvc, webhookSvc, authSvc, &metricsRec, srv.Logger))

	r.Mount("/api", routers.NewApiRouter(webhookSvc, authSvc, srv.Logger, &metricsRec))
	r.Mount("/webhooks", routers.NewWebhookRouter(webhookSvc, authSvc, srv.Logger, &metricsRec))

	// metrics
	r.Handle("/metrics", promhttp.Handler())

	// API documentation
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		htmlContent, err := scalar.ApiReferenceHTML(&scalar.Options{
			SpecURL: "./docs/swagger.json",
			CustomOptions: scalar.CustomOptions{
				PageTitle: "Simple API",
			},
			DarkMode: true,
		})

		if err != nil {
			fmt.Printf("%v", err)
		}

		fmt.Fprintln(w, htmlContent)
	})
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

	gs := gormstore.New(conn, []byte(os.Getenv("AUTH_SECRET")))
	quit := make(chan struct{})
	go gs.PeriodicCleanup(48*time.Hour, quit)

	return &Server{
		Router:    r,
		DB:        conn,
		GormStore: gs,
		Logger:    log.New(os.Stdout, "[server] ", log.LstdFlags),
		Srv:       &srv,
	}
}
