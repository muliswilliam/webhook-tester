package models

import (
	"time"

	"gorm.io/datatypes"
)

type WebhookRequest struct {
	ID         string            `gorm:"primaryKey" json:"id"`
	WebhookID  string            `json:"webhook_id"`
	Method     string            `json:"method"`
	Headers    datatypes.JSONMap `json:"headers"`
	Query      datatypes.JSONMap `json:"query"`
	Body       string            `json:"body"`
	ReceivedAt time.Time         `json:"received_at"`
}
