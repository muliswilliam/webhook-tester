package sqlstore

import (
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

func CreateWebhookRequest(wr models.WebhookRequest) error {
	result := db.DB.Create(&wr)
	return result.Error
}
