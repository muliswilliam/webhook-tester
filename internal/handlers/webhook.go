package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	webhookSvc    *service.WebhookService
	webhookReqSvc *service.WebhookRequestService
	authSvc       *service.AuthService
	logger        *log.Logger
	metrics       metrics.Recorder
}

func NewWebhookHandler(
	webhookSvc *service.WebhookService,
	webhookReqSvc *service.WebhookRequestService,
	authSvc *service.AuthService,
	logger *log.Logger,
	metrics metrics.Recorder) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc:    webhookSvc,
		webhookReqSvc: webhookReqSvc,
		authSvc:       authSvc,
		logger:        logger,
		metrics:       metrics,
	}
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.Authorize(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	err = r.ParseForm()
	if err != nil {
		h.logger.Printf("error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	contentType := r.FormValue("content_type")
	responseCode, _ := strconv.Atoi(r.FormValue("response_code"))
	if responseCode == 0 {
		responseCode = http.StatusOK
	}
	responseDelay, _ := strconv.Atoi(r.FormValue("response_delay")) // defaults to 0
	payload := r.FormValue("payload")
	notify := r.FormValue("notify_on_event") == "true"

	headersStr := r.FormValue("response_headers")
	var headers datatypes.JSONMap
	if headersStr != "" {
		err := json.Unmarshal([]byte(headersStr), &headers)
		if err != nil {
			log.Printf("error parsing json %s", err)
		}
	}

	webhookID := utils.GenerateID()
	wh := models.Webhook{
		ID:              webhookID,
		UserID:          int(userID),
		Title:           title,
		ContentType:     &contentType,
		ResponseCode:    responseCode,
		ResponseDelay:   uint(responseDelay),
		Payload:         &payload,
		ResponseHeaders: headers,
		NotifyOnEvent:   notify,
	}

	err = h.webhookSvc.CreateWebhook(&wh)
	if err != nil {
		h.logger.Printf("Error creating webhook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.metrics.IncWebhooksCreated()

	http.Redirect(w, r, fmt.Sprintf("/?address=%s", webhookID), http.StatusSeeOther)
}

func (h *WebhookHandler) DeleteRequests(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.Authorize(r)
	if err != nil {
		userID = 0 // anonymous/guest managing one of their own public webhooks
	}

	webhookID := chi.URLParam(r, "id")
	if webhookID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if _, err := h.webhookSvc.GetUserWebhook(webhookID, userID); err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err := h.webhookReqSvc.DeleteAll(webhookID); err != nil {
		h.logger.Printf("Error deleting webhook requests: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?address=%s", webhookID), http.StatusSeeOther)
}

func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.Authorize(r)
	if err != nil {
		userID = 0 // anonymous/guest managing one of their own public webhooks
	}

	webhookID := chi.URLParam(r, "id")
	if webhookID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := h.webhookSvc.DeleteWebhook(webhookID, userID); err != nil {
		h.logger.Printf("Error deleting webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	cleanupWebhookState(webhookID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *WebhookHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.Authorize(r)
	if err != nil {
		userID = 0 // anonymous/guest managing one of their own public webhooks
	}

	webhookID := chi.URLParam(r, "id")
	if webhookID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err = r.ParseForm()
	if err != nil {
		h.logger.Printf("error parsing form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	contentType := r.FormValue("content_type")
	responseCode, _ := strconv.Atoi(r.FormValue("response_code"))
	if responseCode == 0 {
		responseCode = http.StatusOK
	}
	responseDelay, _ := strconv.Atoi(r.FormValue("response_delay")) // defaults to 0
	payload := r.FormValue("payload")
	notify := r.FormValue("notify_on_event") == "true"

	headersStr := r.FormValue("response_headers")
	var headers datatypes.JSONMap
	if headersStr != "" {
		err := json.Unmarshal([]byte(headersStr), &headers)
		if err != nil {
			log.Printf("error parsing json %s", err)
		}
	}
	wh, err := h.webhookSvc.GetUserWebhook(webhookID, userID)
	if err != nil {
		h.logger.Printf("Error getting webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	wh.Title = title
	wh.ContentType = &contentType
	wh.ResponseCode = responseCode
	wh.ResponseDelay = uint(responseDelay)
	wh.NotifyOnEvent = notify
	wh.Payload = &payload
	wh.ResponseHeaders = headers

	err = h.webhookSvc.UpdateWebhook(wh)
	if err != nil {
		h.logger.Printf("Error updating webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/?address=%s", webhookID), http.StatusSeeOther)
}

// webhookEvent carries a captured request together with the request count
// computed atomically at insert time, so every SSE subscriber for a webhook
// sees the same count without re-querying the DB itself. CountErr is set when
// the count query failed; consumers still show the row but skip the
// count-dependent placeholder removal and counter patch.
type webhookEvent struct {
	Request  models.WebhookRequest
	Count    int64
	CountErr bool
}

var webhookStreams = make(map[string][]chan webhookEvent)
var mu sync.Mutex

// requestMus holds one mutex per webhook ID, so the CreateRequest+CountRequests
// atomicity below only serializes requests to the *same* webhook instead of
// forcing every webhook in the app through a single global lock around two DB
// round-trips.
var (
	requestMus   = make(map[string]*sync.Mutex)
	requestMusMu sync.Mutex
)

func requestMuFor(webhookID string) *sync.Mutex {
	requestMusMu.Lock()
	defer requestMusMu.Unlock()
	m, ok := requestMus[webhookID]
	if !ok {
		m = &sync.Mutex{}
		requestMus[webhookID] = m
	}
	return m
}

// cleanupWebhookState drops the per-webhook entries in requestMus and
// webhookStreams once a webhook is deleted, so a long-running server doesn't
// accumulate a mutex and a (by then empty) channel slice per deleted webhook
// forever.
func cleanupWebhookState(webhookID string) {
	requestMusMu.Lock()
	delete(requestMus, webhookID)
	requestMusMu.Unlock()

	mu.Lock()
	delete(webhookStreams, webhookID)
	mu.Unlock()
}

func (h *WebhookHandler) HandleWebhookRequest(w http.ResponseWriter, r *http.Request) {
	webhookID := strings.TrimPrefix(r.URL.Path, "/webhooks/")
	h.logger.Printf("Handling webhook request for %s", webhookID)
	webhook, err := h.webhookSvc.GetWebhook(webhookID)

	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// Read body
	body, _ := io.ReadAll(r.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("error closing body: %s", err)
		}
	}(r.Body)

	// Convert headers to a map[string]string
	headers := datatypes.JSONMap{}
	for k, v := range r.Header {
		headers[k] = strings.Join(v, ",")
	}

	query := datatypes.JSONMap{}
	for k, v := range r.URL.Query() {
		query[k] = strings.Join(v, ",")
	}

	wr := models.WebhookRequest{
		ID:         utils.GenerateID(),
		WebhookID:  webhookID,
		Method:     r.Method,
		Headers:    headers,
		Query:      query,
		Body:       string(body),
		ReceivedAt: time.Now().UTC(),
	}

	// CreateRequest, CountRequests and the broadcast below all run as one
	// atomic unit per webhook, and in that order, under whMu: otherwise two
	// requests arriving for the same brand-new webhook could both observe a
	// count > 1 (stranding the "first request" placeholder-removal check in
	// StreamWebhookEvents), or their events could reach subscribers out of
	// insertion order if the broadcast ran after an unguarded ResponseDelay
	// sleep.
	whMu := requestMuFor(webhookID)
	whMu.Lock()
	err = h.webhookSvc.CreateRequest(&wr)
	if err != nil {
		whMu.Unlock()
		h.logger.Printf("error creating webhook request: %s", err)
		utils.RenderJSON(w, http.StatusInternalServerError, nil)
		return
	}
	count, countErr := h.webhookSvc.CountRequests(webhookID)
	if countErr != nil {
		h.logger.Printf("error counting webhook requests: %s", countErr)
	}
	// The row is broadcast even on a count error - only the count-dependent
	// placeholder removal and counter patch are skipped downstream - so a
	// transient DB hiccup doesn't hide a captured request from the live view.
	event := webhookEvent{Request: wr, Count: count, CountErr: countErr != nil}
	mu.Lock()
	for _, ch := range webhookStreams[webhookID] {
		select {
		case ch <- event:
		default: // drop if blocked
		}
	}
	mu.Unlock()
	whMu.Unlock()

	h.metrics.IncWebhookRequest(webhookID)

	// Delay response
	if webhook.ResponseDelay > 0 {
		time.Sleep(time.Duration(webhook.ResponseDelay) * time.Millisecond)
	}

	// Set custom response headers if defined
	if webhook.ResponseHeaders != nil {
		for k, v := range webhook.ResponseHeaders {
			w.Header().Set(k, fmt.Sprintf("%v", v))
		}
	}

	// Return custom response
	if webhook.ContentType != nil {
		w.Header().Set("Content-Type", *webhook.ContentType)
	} else {
		// Default to application json if content type is not specified
		w.Header().Set("Content-Type", "application/json")
	}

	w.WriteHeader(webhook.ResponseCode)
	if webhook.Payload != nil {
		if _, err := w.Write([]byte(*webhook.Payload)); err != nil {
			h.logger.Printf("error writing payload: %s", err)
		}
	}
}

func (h *WebhookHandler) StreamWebhookEvents(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")

	// Computed once for the life of the connection: gorilla/csrf tokens are masked
	// per call but all validate against the same session cookie, so a single token
	// can safely be reused for every row patched over this SSE connection.
	csrfField := csrf.TemplateField(r)

	// Create a buffered channel for this client: a burst of requests would
	// otherwise drop live updates as soon as one consumer iteration (DB
	// count + template renders + SSE writes) fell behind an unbuffered send.
	eventChan := make(chan webhookEvent, 32)
	mu.Lock()
	webhookStreams[webhookID] = append(webhookStreams[webhookID], eventChan)
	mu.Unlock()

	// Deregister on every exit path (render/patch errors included), not just
	// the ctx.Done() path, so a broken connection doesn't leak this channel
	// in webhookStreams forever.
	defer func() {
		mu.Lock()
		subs := webhookStreams[webhookID]
		for i, sub := range subs {
			if sub == eventChan {
				webhookStreams[webhookID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		mu.Unlock()
	}()

	sse := datastar.NewSSE(w, r)

	// The "waiting"/"empty" placeholders only exist in the DOM until this
	// connection's first event, regardless of what count that event reports:
	// gating on count == 1 instead would permanently skip the removal for
	// this connection if the count query happened to fail on the webhook's
	// actual first request.
	placeholdersCleared := false

	for {
		select {
		case evt := <-eventChan:
			wr, count := evt.Request, evt.Count

			if evt.CountErr {
				h.logger.Printf("skipping count-dependent updates for %s: count was unavailable", wr.ID)
			}

			sidebarHTML, err := utils.RenderPartialToString("sidebar-request-row", wr)
			if err != nil {
				h.logger.Printf("error rendering sidebar request row: %s", err)
				continue
			}
			mainHTML, err := utils.RenderPartialToString("main-request-row", map[string]interface{}{
				"Request":   wr,
				"CSRFField": csrfField,
			})
			if err != nil {
				h.logger.Printf("error rendering main request row: %s", err)
				continue
			}

			if !placeholdersCleared {
				if err := sse.RemoveElementByID("request-log-waiting-" + wr.WebhookID); err != nil {
					h.logger.Printf("error removing waiting placeholder: %s", err)
					return
				}
				if err := sse.RemoveElementByID("request-log-empty-" + wr.WebhookID); err != nil {
					h.logger.Printf("error removing empty placeholder: %s", err)
					return
				}
				placeholdersCleared = true
			}
			if err := sse.PatchElements(sidebarHTML,
				datastar.WithSelectorID("request-log-"+wr.WebhookID),
				datastar.WithModePrepend(),
			); err != nil {
				h.logger.Printf("error patching sidebar request row: %s", err)
				return
			}
			if err := sse.PatchElements(mainHTML,
				datastar.WithSelectorID("request-log-list-"+wr.WebhookID),
				datastar.WithModePrepend(),
			); err != nil {
				h.logger.Printf("error patching main request row: %s", err)
				return
			}

			if evt.CountErr {
				continue
			}

			counterHTML, err := utils.RenderPartialToString("request-counter", map[string]interface{}{
				"WebhookID": wr.WebhookID,
				"Count":     count,
			})
			if err != nil {
				h.logger.Printf("error rendering request counter: %s", err)
				continue
			}
			if err := sse.PatchElements(counterHTML); err != nil {
				h.logger.Printf("error patching request counter: %s", err)
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}
