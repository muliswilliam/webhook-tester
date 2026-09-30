package server

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	metricsMiddleware "github.com/slok/go-http-metrics/middleware"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
	"webhook-tester/internal/web/view"

	"github.com/slok/go-http-metrics/middleware/std"
	"github.com/wader/gormstore/v2"

	"gorm.io/gorm"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
	"webhook-tester/config"
	"webhook-tester/docs"
	"webhook-tester/internal/db"
	"webhook-tester/internal/mailer"
	appMetrics "webhook-tester/internal/metrics"
	"webhook-tester/internal/web/templates"
)

type Server struct {
	Router       *chi.Mux
	DB           *gorm.DB
	SessionStore *gormstore.Store
	Logger       *log.Logger
	Srv          *http.Server
	// MetricsSrv serves Prometheus metrics on a separate, internal-only
	// address, so they aren't exposed on the public port.
	MetricsSrv *http.Server
	// WebhookSvc is set by MountHandlers.
	WebhookSvc *service.WebhookService
}

func (srv *Server) MountHandlers() {
	r := srv.Router
	authSecret := os.Getenv("AUTH_SECRET")
	repo := store.NewGormWebookRepo(srv.DB, srv.Logger)
	userRepo := store.NewGormUserRepo(srv.DB, srv.Logger)
	webhookReqRepo := store.NewGormWebhookRequestRepo(srv.DB, srv.Logger)
	webhookSvc := service.NewWebhookService(repo)
	webhookReqSvc := service.NewWebhookRequestService(webhookReqRepo)
	authSvc := service.NewAuthService(userRepo, srv.DB, authSecret)
	srv.WebhookSvc = webhookSvc
	metricsRec := appMetrics.PrometheusRecorder{}
	view.SetLogger(srv.Logger)
	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/health"))
	r.NotFound(view.RenderNotFound)

	mdlw := metricsMiddleware.New(metricsMiddleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})

	// Instrument all routes
	r.Use(std.HandlerProvider("", mdlw))

	// Static file server for /static/*
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	r.Mount("/", routers.NewWebRouter(webhookReqSvc, webhookSvc, authSvc, mailer.FromEnv(srv.Logger), &metricsRec, srv.Logger))

	r.Mount("/api", routers.NewApiRouter(webhookSvc, authSvc, srv.Logger, &metricsRec))
	r.Mount("/webhooks", routers.NewWebhookRouter(webhookSvc, webhookReqSvc, authSvc, srv.Logger, &metricsRec))

	if srv.MetricsSrv != nil {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		srv.MetricsSrv.Handler = metricsMux
	}

	// API documentation
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, templates.Templates, "docs.html")
	})
	r.Get("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, docs.SwaggerInfo.ReadDoc()); err != nil {
			srv.Logger.Printf("error writing OpenAPI document: %v", err)
		}
	})
}

func NewServer() *Server {
	config.LoadEnv()
	conn := db.Connect()
	db.AutoMigrate(conn)

	r := chi.NewRouter()
	// Cancelled when shutdown starts, so long-lived requests (the SSE
	// streams) end instead of holding up graceful shutdown.
	baseCtx, cancelRequests := context.WithCancel(context.Background())
	srv := http.Server{
		Addr:        ":3000",
		Handler:     r,
		IdleTimeout: time.Minute,
		BaseContext: func(net.Listener) context.Context { return baseCtx },
	}
	srv.RegisterOnShutdown(cancelRequests)

	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":9091"
	}

	return &Server{
		Router:     r,
		DB:         conn,
		Logger:     log.New(os.Stdout, "[server] ", log.LstdFlags),
		Srv:        &srv,
		MetricsSrv: &http.Server{Addr: metricsAddr, ReadHeaderTimeout: 10 * time.Second},
	}
}
