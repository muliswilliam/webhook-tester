package models

import (
	"time"

	"gorm.io/datatypes"
)

// swagger:model WebhookRequest
type WebhookRequest struct {
	ID         string            `gorm:"primaryKey" json:"id"`
	WebhookID  string            `json:"webhook_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"` // subpath after the webhook URL, e.g. "/orders/42"; "" for none
	Headers    datatypes.JSONMap `json:"headers"`
	Query      datatypes.JSONMap `json:"query"`
	Body       string            `json:"body"`
	ReceivedAt time.Time         `json:"received_at"`
} // @name WebhookRequest
