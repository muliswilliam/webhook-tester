package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/view"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/starfederation/datastar-go/datastar"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	webhookSvc    *service.WebhookService
	webhookReqSvc *service.WebhookRequestService
	authSvc       *service.AuthService
	forwarder     *service.Forwarder
	logger        *log.Logger
	metrics       metrics.Recorder
}

func NewWebhookHandler(
	webhookSvc *service.WebhookService,
	webhookReqSvc *service.WebhookRequestService,
	authSvc *service.AuthService,
	forwarder *service.Forwarder,
	logger *log.Logger,
	metrics metrics.Recorder) *WebhookHandler {
	return &WebhookHandler{
		webhookSvc:    webhookSvc,
		webhookReqSvc: webhookReqSvc,
		authSvc:       authSvc,
		forwarder:     forwarder,
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

	wh := models.Webhook{ID: utils.GenerateID(), UserID: int(userID)}
	if err := h.applyWebhookForm(r, &wh, true); err != nil {
		utils.SetFlashError(w, "Couldn't create the endpoint: "+err.Error())
		http.Redirect(w, r, backURL(r), http.StatusSeeOther)
		return
	}

	if err := h.webhookSvc.CreateWebhook(&wh); err != nil {
		h.logger.Printf("Error creating webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.metrics.IncWebhooksCreated()

	http.Redirect(w, r, webhookPageURL(wh.ID), http.StatusSeeOther)
}

// managedWebhookID returns the {id} URL param of a request that manages a
// webhook, along with the caller's user ID: 0 for a guest managing their own
// public webhook. It answers 400 and returns ok=false when the ID is missing.
func (h *WebhookHandler) managedWebhookID(w http.ResponseWriter, r *http.Request) (webhookID string, userID uint, ok bool) {
	userID, _ = h.authSvc.Authorize(r) // 0 for guests
	webhookID = chi.URLParam(r, "id")
	if webhookID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return "", 0, false
	}
	return webhookID, userID, true
}

// webhookPageURL is the workspace page showing the webhook.
func webhookPageURL(webhookID string) string {
	return "/?address=" + url.QueryEscape(webhookID)
}

func (h *WebhookHandler) DeleteRequests(w http.ResponseWriter, r *http.Request) {
	webhookID, userID, ok := h.managedWebhookID(w, r)
	if !ok {
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

	utils.SetFlashSuccess(w, "All requests cleared.")
	http.Redirect(w, r, webhookPageURL(webhookID), http.StatusSeeOther)
}

func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID, userID, ok := h.managedWebhookID(w, r)
	if !ok {
		return
	}

	if err := h.webhookSvc.DeleteWebhook(webhookID, userID); err != nil {
		h.logger.Printf("Error deleting webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	utils.SetFlashSuccess(w, "Endpoint deleted.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *WebhookHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID, userID, ok := h.managedWebhookID(w, r)
	if !ok {
		return
	}

	wh, err := h.webhookSvc.GetUserWebhook(webhookID, userID)
	if err != nil {
		h.logger.Printf("Error getting webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err := h.applyWebhookForm(r, wh, false); err != nil {
		utils.SetFlashError(w, "Changes not saved: "+err.Error())
		http.Redirect(w, r, webhookPageURL(webhookID), http.StatusSeeOther)
		return
	}

	if err := h.webhookSvc.UpdateWebhook(wh); err != nil {
		h.logger.Printf("Error updating webhook: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	utils.SetFlashSuccess(w, "Response settings saved.")
	http.Redirect(w, r, webhookPageURL(webhookID), http.StatusSeeOther)
}

// applyWebhookForm sets wh's title and response settings from the submitted
// create/edit form; isNew is set for the create form, and otherwise wh is the
// stored webhook. The result goes through the same Normalize and Validate as
// the API; invalid input is rejected without modifying wh.
func (h *WebhookHandler) applyWebhookForm(r *http.Request, wh *models.Webhook, isNew bool) error {
	if err := r.ParseForm(); err != nil {
		return errors.New("the form couldn't be read")
	}

	next := *wh
	next.Title = r.FormValue("title")
	next.ResponseCode = 0 // Normalize applies the default when left blank
	if v := r.FormValue("response_code"); v != "" {
		code, err := strconv.Atoi(v)
		if err != nil {
			return errors.New("response code must be a number")
		}
		next.ResponseCode = code
	}
	next.ResponseDelay = 0
	if v := r.FormValue("response_delay"); v != "" {
		delay, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("response delay must be a whole number between 0 and %d ms", models.MaxResponseDelay)
		}
		next.ResponseDelay = uint(delay)
	}
	next.ResponseHeaders = nil
	if v := r.FormValue("response_headers"); v != "" {
		if err := json.Unmarshal([]byte(v), &next.ResponseHeaders); err != nil {
			return errors.New("response headers must be a JSON object")
		}
	}
	contentType := r.FormValue("content_type")
	payload := r.FormValue("payload")
	next.ContentType = &contentType
	next.Payload = &payload
	next.NotifyOnEvent = r.FormValue("notify_on_event") == "true"
	// Only webhooks in an account forward, so the form offers the field to
	// owners alone; a guest webhook ignores it. A blank value clears it.
	if next.UserID != 0 && r.PostForm.Has("forward_url") {
		forwardURL := r.PostForm.Get("forward_url")
		next.ForwardURL = &forwardURL
	}

	next.Normalize()
	saved := wh
	if isNew {
		saved = nil
	}
	if err := h.webhookSvc.ValidateWebhook(r.Context(), &next, saved); err != nil {
		return err
	}
	*wh = next
	return nil
}

// backURL is the same-site page the request came from, or "/".
func backURL(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host != r.Host || ref.Path == "" {
		return "/"
	}
	return ref.RequestURI()
}

// HandleWebhookRequest captures a request sent to /webhooks/{id}, or to any
// subpath of it, and answers with the webhook's configured response. If the
// webhook forwards, the captured request is also relayed to its forward URL.
func (h *WebhookHandler) HandleWebhookRequest(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	path := capturedPath(r)
	webhook, err := h.webhookSvc.GetWebhook(webhookID)

	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "webhook not found", http.StatusNotFound)
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
			h.logger.Printf("error closing body: %s", err)
		}
	}(r.Body)

	wr := models.WebhookRequest{
		ID:        utils.GenerateID(),
		WebhookID: webhookID,
		Method:    r.Method,
		Path:      path,
		Headers:   models.CapturedValues(r.Header),
		Query:     models.CapturedValues(r.URL.Query()),
		RawQuery:  r.URL.RawQuery,
		Body:      string(body),
	}
	if err := h.webhookSvc.RecordRequest(webhook, &wr); err != nil {
		h.logger.Printf("error creating webhook request: %s", err)
		utils.RenderJSON(w, http.StatusInternalServerError, nil)
		return
	}

	h.metrics.IncWebhookRequest(webhookID)

	// Relay in the background: the provider's response never waits on the
	// forward target.
	if webhook.Forwards() {
		h.forwarder.ForwardAsync(*webhook, wr)
	}

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
		w.Header().Set("Content-Type", models.DefaultContentType)
	}

	// Validation keeps bad codes out, but WriteHeader panics on one, so
	// guard against rows saved before it existed.
	code := webhook.ResponseCode
	if models.ValidateResponseCode(code) != nil {
		h.logger.Printf("webhook %s has invalid response code %d", webhookID, code)
		code = http.StatusInternalServerError
	}
	w.WriteHeader(code)
	if webhook.Payload != nil {
		if _, err := w.Write([]byte(*webhook.Payload)); err != nil {
			h.logger.Printf("error writing payload: %s", err)
		}
	}
}

// capturedPath is the subpath a request to /webhooks/{id}/* was sent to, as
// sent: percent-encoded, e.g. "/orders/a%2Fb". It is "" for none, and "/"
// for a bare trailing slash.
func capturedPath(r *http.Request) string {
	sub := chi.URLParam(r, "*")
	if sub == "" && !strings.HasSuffix(r.URL.Path, "/") {
		return ""
	}
	path := "/" + sub
	// chi routes on RawPath when the path has escapes that its default
	// encoding wouldn't produce, which leaves sub escaped. Otherwise sub is
	// decoded, and its default encoding is what was sent.
	if r.URL.RawPath == "" {
		path = (&url.URL{Path: path}).EscapedPath()
	}
	return path
}

// StreamWebhookEvents streams a webhook's captured requests and their
// deliveries as Datastar element patches. A connection first replays the
// requests after its cursor - the Last-Event-ID of a reconnect, else the
// ?since= cursor the page was rendered with - with their deliveries, and
// then streams new ones live, so reconnects never lose requests. Every
// connection patches the sidebar's request list. The connection of the page's
// own webhook also patches its main panel: with ?active, the workspace's
// request list, counter and delivery badges and lists; with ?request=<id>,
// the request page's delivery list of that request.
//
// The client retries whenever the stream ends; a 204 tells it to stop.
func (h *WebhookHandler) StreamWebhookEvents(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	userID, _ := h.authSvc.Authorize(r) // 0 for guests

	webhook, err := h.webhookSvc.GetAccessibleWebhook(webhookID, userID)
	if err != nil {
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
		sse:           datastar.NewSSE(w, r),
		webhookID:     webhookID,
		forwardURL:    webhook.ActiveForwardURL(),
		mainPanel:     r.URL.Query().Has("active"),
		pageRequestID: r.URL.Query().Get("request"),
		csrfField:     csrf.TemplateField(r),
		replayed:      make(map[string]bool, len(missed)),
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
			switch evt.Kind {
			case service.EventRequestCaptured:
				if stream.replayed[evt.Request.ID] {
					continue
				}
				// The forward URL may have changed since the stream
				// started; new rows offer the replay targets current at
				// capture.
				stream.forwardURL = evt.ForwardURL
				if err := stream.send([]models.WebhookRequest{evt.Request}, evt.Count); err != nil {
					h.logger.Printf("error streaming request %s: %s", evt.Request.ID, err)
					return
				}
			case service.EventDeliveryRecorded:
				if err := stream.sendDeliveries(evt.Delivery.RequestID, evt.Deliveries); err != nil {
					h.logger.Printf("error streaming delivery %s: %s", evt.Delivery.ID, err)
					return
				}
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
	// ForwardURL is the webhook's forward URL if it forwards, which offers
	// the replay to it; "" otherwise.
	ForwardURL string
}

// requestCounterView is the data for the "request-counter" template.
type requestCounterView struct {
	WebhookID string
	Count     int64
}

// requestStream renders captured requests and their deliveries into one SSE
// connection.
type requestStream struct {
	sse        *datastar.ServerSentEventGenerator
	webhookID  string
	forwardURL string // see requestRowView.ForwardURL
	mainPanel  bool   // the page shows the webhook's request list
	// pageRequestID is the request whose page the stream is on, if any.
	pageRequestID string
	csrfField     template.HTML
	replayed      map[string]bool // IDs of requests sent from the backlog
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
			row := requestRowView{Request: wr, CSRFField: s.csrfField, IsNew: true, ForwardURL: s.forwardURL}
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

// sendDeliveries shows a request's deliveries, newest first, wherever the
// page shows the request: its delivery list is morphed into the given one
// and, in the request list, its badge shows the newest. Re-rendering the
// whole list keeps it in start order however deliveries finish, and picks
// up any the page missed. Pages that don't show the request get nothing.
func (s *requestStream) sendDeliveries(requestID string, deliveries []models.Delivery) error {
	if !s.mainPanel && s.pageRequestID != requestID {
		return nil
	}
	wr := models.WebhookRequest{ID: requestID, Deliveries: deliveries}
	if s.mainPanel {
		if err := s.patch("delivery-badge", view.NewDeliveryBadge(wr)); err != nil {
			return err
		}
	}
	return s.patch("delivery-list", wr)
}

func (s *requestStream) patch(tmpl string, data any, opts ...datastar.PatchElementOption) error {
	html, err := view.RenderRequestPartial(tmpl, data)
	if err != nil {
		return fmt.Errorf("rendering %s: %w", tmpl, err)
	}
	if err := s.sse.PatchElements(html, opts...); err != nil {
		return fmt.Errorf("patching %s: %w", tmpl, err)
	}
	return nil
}
