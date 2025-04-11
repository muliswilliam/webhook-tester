package sqlstore

import (
	"encoding/json"
	"log"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

func mapToJSON(m map[string]string) (string, error) {
	j, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	return string(j), nil
}

func CreateWebhookRequest(wr models.WebhookRequest) error {
	sql := `INSERT INTO webhook_requests
	(id, webhook_id, method, headers, query, body, received_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)`

	headersJson, err := mapToJSON(wr.Headers)
	if err != nil {
		log.Printf("error headers json: %s", err)
		return err
	}

	queryJson, err := mapToJSON(wr.Query)
	if err != nil {
		log.Printf("error query json: %s", err)
		return err
	}

	_, err = db.DB.Exec(sql, wr.ID, wr.WebhookID, wr.Method, headersJson, queryJson, wr.Body, wr.ReceivedAt)
	if err != nil {
		log.Printf("error inserting webhook request: %v", err)
	}
	return err
}
