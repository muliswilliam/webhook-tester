package models

import (
	"time"
)

type Webhook struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	Title         string    `json:"title"`
	ResponseCode  int       `json:"response_code"`
	ResponseDelay uint      `json:"response_delay"` // milliseconds
	ContentType   *string   `json:"content_type"`
	Payload       *string   `json:"payload"`
	NotifyOnEvent bool      `json:"notify_on_event"`
	UserID        int       `json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`

	Requests []WebhookRequest `gorm:"foreignKey:WebhookID" json:"requests,omitempty"`
}
