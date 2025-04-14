package handlers

import (
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web"

	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

var sessionIdName = "_webhook_test_session_id"

func Home(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionIdName) // or login cookie

	if err != nil {
		log.Printf("Cookie err: %v", err)

		// create a default webhook
		defaultWh := models.Webhook{
			ID:            utils.GenerateID(),
			Title:         "Default Webhook",
			ResponseCode:  http.StatusOK,
			ResponseDelay: 0,
		}
		err := sqlstore.InsertWebhook(defaultWh)
		if err != nil {
			log.Printf("Error inserting default webhook: %v", err)
		}

		cookie = &http.Cookie{
			Name:     sessionIdName,
			Value:    defaultWh.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,     // Set to true in production with HTTPS
			MaxAge:   86400 * 2, // 2 days
		}
		http.SetCookie(w, cookie)
	}

	var webhookId string
	address := r.URL.Query().Get("address")
	if address == "" {
		webhookId = cookie.Value
	} else {
		webhookId = address
	}

	var webhook models.Webhook
	err = db.DB.Model(&models.Webhook{}).Preload("Requests", func(db *gorm.DB) *gorm.DB {
		db = db.Order("received_at DESC")
		return db
	}).Find(&webhook, "id = ?", webhookId).Error

	if err != nil {
		log.Printf("failed to get webhook: %v", err)
		http.Error(w, "failed to get webhook", http.StatusInternalServerError)
	}

	data := struct {
		Webhook models.Webhook
		Domain  string
		Year    int
	}{
		Webhook: webhook,
		Domain:  os.Getenv("DOMAIN"),
		Year:    time.Now().Year(),
	}

	web.Render(w, "home", data)
}

func Request(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")
	address := r.URL.Query().Get("address")
	var webhook models.Webhook
	err := db.DB.Model(&models.Webhook{}).Preload("Requests", func(db *gorm.DB) *gorm.DB {
		db = db.Order("received_at DESC")
		return db
	}).Find(&webhook, "id = ?", address).Error

	if err != nil {
		log.Printf("failed to get webhook request: %v", err)
		http.Error(w, "failed to get webhook request", http.StatusInternalServerError)
	}

	var request models.WebhookRequest

	for _, r := range webhook.Requests {
		if r.ID == requestId {
			request = r
		}
	}

	data := struct {
		ID      string
		Year    int
		Webhook models.Webhook
		Request models.WebhookRequest
	}{
		ID:      requestId,
		Webhook: webhook,
		Request: request,
		Year:    time.Now().Year(),
	}

	web.Render(w, "request", data)
}
