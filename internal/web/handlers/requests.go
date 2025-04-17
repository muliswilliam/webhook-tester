package handlers

import (
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"
)

func GetRequest(w http.ResponseWriter, r *http.Request) {
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
		ID       string
		Year     int
		User     models.User
		Webhooks []models.Webhook
		Webhook  models.Webhook
		Request  models.WebhookRequest
	}{
		ID:       requestId,
		Webhooks: []models.Webhook{webhook},
		Webhook:  webhook,
		User:     sessions.GetLoggedInUser(r),
		Request:  request,
		Year:     time.Now().Year(),
	}

	utils.RenderHtml(w, "request", data)
}

func DeleteRequest(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")

	db.DB.Delete(&models.WebhookRequest{}, "id = ?", requestId)

	// if

	referer := r.Referer()
	if referer == "" {
		referer = "/" // fallback
	}

	// Redirect back to the referring page
	http.Redirect(w, r, referer, http.StatusFound)
}
