package webhook

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"gorm.io/datatypes"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
)

var webhookStreams = make(map[string][]chan string)
var mu sync.Mutex

func HandleWebhookRequest(w http.ResponseWriter, r *http.Request) {
	webhookID := strings.TrimPrefix(r.URL.Path, "/webhooks/")
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

	jsonData, _ := json.Marshal(wr)

	mu.Lock()
	for _, ch := range webhookStreams[webhookID] {
		select {
		case ch <- string(jsonData):
		default: // drop if blocked
		}
	}
	mu.Unlock()

	// Return custom response
	if webhook.ContentType != nil {
		w.Header().Set("Content-Type", *webhook.ContentType)
	} else {
		// Default to application json if content type is not specified
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(webhook.ResponseCode)
	if webhook.Payload != nil {
		if _, err := w.Write([]byte(*webhook.Payload)); err != nil {
			log.Printf("error writing payload: %s", err)
		}
	}
}

func StreamWebhookEvents(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create a channel for this client
	eventChan := make(chan string)
	mu.Lock()
	webhookStreams[webhookID] = append(webhookStreams[webhookID], eventChan)
	mu.Unlock()

	// Send keep-alive every 15s to avoid timeouts
	//go func() {
	//	ticker := time.NewTicker(15 * time.Second)
	//	defer ticker.Stop()
	//	for range ticker.C {
	//		fmt.Fprintf(w, ":\n\n")
	//		flusher, _ := w.(http.Flusher)
	//		flusher.Flush()
	//	}
	//}()

	// Stream new events
	flusher, _ := w.(http.Flusher)
	for {
		select {
		case msg := <-eventChan:
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				log.Printf("error writing data: %s", err)
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			mu.Lock()
			subs := webhookStreams[webhookID]
			for i, sub := range subs {
				if sub == eventChan {
					webhookStreams[webhookID] = append(subs[:i], subs[i+1:]...)
					break
				}
			}
			mu.Unlock()
			return
		}
	}
}
