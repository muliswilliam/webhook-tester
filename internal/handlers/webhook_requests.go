package handlers

import (
	"fmt"
	"github.com/gorilla/csrf"
	"github.com/wader/gormstore/v2"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

	"github.com/go-chi/chi/v5"
)

type WebhookRequestHandler struct {
	reqService     service.WebhookRequestService
	authSvc        service.AuthService
	metrics        metrics.Recorder
	logger         *log.Logger
	webhookService service.WebhookService
	sessionStore   gormstore.Store
}

// NewWebhookRequestHandler creates a new handler.
func NewWebhookRequestHandler(
	reqSvc service.WebhookRequestService,
	authSvc service.AuthService,
	webhookSvc service.WebhookService,
	metricsRec metrics.Recorder,
	logger *log.Logger,
) *WebhookRequestHandler {
	return &WebhookRequestHandler{reqService: reqSvc, webhookService: webhookSvc, metrics: metricsRec, logger: logger, authSvc: authSvc}
}

func (h *WebhookRequestHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	// 1) Extract path & query params
	reqID := chi.URLParam(r, "id")
	address := r.URL.Query().Get("address")

	// 2) Load the webhook and its requests via the service
	wh, err := h.webhookService.GetWebhookWithRequests(address)
	if err != nil {
		h.logger.Printf("failed to load webhook %s: %v", address, err)
		http.Error(w, "could not load webhook", http.StatusInternalServerError)
		return
	}

	// 3) Load the individual request via the service
	reqEvent, err := h.reqService.Get(reqID)
	if err != nil {
		h.logger.Printf("request %s not found: %v", reqID, err)
		http.NotFound(w, r)
		return
	}

	// 4) Build the sidebar list: either the user’s own webhooks, or just the one
	user, _ := h.authSvc.GetCurrentUser(r)
	var list []models.Webhook
	if user.ID != 0 {
		if list, err = h.webhookService.ListWebhooks(user.ID); err != nil {
			h.logger.Printf("failed to list webhooks for user %d: %v", user.ID, err)
			http.Error(w, "could not load your webhooks", http.StatusInternalServerError)
			return
		}
	} else {
		list = []models.Webhook{*wh}
	}

	// 5) Render
	data := struct {
		ID        string
		Year      int
		User      models.User
		Webhooks  []models.Webhook
		Webhook   *models.Webhook
		Request   *models.WebhookRequest
		CSRFField template.HTML
	}{
		ID:        reqID,
		Year:      time.Now().Year(),
		User:      *user,
		Webhooks:  list,
		Webhook:   wh,
		Request:   reqEvent,
		CSRFField: csrf.TemplateField(r),
	}

	utils.RenderHtml(w, r, "request", data)
}

func (h *WebhookRequestHandler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")

	err := h.reqService.Delete(requestId)

	if err != nil {
		h.logger.Printf("failed to delete webhook %s: %v", requestId, err)
		http.Error(w, "could not delete webhook", http.StatusInternalServerError)
		return
	}

	referer := r.Referer()
	if referer == "" {
		referer = "/" // fallback
	}

	// Redirect back to the referring page
	http.Redirect(w, r, referer, http.StatusFound)
}

// ReplayRequest re‐sends a stored webhook request via your services.
func (h *WebhookRequestHandler) ReplayRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	reqEvent, err := h.reqService.Get(id)
	if err != nil {
		h.logger.Printf("replay: request %s not found: %v", id, err)
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	domain := os.Getenv("DOMAIN")
	target, err := url.JoinPath(domain, "webhooks", reqEvent.WebhookID)
	if err != nil {
		h.logger.Printf("replay: invalid target URL: %v", err)
		http.Error(w, "could not construct replay URL", http.StatusInternalServerError)
		return
	}

	parsed, _ := url.Parse(target)
	q := parsed.Query()
	for k, v := range reqEvent.Query {
		if s, ok := v.(string); ok {
			q.Set(k, s)
		}
	}
	parsed.RawQuery = q.Encode()

	bodyReader := strings.NewReader(reqEvent.Body)
	outReq, err := http.NewRequest(reqEvent.Method, parsed.String(), bodyReader)
	if err != nil {
		h.logger.Printf("replay: error creating HTTP request: %v", err)
		http.Error(w, "error creating request", http.StatusInternalServerError)
		return
	}
	for k, v := range reqEvent.Headers {
		if s, ok := v.(string); ok {
			outReq.Header.Set(k, s)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(outReq)
	if err != nil {
		h.logger.Printf("replay: error sending request: %v", err)
		http.Error(w, "error sending request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 6) Redirect back to the request details page
	redirectURL := fmt.Sprintf("/requests/%s?address=%s", reqEvent.ID, reqEvent.WebhookID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
