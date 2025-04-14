package webhook

import (
	"database/sql"
	"errors"
	"gorm.io/datatypes"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
)

func HandleWebhookRequest(w http.ResponseWriter, r *http.Request) {
	webhookID := strings.TrimPrefix(r.URL.Path, "/")
	log.Printf("Handling webhook request for %s", webhookID)
	var webhook models.Webhook
	webhook, err := sqlstore.GetWebhook(webhookID)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			w.WriteHeader(http.StatusNotFound)
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
			log.Printf("error closing body: %s", err)
		}
	}(r.Body)

	// Convert headers to a map[string]string
	headers := datatypes.JSONMap{}
	for k, v := range r.Header {
		headers[k] = strings.Join(v, ",")
	}

	query := datatypes.JSONMap{}
	for k, v := range r.URL.Query() {
		query[k] = strings.Join(v, ",")
	}

	wr := models.WebhookRequest{
		ID:         utils.GenerateID(),
		WebhookID:  webhookID,
		Method:     r.Method,
		Headers:    headers,
		Query:      query,
		Body:       string(body),
		ReceivedAt: time.Now().UTC(),
	}

	err = sqlstore.CreateWebhookRequest(wr)
	if err != nil {
		log.Printf("error creating webhook request: %s", err)
		utils.RenderJSON(w, http.StatusInternalServerError, nil)

		return
	}

	// Delay response
	if webhook.ResponseDelay > 0 {
		time.Sleep(time.Duration(webhook.ResponseDelay) * time.Millisecond)
	}

	// Return custom response
	if webhook.ContentType != nil {
		w.Header().Set("Content-Type", *webhook.ContentType)
	} else {
		// Default to application json if content type is not specified
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(webhook.ResponseCode)
	if &webhook.Payload != nil {
		if _, err := w.Write([]byte(*webhook.Payload)); err != nil {
			log.Printf("error writing payload: %s", err)
		}
	}
}
