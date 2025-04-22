package sqlstore

import (
	"gorm.io/gorm"
	"webhook-tester/internal/models"
)

func CreateWebhookRequest(db *gorm.DB, wr models.WebhookRequest) error {
	result := db.Create(&wr)
	return result.Error
}
