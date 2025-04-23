package handlers

import (
	"github.com/gorilla/csrf"
	"gorm.io/gorm"
	"html/template"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"

	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/models"
)

type HomePageData struct {
	CSRFField     template.HTML
	User          models.User
	Webhooks      []models.Webhook
	Webhook       models.Webhook
	RequestsCount uint
	Domain        string
	Year          int
}

var sessionIdName = "_webhook_tester_guest_session_id"

func createDefaultWebhook(db *gorm.DB) (string, error) {
	defaultWh := models.Webhook{
		ID:           utils.GenerateID(),
		Title:        "Default Webhook",
		ResponseCode: http.StatusOK,
	}

	err := sqlstore.InsertWebhook(db, defaultWh)
	if err != nil {
		log.Printf("Error inserting default webhook: %v", err)
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

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	userID, err := sessions.Authorize(r, h.SessionStore)
	if err != nil {
		log.Println("user not logged in")
	}

	// Get or create default webhook via cookie
	cookie, err := r.Cookie(sessionIdName)
	if err != nil && userID == 0 {
		log.Printf("Cookie err: %v", err)
		defaultWhID, err := createDefaultWebhook(h.DB)
		if err != nil {
			log.Printf("Error creating default webhook: %v", err)
			http.Error(w, "failed to create webhook", http.StatusInternalServerError)
			return
		}
		cookie = createDefaultWebhookCookie(defaultWhID, w)
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
		webhook, err = sqlstore.GetWebhookWithRequests(webhookID, h.DB)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
			http.Error(w, "failed to get webhook", http.StatusInternalServerError)
			return
		}
		webhooks = append(webhooks, webhook)
	} else {
		// Load user's other webhooks if logged in
		webhooks = sqlstore.GetUserWebhooks(userID, h.DB)
	}

	if address != "" {
		activeWebhook, err = sqlstore.GetWebhookWithRequests(address, h.DB)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
		}
	} else if len(webhooks) > 0 {
		activeWebhook = webhooks[0]
	}

	// RenderHtml the home page
	data := HomePageData{
		CSRFField:     csrf.TemplateField(r),
		User:          sessions.GetLoggedInUser(r, h.SessionStore, h.DB),
		Webhooks:      webhooks,
		Webhook:       activeWebhook,
		RequestsCount: uint(len(activeWebhook.Requests)),
		Domain:        os.Getenv("DOMAIN"),
		Year:          time.Now().Year(),
	}

	utils.RenderHtml(w, r, "home", data)
}
