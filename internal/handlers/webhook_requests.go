package handlers

import (
	"errors"
	"fmt"
	"github.com/gorilla/csrf"
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
	"webhook-tester/internal/web/view"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// replayClient re-sends captured requests.
var replayClient = &http.Client{
	// Longer than the maximum response delay, so a slow endpoint still
	// completes.
	Timeout: models.MaxResponseDelay*time.Millisecond + 10*time.Second,
	Transport: func() http.RoundTripper {
		t := http.DefaultTransport.(*http.Transport).Clone()
		// Otherwise the client adds an Accept-Encoding header the original
		// request didn't have.
		t.DisableCompression = true
		return t
	}(),
}

type WebhookRequestHandler struct {
	reqService     *service.WebhookRequestService
	authSvc        *service.AuthService
	metrics        *metrics.Recorder
	logger         *log.Logger
	webhookService *service.WebhookService
}

// NewWebhookRequestHandler creates a new handler.
func NewWebhookRequestHandler(
	reqSvc *service.WebhookRequestService,
	authSvc *service.AuthService,
	webhookSvc *service.WebhookService,
	metricsRec *metrics.Recorder,
	logger *log.Logger,
) *WebhookRequestHandler {
	return &WebhookRequestHandler{reqService: reqSvc, webhookService: webhookSvc, metrics: metricsRec, logger: logger, authSvc: authSvc}
}

func (h *WebhookRequestHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	// 1) Extract path & query params
	reqID := chi.URLParam(r, "id")
	address := r.URL.Query().Get("address")

	userID, _ := h.authSvc.Authorize(r) // 0 for guests

	// 2) Load the webhook and its requests via the service
	wh, err := h.webhookService.GetAccessibleWebhookWithRequests(address, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		view.RenderNotFound(w, r)
		return
	}
	if err != nil {
		h.logger.Printf("failed to load webhook %s: %v", address, err)
		http.Error(w, "could not load webhook", http.StatusInternalServerError)
		return
	}

	// 3) Load the individual request via the service
	reqEvent, err := h.reqService.Get(reqID)
	if err != nil || reqEvent.WebhookID != wh.ID {
		h.logger.Printf("request %s not found: %v", reqID, err)
		view.RenderNotFound(w, r)
		return
	}

	// 4) Build the sidebar list: either the user’s own webhooks, or just the one
	user, _ := h.authSvc.GetCurrentUser(r)
	if user == nil {
		user = &models.User{}
	}
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

	view.RenderHTML(w, r, "request", data)
}

// accessibleRequest loads a captured request, provided the caller may access
// the webhook it belongs to.
func (h *WebhookRequestHandler) accessibleRequest(r *http.Request, id string) (*models.WebhookRequest, error) {
	wr, err := h.reqService.Get(id)
	if err != nil {
		return nil, err
	}
	userID, _ := h.authSvc.Authorize(r) // 0 for guests
	if _, err := h.webhookService.GetAccessibleWebhook(wr.WebhookID, userID); err != nil {
		return nil, err
	}
	return wr, nil
}

func (h *WebhookRequestHandler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")

	wr, err := h.accessibleRequest(r, requestId)
	if err != nil {
		h.logger.Printf("delete: request %s not accessible: %v", requestId, err)
		view.RenderNotFound(w, r)
		return
	}

	if err := h.reqService.Delete(requestId); err != nil {
		h.logger.Printf("failed to delete webhook %s: %v", requestId, err)
		http.Error(w, "could not delete webhook", http.StatusInternalServerError)
		return
	}

	// Back to the webhook's page: the referer may be the deleted request's
	// own detail page.
	utils.SetFlashSuccess(w, "Request deleted.")
	http.Redirect(w, r, "/?address="+url.QueryEscape(wr.WebhookID), http.StatusSeeOther)
}

// ReplayRequest re‐sends a stored webhook request via your services.
func (h *WebhookRequestHandler) ReplayRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	reqEvent, err := h.accessibleRequest(r, id)
	if err != nil {
		h.logger.Printf("replay: request %s not found: %v", id, err)
		view.RenderNotFound(w, r)
		return
	}

	domain := os.Getenv("DOMAIN")
	target, err := url.JoinPath(domain, "webhooks", reqEvent.WebhookID, reqEvent.Path)
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

	resp, err := replayClient.Do(outReq)
	if err != nil {
		h.logger.Printf("replay: error sending request: %v", err)
		utils.SetFlashError(w, "Replay failed: the request couldn't be sent.")
		http.Redirect(w, r, backURL(r), http.StatusSeeOther)
		return
	}
	_ = resp.Body.Close() // only the status is used

	// Stay on the page the replay was started from; the replayed copy
	// streams into its request log.
	utils.SetFlashSuccess(w, fmt.Sprintf("Request replayed. The endpoint answered %s.", resp.Status))
	http.Redirect(w, r, backURL(r), http.StatusSeeOther)
}
