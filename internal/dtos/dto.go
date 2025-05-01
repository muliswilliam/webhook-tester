package dtos

import (
	"time"
	"webhook-tester/internal/models"

	"gorm.io/datatypes"
)

// CreateWebhookRequest
// swagger:request
type CreateWebhookRequest struct {
	// Title of the webhook
	// required: true
	Title         string `json:"title"`
	ResponseCode  int    `json:"response_code"`
	ResponseDelay uint   `json:"response_delay"` // milliseconds
	ContentType   string `json:"content_type"`
	Payload       string `json:"payload"`
	NotifyOnEvent bool   `json:"notify_on_event"`
} // @name CreateWebhookRequest

type UpdateWebhookRequest struct {
	CreateWebhookRequest
} // @name UpdateWebhookRequest

// ErrorResponse represents an error payload
type ErrorResponse struct {
	Error string `json:"error" example:"Webhook not found"`
} // @name ErrorResponse

type WebhookRequest struct {
	ID         string            `gorm:"primaryKey" json:"id"`
	WebhookID  string            `json:"webhook_id"`
	Method     string            `json:"method"`
	Headers    datatypes.JSONMap `json:"headers"`
	Query      datatypes.JSONMap `json:"query"`
	Body       string            `json:"body"`
	ReceivedAt time.Time         `json:"received_at"`
} // @name WebhookRequest

// swagger:model
type Webhook struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	Title         string    `json:"title"`
	ResponseCode  int       `json:"response_code"`
	ResponseDelay uint      `json:"response_delay"` // milliseconds
	ContentType   string    `json:"content_type"`
	Payload       string    `json:"payload"`
	NotifyOnEvent bool      `json:"notify_on_event"`
	UserID        int       `json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Requests      []models.WebhookRequest
} // @name Webhook

// Creates a new instance of Webhook DTO from models.Webhook
func NewWebhookDTO(w models.Webhook) Webhook {
	dto := Webhook{
		ID:            w.ID,
		Title:         w.Title,
		ResponseCode:  w.ResponseCode,
		ResponseDelay: w.ResponseDelay,
		ContentType:   *w.ContentType,
		Payload:       *w.Payload,
		UserID:        w.UserID,
		CreatedAt:     w.CreatedAt,
		UpdatedAt:     w.UpdatedAt,
		NotifyOnEvent: w.NotifyOnEvent,
		Requests:      w.Requests,
	}

	return dto
}
