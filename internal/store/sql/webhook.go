package sqlstore

import (
	"log"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

func InsertWebhook(w models.Webhook) error {
	_, err := db.DB.Exec(`
		INSERT INTO webhooks (id, title, response_code, content_type, response_delay, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Title, w.ResponseCode, w.ContentType, w.ResponseDelay, w.Payload, w.CreatedAt,
	)

	if err != nil {
		log.Printf("error inserting webhook: %v", err)
	}

	return err
}

func GetWebhook(id string) (models.Webhook, error) {
	var w models.Webhook

	row := db.DB.QueryRow(`
		SELECT id, title, response_code, content_type, response_delay, payload, created_at
		FROM webhooks
		WHERE id = ?`, id)

	err := row.Scan(&w.ID, &w.Title, &w.ResponseCode, &w.ContentType, &w.ResponseDelay, &w.Payload, &w.CreatedAt)
	return w, err
}
