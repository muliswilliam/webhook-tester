package handlers

import (
	"encoding/json"
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
	"html/template"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"

	"github.com/gorilla/csrf"
	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/models"
)

type HomeHandler struct {
	Service      *service.WebhookService
	SessionStore *gormstore.Store
	Logger       *log.Logger
	Metrics      metrics.Recorder
	DB           *gorm.DB // TODO: Remove when sessions.GetLoggedInUser is refactored
}

func NewHomeHandler(
	svc *service.WebhookService,
	sessionStore *gormstore.Store,
	l *log.Logger,
	mr metrics.Recorder,
	db *gorm.DB) *HomeHandler {
	return &HomeHandler{
		Service:      svc,
		SessionStore: sessionStore,
		Logger:       l,
		Metrics:      mr,
		DB:           db,
	}
}

type HomePageData struct {
	CSRFField       template.HTML
	User            models.User
	Webhooks        []models.Webhook
	Webhook         models.Webhook
	ResponseHeaders string
	RequestsCount   uint
	Domain          string
	Year            int
}

var sessionIdName = "_webhook_tester_guest_session_id"

func createDefaultWebhook(svc *service.WebhookService, l *log.Logger) (string, error) {
	defaultWh := models.Webhook{
		ID:           utils.GenerateID(),
		Title:        "Default Webhook",
		ResponseCode: http.StatusOK,
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
	userID, _ := sessions.Authorize(r, h.SessionStore)

	// Get or create default webhook via cookie
	cookie, err := r.Cookie(sessionIdName)
	if err != nil && userID == 0 {
		defaultWhID, err := createDefaultWebhook(h.Service, h.Logger)
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
		wrr, err := h.Service.GetWebhookWithRequests(webhookID)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
			cookie.MaxAge = -1
			http.SetCookie(w, cookie)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		webhook = *wrr
		webhooks = append(webhooks, webhook)
	} else {
		// Load user's other webhooks if logged in
		webhooks, _ = h.Service.ListWebhooks(userID)
	}

	if address != "" {
		aw, err := h.Service.GetWebhook(address)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
		}
		if aw != nil {
			activeWebhook = *aw
		}
	} else if len(webhooks) > 0 {
		activeWebhook = webhooks[0]
	}

	var headersJSON = ""
	if activeWebhook.ResponseHeaders != nil {
		b, err := json.Marshal(activeWebhook.ResponseHeaders)
		if err != nil {
			log.Printf("error marshalling response headers: %v", err)
		} else {
			headersJSON = string(b)
		}
	}

	// RenderHtml the home page
	data := HomePageData{
		CSRFField:       csrf.TemplateField(r),
		User:            sessions.GetLoggedInUser(r, h.SessionStore, h.DB),
		Webhooks:        webhooks,
		Webhook:         activeWebhook,
		ResponseHeaders: headersJSON,
		RequestsCount:   uint(len(activeWebhook.Requests)),
		Domain:          os.Getenv("DOMAIN"),
		Year:            time.Now().Year(),
	}

	utils.RenderHtml(w, r, "home", data)
}
