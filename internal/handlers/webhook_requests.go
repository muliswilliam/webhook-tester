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
	forwarder      *service.Forwarder
	metrics        *metrics.Recorder
	logger         *log.Logger
	webhookService *service.WebhookService
}

// NewWebhookRequestHandler creates a new handler.
func NewWebhookRequestHandler(
	reqSvc *service.WebhookRequestService,
	authSvc *service.AuthService,
	webhookSvc *service.WebhookService,
	forwarder *service.Forwarder,
	metricsRec *metrics.Recorder,
	logger *log.Logger,
) *WebhookRequestHandler {
	return &WebhookRequestHandler{
		reqService:     reqSvc,
		webhookService: webhookSvc,
		forwarder:      forwarder,
		metrics:        metricsRec,
		logger:         logger,
		authSvc:        authSvc,
	}
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

// accessibleRequest loads a captured request and its webhook, provided the
// caller may access that webhook.
func (h *WebhookRequestHandler) accessibleRequest(r *http.Request, id string) (*models.WebhookRequest, *models.Webhook, error) {
	wr, err := h.reqService.Get(id)
	if err != nil {
		return nil, nil, err
	}
	userID, _ := h.authSvc.Authorize(r) // 0 for guests
	wh, err := h.webhookService.GetAccessibleWebhook(wr.WebhookID, userID)
	if err != nil {
		return nil, nil, err
	}
	return wr, wh, nil
}

func (h *WebhookRequestHandler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")

	wr, _, err := h.accessibleRequest(r, requestId)
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

// Replay targets, chosen by the replay form's "target" field.
const (
	// ReplayTargetEndpoint re-sends the request to its Webhook Tester
	// endpoint, capturing a copy. It is the default.
	ReplayTargetEndpoint = "endpoint"
	// ReplayTargetForward relays the request to its webhook's forward URL,
	// recording a delivery on the original request.
	ReplayTargetForward = "forward"
)

// ReplayRequest re-sends a captured request, to its endpoint or to its
// webhook's forward URL, and flashes the outcome.
func (h *WebhookRequestHandler) ReplayRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	reqEvent, wh, err := h.accessibleRequest(r, id)
	if err != nil {
		h.logger.Printf("replay: request %s not found: %v", id, err)
		view.RenderNotFound(w, r)
		return
	}

	switch target := r.FormValue("target"); target {
	case "", ReplayTargetEndpoint:
		h.replayToEndpoint(w, r, reqEvent)
	case ReplayTargetForward:
		h.replayToForwardURL(w, r, wh, reqEvent)
	default:
		http.Error(w, fmt.Sprintf("unknown replay target %q", target), http.StatusBadRequest)
	}
}

// replayToForwardURL forwards the request synchronously, recording a replay
// delivery on it rather than capturing a copy.
func (h *WebhookRequestHandler) replayToForwardURL(w http.ResponseWriter, r *http.Request, wh *models.Webhook, reqEvent *models.WebhookRequest) {
	defer http.Redirect(w, r, backURL(r), http.StatusSeeOther)
	if !wh.Forwards() {
		if wh.UserID == 0 {
			utils.SetFlashError(w, "Forwarding is only available for endpoints in an account.")
		} else {
			utils.SetFlashError(w, "This endpoint has no forward URL. Set one in its settings first.")
		}
		return
	}

	d := h.forwarder.Forward(r.Context(), *wh, *reqEvent, models.DeliveryTriggerReplay)
	if d.Outcome.Answered() {
		utils.SetFlashSuccess(w, fmt.Sprintf("Forwarded. Your server answered %s.", d.StatusLine()))
		return
	}
	reason := "the delivery failed"
	if d.Error != nil {
		reason = strings.TrimSuffix(*d.Error, ".")
	}
	utils.SetFlashError(w, fmt.Sprintf("Forward failed: %s.", reason))
}

// replayToEndpoint re-sends the request to its Webhook Tester endpoint,
// which captures it as a new request.
func (h *WebhookRequestHandler) replayToEndpoint(w http.ResponseWriter, r *http.Request, reqEvent *models.WebhookRequest) {
	domain := os.Getenv("DOMAIN")
	endpoint, err := url.JoinPath(domain, "webhooks", reqEvent.WebhookID)
	if err == nil {
		endpoint, err = reqEvent.URLAt(endpoint)
	}
	if err != nil {
		h.logger.Printf("replay: invalid target URL: %v", err)
		http.Error(w, "could not construct replay URL", http.StatusInternalServerError)
		return
	}

	bodyReader := strings.NewReader(reqEvent.Body)
	outReq, err := http.NewRequest(reqEvent.Method, endpoint, bodyReader)
	if err != nil {
		h.logger.Printf("replay: error creating HTTP request: %v", err)
		http.Error(w, "error creating request", http.StatusInternalServerError)
		return
	}
	for k, values := range reqEvent.HeaderValues() {
		for _, v := range values {
			outReq.Header.Add(k, v)
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
