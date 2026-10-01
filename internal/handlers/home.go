package handlers

import (
	"html/template"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/view"

	"github.com/gorilla/csrf"
	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/models"
)

type HomeHandler struct {
	webhookSvc *service.WebhookService
	authSvc    *service.AuthService
	Logger     *log.Logger
	Metrics    metrics.Recorder
}

func NewHomeHandler(
	webhookSvc *service.WebhookService,
	authSvc *service.AuthService,
	l *log.Logger,
	mr metrics.Recorder,
) *HomeHandler {
	return &HomeHandler{
		webhookSvc: webhookSvc,
		authSvc:    authSvc,
		Logger:     l,
		Metrics:    mr,
	}
}

type HomePageData struct {
	CSRFField template.HTML
	User      models.User
	Webhooks  []models.Webhook
	Webhook   models.Webhook
	// CanManage is set when the viewer may change, clear or delete Webhook
	// and its requests; see models.Webhook.ManagedBy.
	CanManage      bool
	RequestRows    []requestRowView
	RequestCounter requestCounterView
	Domain         string
	Year           int
}

var sessionIdName = "_webhook_tester_guest_session_id"

func createDefaultWebhook(svc *service.WebhookService, l *log.Logger) (string, error) {
	contentType := models.DefaultContentType
	payload := `{"message":"ok"}`
	defaultWh := models.Webhook{
		ID:           utils.GenerateID(),
		Title:        "Default Webhook",
		ResponseCode: http.StatusOK,
		ContentType:  &contentType,
		Payload:      &payload,
	}

	err := svc.CreateWebhook(&defaultWh)
	if err != nil {
		l.Printf("Error inserting default webhook: %v", err)
		return "", err
	}

	return defaultWh.ID, nil
}

func createDefaultWebhookCookie(webhookID string, w http.ResponseWriter) *http.Cookie {
	cookie := &http.Cookie{
		Name:     sessionIdName,
		Value:    webhookID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,     // Set to true in production
		MaxAge:   86400 * 2, // 2 days
	}
	http.SetCookie(w, cookie)
	return cookie
}

func (h *HomeHandler) Home(w http.ResponseWriter, r *http.Request) {
	userID, _ := h.authSvc.Authorize(r)

	// Get or create default webhook via cookie
	cookie, err := r.Cookie(sessionIdName)
	if err != nil && userID == 0 {
		defaultWhID, err := createDefaultWebhook(h.webhookSvc, h.Logger)
		if err != nil {
			h.Logger.Printf("Error creating default webhook: %v", err)
			http.Error(w, "failed to create webhook", http.StatusInternalServerError)
			return
		}
		cookie = createDefaultWebhookCookie(defaultWhID, w)
		h.Metrics.IncWebhooksCreated()
	}
	var webhooks []models.Webhook
	var webhook models.Webhook
	var activeWebhook models.Webhook

	// Determine active webhook ID
	address := r.URL.Query().Get("address")
	var webhookID = address
	if webhookID == "" && cookie != nil {
		webhookID = cookie.Value
	}

	if webhookID != "" && userID == 0 {
		wrr, err := h.webhookSvc.GetAccessibleWebhookWithRequests(webhookID, userID)
		if err != nil {
			h.Logger.Printf("failed to get webhook: %v", err)
			// Only a dead workspace cookie is cleared; a bad ?address= link
			// must not cost the guest their own workspace.
			if cookie != nil && cookie.Value == webhookID {
				cookie.MaxAge = -1
				http.SetCookie(w, cookie)
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		webhook = *wrr
		webhooks = append(webhooks, webhook)
	} else {
		// Load user's other webhooks if logged in
		webhooks, _ = h.webhookSvc.ListWebhooks(userID)
	}

	if address != "" {
		aw, err := h.webhookSvc.GetAccessibleWebhookWithRequests(address, userID)
		if err != nil {
			h.Logger.Printf("failed to get webhook: %v", err)
		} else {
			activeWebhook = *aw
		}
	} else if len(webhooks) > 0 {
		activeWebhook = webhooks[0]
	}

	user, _ := h.authSvc.GetCurrentUser(r)
	if user == nil {
		user = &models.User{}
	}

	csrfField := csrf.TemplateField(r)
	forwardURL := activeWebhook.ActiveForwardURL()
	canManage := activeWebhook.ManagedBy(userID)
	rows := make([]requestRowView, len(activeWebhook.Requests))
	for i, wr := range activeWebhook.Requests {
		rows[i] = requestRowView{Request: wr, CSRFField: csrfField, ForwardURL: forwardURL, CanManage: canManage}
	}

	// RenderHTML the home page
	data := HomePageData{
		CSRFField:      csrfField,
		User:           *user,
		Webhooks:       webhooks,
		Webhook:        activeWebhook,
		CanManage:      canManage,
		RequestRows:    rows,
		RequestCounter: requestCounterView{WebhookID: activeWebhook.ID, Count: int64(len(activeWebhook.Requests))},
		Domain:         os.Getenv("DOMAIN"),
		Year:           time.Now().Year(),
	}

	view.RenderHTML(w, r, "home", data)
}
