package routers

import (
	"log"
	"net/http"
	"webhook-tester/internal/dtos"
	"webhook-tester/internal/handlers"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

	"github.com/go-chi/chi/v5"
)

func NewApiRouter(webhookSvc *service.WebhookService, authSvc *service.AuthService, l *log.Logger, metricsRec metrics.Recorder) http.Handler {
	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		utils.RenderJSON(w, http.StatusNotFound, dtos.ErrorResponse{Error: "not found"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		utils.RenderJSON(w, http.StatusMethodNotAllowed, dtos.ErrorResponse{Error: "method not allowed"})
	})

	h := handlers.NewWebhookApiHandler(webhookSvc, metricsRec, l)

	r.Route("/webhooks", func(r chi.Router) {
		r.Use(middlewares.RequireAPIKey(authSvc))
		r.Get("/", h.ListWebhooksApi)
		r.Post("/", h.CreateWebhookApi)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetWebhookApi)
			r.Patch("/", h.UpdateWebhookApi)
			r.Put("/", h.UpdateWebhookApi) // alias kept for existing clients
			r.Delete("/", h.DeleteWebhookApi)
		})
	})

	return r
}
