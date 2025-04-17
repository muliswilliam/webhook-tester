package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"
)

func CreateWebhook(w http.ResponseWriter, r *http.Request) {
	userID, err := sessions.Authorize(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}

	err = r.ParseForm()
	if err != nil {
		log.Printf("error parsing form: %v", err)
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

	webhookID := utils.GenerateID()
	wh := models.Webhook{
		ID:            webhookID,
		UserID:        int(userID),
		Title:         title,
		ContentType:   &contentType,
		ResponseCode:  responseCode,
		ResponseDelay: uint(responseDelay),
		Payload:       &payload,
		NotifyOnEvent: notify,
	}

	err = db.DB.Create(&wh).Error
	if err != nil {
		log.Printf("Error creating webhook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/?address=%s", webhookID), http.StatusSeeOther)
}
