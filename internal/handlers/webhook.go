package handlers

import (
	"database/sql"
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
	"gorm.io/datatypes"
)

type WebhookHandler struct {
	webhookSvc *service.WebhookService
	authSvc    *service.AuthService
	logger     *log.Logger
	metrics    metrics.Recorder
}

func NewWebhookHandler(
	webhookSvc *service.WebhookService,
	authSvc *service.AuthService,
	logger *log.Logger,
	metrics metrics.Recorder) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc: webhookSvc,
		authSvc:    authSvc,
		logger:     logger,
		metrics:    metrics,
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
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}

	webhookID := chi.URLParam(r, "id")

	if webhookID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}

	err = h.webhookSvc.DeleteWebhook(webhookID, userID)

	if err != nil {
		h.logger.Printf("Error deleting webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	http.Redirect(w, r, fmt.Sprintf("/?address=%s", webhookID), http.StatusSeeOther)
}

func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.Authorize(r)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}

	webhookID := chi.URLParam(r, "id")

	if webhookID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}

	err = h.webhookSvc.DeleteWebhook(webhookID, userID)

	if err != nil {
		h.logger.Printf("Error deleting webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *WebhookHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	userID, err := h.authSvc.Authorize(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
	}
	http.Redirect(w, r, fmt.Sprintf("/?address=%s", webhookID), http.StatusSeeOther)
}

var webhookStreams = make(map[string][]chan string)
var mu sync.Mutex

func (h *WebhookHandler) HandleWebhookRequest(w http.ResponseWriter, r *http.Request) {
	webhookID := strings.TrimPrefix(r.URL.Path, "/webhooks/")
	h.logger.Printf("Handling webhook request for %s", webhookID)
	webhook, err := h.webhookSvc.GetWebhook(webhookID)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
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

	err = h.webhookSvc.CreateRequest(&wr)
	if err != nil {
		h.logger.Printf("error creating webhook request: %s", err)
		utils.RenderJSON(w, http.StatusInternalServerError, nil)
		return
	}
	h.metrics.IncWebhookRequest(webhookID)

	// Delay response
	if webhook.ResponseDelay > 0 {
		time.Sleep(time.Duration(webhook.ResponseDelay) * time.Millisecond)
	}

	jsonData, _ := json.Marshal(wr)

	mu.Lock()
	for _, ch := range webhookStreams[webhookID] {
		select {
		case ch <- string(jsonData):
		default: // drop if blocked
		}
	}
	mu.Unlock()

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

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel for this client
	eventChan := make(chan string)
	mu.Lock()
	webhookStreams[webhookID] = append(webhookStreams[webhookID], eventChan)
	mu.Unlock()

	// Stream new events
	flusher, _ := w.(http.Flusher)
	for {
		select {
		case msg := <-eventChan:
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				h.logger.Printf("error writing data: %s", err)
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			mu.Lock()
			subs := webhookStreams[webhookID]
			for i, sub := range subs {
				if sub == eventChan {
					webhookStreams[webhookID] = append(subs[:i], subs[i+1:]...)
					break
				}
			}
			mu.Unlock()
			return
		}
	}
}
