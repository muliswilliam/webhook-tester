package sqlstore

import (
	"database/sql"
	"fmt"
	"log"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

func InsertWebhook(w models.Webhook) error {
	_, err := db.DB.Exec(`
		INSERT INTO webhooks (id, title, response_code, content_type, response_delay, payload, notify_on_event, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Title, w.ResponseCode, w.ContentType, w.ResponseDelay, w.Payload, w.NotifyOnEvent, w.CreatedAt,
	)

	if err != nil {
		log.Printf("error inserting webhook: %v", err)
	}

	return err
}

func GetWebhook(id string) (models.Webhook, error) {
	var w models.Webhook
	row := db.DB.QueryRow(`SELECT
		id, title, response_code, content_type, response_delay, payload, notify_on_event, created_at, updated_at
		FROM webhooks
		WHERE id = ?`, id)

	var updatedAt sql.NullTime

	err := row.Scan(&w.ID, &w.Title, &w.ResponseCode, &w.ContentType, &w.ResponseDelay, &w.Payload, &w.NotifyOnEvent, &w.CreatedAt, &updatedAt)
	if updatedAt.Valid {
		w.UpdatedAt = updatedAt.Time
	}
	return w, err
}

func GetAllWebhooks() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	rows, err := db.DB.Query(`SELECT
	id, title, response_code, content_type, response_delay, payload, notify_on_event, created_at, updated_at
	FROM webhooks`)
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}(rows)

	if err != nil {
		log.Printf("error getting all webhooks: %v", err)
		return nil, err
	}

	for rows.Next() {
		var w models.Webhook
		var updatedAt sql.NullTime
		err = rows.Scan(&w.ID, &w.Title, &w.ResponseCode, &w.ContentType, &w.ResponseDelay, &w.Payload, &w.NotifyOnEvent, &w.CreatedAt, &updatedAt)
		if updatedAt.Valid {
			w.UpdatedAt = updatedAt.Time
		}
		if err != nil {
			log.Printf("error scanning row: %v", err)
			continue
		}
		webhooks = append(webhooks, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error in rows: %v", err)
	}

	return webhooks, nil
}

func UpdateWebhook(w models.Webhook) error {
	sql := `UPDATE webhooks SET(title, response_code, content_type, response_delay, payload, notify_on_event, updated_at) = (?, ?, ?, ?, ?, ?, ?) WHERE id = ?`

	_, err := db.DB.Exec(sql, w.Title, w.ResponseCode, w.ContentType, w.ResponseDelay, w.Payload, w.NotifyOnEvent, w.UpdatedAt, w.ID)
	if err != nil {
		log.Printf("error updating webhook: %v", err)
	}

	return err
}

func DeleteWebhook(id string) error {
	res, err := db.DB.Exec(`DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		log.Printf("error deleting webhook: %v", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		log.Printf("error deleting webhooks: %v", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return err
}
