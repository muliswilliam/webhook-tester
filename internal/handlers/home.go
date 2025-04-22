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

func createDefaultWebhookCookie(db *gorm.DB, w http.ResponseWriter) *http.Cookie {
	defaultWh := models.Webhook{
		ID:            utils.GenerateID(),
		Title:         "Default Webhook",
		ResponseCode:  http.StatusOK,
		ResponseDelay: 0,
	}
	if err := sqlstore.InsertWebhook(db, defaultWh); err != nil {
		log.Printf("Error inserting default webhook: %v", err)
	}

	cookie := &http.Cookie{
		Name:     sessionIdName,
		Value:    defaultWh.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,     // Set to true in production
		MaxAge:   86400 * 2, // 2 days
	}
	http.SetCookie(w, cookie)
	return cookie
}

func fetchWebhookWithRequests(id string, db *gorm.DB) (models.Webhook, error) {
	var webhook models.Webhook
	err := db.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC")
	}).First(&webhook, "id = ?", id).Error
	return webhook, err
}

func fetchUserWebhooks(userID interface{}, db *gorm.DB) []models.Webhook {
	var webhooks []models.Webhook
	err := db.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC").Limit(1000)
	}).
		Where("user_id = ?", userID).Find(&webhooks).
		Order("created_at DESC").Error

	if err != nil {
		log.Printf("Error loading user webhooks: %v", err)
	}
	return webhooks
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
		cookie = createDefaultWebhookCookie(h.DB, w)
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
		webhook, err = fetchWebhookWithRequests(webhookID, h.DB)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
			http.Error(w, "failed to get webhook", http.StatusInternalServerError)
			return
		}
		webhooks = append(webhooks, webhook)
	} else {
		// Load user's other webhooks if logged in
		webhooks = fetchUserWebhooks(userID, h.DB)
	}

	if address != "" {
		activeWebhook, err = fetchWebhookWithRequests(address, h.DB)
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
