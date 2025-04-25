package handlers

import (
	"bytes"
	"net/http"
	"net/url"
	"os"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")
	address := r.URL.Query().Get("address")
	var webhook models.Webhook
	err := h.DB.Model(&models.Webhook{}).Preload("Requests", func(db *gorm.DB) *gorm.DB {
		db = db.Order("received_at DESC")
		return db
	}).Find(&webhook, "id = ?", address).Error

	if err != nil {
		h.Logger.Printf("failed to get webhook request: %v", err)
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
		User:     sessions.GetLoggedInUser(r, h.SessionStore, h.DB),
		Request:  request,
		Year:     time.Now().Year(),
	}

	utils.RenderHtml(w, r, "request", data)
}

func (h *Handler) DeleteRequest(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")

	h.DB.Delete(&models.WebhookRequest{}, "id = ?", requestId)

	// if

	referer := r.Referer()
	if referer == "" {
		referer = "/" // fallback
	}

	// Redirect back to the referring page
	http.Redirect(w, r, referer, http.StatusFound)
}

func (h *Handler) ReplayRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// target := r.FormValue("target")

	var req models.WebhookRequest
	if err := h.DB.First(&req, " id = ?", id).Error; err != nil {
		http.Error(w, "request not found", http.StatusNotFound)
	}

	// prepare req
	target, err := url.JoinPath(os.Getenv("DOMAIN"), "webhooks", req.WebhookID)
	if err != nil {
		http.Error(w, "error creating request url", http.StatusInternalServerError)
		return
	}

	// query params
	parsedUrl, err := url.Parse(target)
	for k, v := range req.Query {
		parsedUrl.Query().Set(k, v.(string))
	}
	parsedUrl.RawQuery = parsedUrl.Query().Encode()

	reader := bytes.NewBufferString(req.Body)
	outReq, err := http.NewRequest(req.Method, parsedUrl.String(), reader)
	if err != nil {
		http.Error(w, "error creating request url", http.StatusInternalServerError)
		return
	}

	// add headers
	for k, v := range req.Headers {
		outReq.Header.Set(k, v.(string))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, "error sending request "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	http.Redirect(w, r, "/requests/"+req.ID, http.StatusSeeOther)

}
