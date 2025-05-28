package handlers

import (
	"encoding/json"
	"html/template"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/models"

	"github.com/gorilla/csrf"
)

type HomeHandler struct {
	webhookSvc service.WebhookService
	authSvc    service.AuthService
	Logger     *log.Logger
	Metrics    metrics.Recorder
}

func NewHomeHandler(
	webhookSvc service.WebhookService,
	authSvc service.AuthService,
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
	CSRFField       template.HTML
	User            *models.User
	Webhooks        []models.Webhook
	Webhook         models.Webhook
	ResponseHeaders string
	RequestsCount   uint
	Domain          string
	Year            int
}

func createDefaultWebhook(svc service.WebhookService, l *log.Logger) (string, error) {
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

func (h *HomeHandler) handleGuestSession(w http.ResponseWriter, r *http.Request) {
	// get guest webhook ID
	var guestWebhook *models.Webhook
	guestWebhookID, _ := h.authSvc.GetGuestSession(r)
	if guestWebhookID == "" {
		defaultWhID, err := createDefaultWebhook(h.webhookSvc, h.Logger)
		if err != nil {
			h.Logger.Printf("Error creating default webhook: %v", err)
			http.Error(w, "failed to create webhook", http.StatusInternalServerError)
			return
		}
		err = h.authSvc.CreateGuestSession(r, w, defaultWhID)
		if err != nil {
			h.Logger.Printf("Error creating default webhook cookie: %v", err)
			http.Error(w, "failed to create webhook cookie", http.StatusInternalServerError)

			return
		}
		guestWebhook, _ = h.webhookSvc.GetWebhookWithRequests(defaultWhID)
		h.Metrics.IncWebhooksCreated()
	} else {
		guestWebhook, _ = h.webhookSvc.GetWebhookWithRequests(guestWebhookID)
	}

	// RenderHtml the home page
	data := HomePageData{
		CSRFField: csrf.TemplateField(r),
		User:      &models.User{},
		Webhooks: []models.Webhook{
			*guestWebhook,
		},
		Webhook:         *guestWebhook,
		ResponseHeaders: "",
		RequestsCount:   0,
		Domain:          os.Getenv("DOMAIN"),
		Year:            time.Now().Year(),
	}
	utils.RenderHtml(w, r, "home", data)
}

func (h *HomeHandler) Home(w http.ResponseWriter, r *http.Request) {
	var webhooks []models.Webhook
	var activeWebhook models.Webhook
	var user *models.User
	address := r.URL.Query().Get("address")

	userID, _ := h.authSvc.Authorize(r)

	if userID == 0 {
		h.handleGuestSession(w, r)
		return
	}

	user, _ = h.authSvc.GetCurrentUser(r)
	webhooks, _ = h.webhookSvc.ListWebhooks(userID)

	if address != "" {
		aw, err := h.webhookSvc.GetWebhookWithRequests(address)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
		}
		activeWebhook = *aw
	} else {
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
		User:            user,
		Webhooks:        webhooks,
		Webhook:         activeWebhook,
		ResponseHeaders: headersJSON,
		RequestsCount:   uint(len(activeWebhook.Requests)),
		Domain:          os.Getenv("DOMAIN"),
		Year:            time.Now().Year(),
	}

	utils.RenderHtml(w, r, "home", data)
}
