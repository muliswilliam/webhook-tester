package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
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
		ID:        utils.GenerateID(),
		WebhookID: webhookID,
		Method:    r.Method,
		Headers:   headers,
		Query:     query,
		Body:      string(body),
	}
	if err := h.webhookSvc.RecordRequest(&wr); err != nil {
		h.logger.Printf("error creating webhook request: %s", err)
		utils.RenderJSON(w, http.StatusInternalServerError, nil)
		return
	}

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

// StreamWebhookEvents streams a webhook's captured requests as Datastar
// element patches. A connection first replays the requests after its cursor -
// the Last-Event-ID of a reconnect, else the ?since= cursor the page was
// rendered with - and then streams new ones live, so reconnects never lose
// requests. The ?active flag marks the connection of the webhook shown in the
// main panel, which also patches the main request list and counter.
//
// The client retries whenever the stream ends; a 204 tells it to stop.
func (h *WebhookHandler) StreamWebhookEvents(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	userID, _ := h.authSvc.Authorize(r) // 0 for guests

	if _, err := h.webhookSvc.GetAccessibleWebhook(webhookID, userID); err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	rawCursor := r.Header.Get("Last-Event-ID")
	if rawCursor == "" {
		rawCursor = r.URL.Query().Get("since")
	}
	cursor, err := models.ParseRequestCursor(rawCursor)
	if err != nil {
		h.logger.Printf("rejecting stream for %s: %s", webhookID, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Subscribe before querying the backlog so nothing captured in between
	// is missed; live events already replayed are skipped.
	sub := h.webhookSvc.Subscribe(webhookID)
	defer sub.Close()

	missed, err := h.webhookSvc.GetRequestsAfter(webhookID, cursor)
	if err != nil {
		h.logger.Printf("error loading missed requests for %s: %s", webhookID, err)
		return
	}

	stream := &requestStream{
		sse:       datastar.NewSSE(w, r),
		webhookID: webhookID,
		mainPanel: r.URL.Query().Has("active"),
		csrfField: csrf.TemplateField(r),
		replayed:  make(map[string]bool, len(missed)),
	}

	for _, wr := range missed {
		stream.replayed[wr.ID] = true
	}
	if len(missed) > 0 {
		var count *int64
		if n, err := h.webhookSvc.CountRequests(webhookID); err == nil {
			count = &n
		}
		if err := stream.send(missed, count); err != nil {
			h.logger.Printf("error streaming missed requests for %s: %s", webhookID, err)
			return
		}
	}

	for {
		select {
		case evt, ok := <-sub.Events:
			if !ok {
				// Evicted for falling behind, or the webhook was deleted.
				// Either way the client reconnects and resumes from its cursor.
				h.logger.Printf("stream for %s closed by broker", webhookID)
				return
			}
			if stream.replayed[evt.Request.ID] {
				continue
			}
			if err := stream.send([]models.WebhookRequest{evt.Request}, evt.Count); err != nil {
				h.logger.Printf("error streaming request %s: %s", evt.Request.ID, err)
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// requestRowView is the data for the "main-request-row" template.
type requestRowView struct {
	Request   models.WebhookRequest
	CSRFField template.HTML
	IsNew     bool
}

// requestCounterView is the data for the "request-counter" template.
type requestCounterView struct {
	WebhookID string
	Count     int64
}

// requestStream renders captured requests into one SSE connection.
type requestStream struct {
	sse       *datastar.ServerSentEventGenerator
	webhookID string
	mainPanel bool
	csrfField template.HTML
	replayed  map[string]bool // IDs of requests sent from the backlog
}

// send prepends each request, oldest first, and then patches the counter if
// count is known. Each request's last patch carries the request's cursor as
// the SSE event ID, which the client echoes back as Last-Event-ID on
// reconnect.
func (s *requestStream) send(requests []models.WebhookRequest, count *int64) error {
	for _, wr := range requests {
		id := datastar.WithPatchElementsEventID(models.CursorAt(wr).String())

		sidebarOpts := []datastar.PatchElementOption{
			datastar.WithSelectorID("request-log-" + s.webhookID),
			datastar.WithModePrepend(),
		}
		if !s.mainPanel {
			sidebarOpts = append(sidebarOpts, id)
		}
		if err := s.patch("sidebar-request-row", wr, sidebarOpts...); err != nil {
			return err
		}

		if s.mainPanel {
			row := requestRowView{Request: wr, CSRFField: s.csrfField, IsNew: true}
			if err := s.patch("main-request-row", row,
				datastar.WithSelectorID("request-log-list-"+s.webhookID),
				datastar.WithModePrepend(),
				id,
			); err != nil {
				return err
			}
		}
	}

	if s.mainPanel && count != nil {
		return s.patch("request-counter", requestCounterView{WebhookID: s.webhookID, Count: *count})
	}
	return nil
}

func (s *requestStream) patch(tmpl string, data any, opts ...datastar.PatchElementOption) error {
	html, err := utils.RenderPartialToString(tmpl, data)
	if err != nil {
		return fmt.Errorf("rendering %s: %w", tmpl, err)
	}
	if err := s.sse.PatchElements(html, opts...); err != nil {
		return fmt.Errorf("patching %s: %w", tmpl, err)
	}
	return nil
}
