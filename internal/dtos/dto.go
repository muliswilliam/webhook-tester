package dtos

import (
	"fmt"
	"time"
	"webhook-tester/internal/models"
)

// CreateWebhookRequest
// swagger:request
type CreateWebhookRequest struct {
	// Title of the webhook
	Title string `json:"title" binding:"required" example:"Payment events"`
	// HTTP status code the endpoint answers with, 100-599. Defaults to 200.
	ResponseCode int `json:"response_code" example:"200"`
	// Milliseconds to wait before answering, 0-30000
	ResponseDelay uint `json:"response_delay" example:"0"`
	// Content-Type of the response. Defaults to application/json.
	ContentType string `json:"content_type" example:"application/json"`
	// Response body
	Payload string `json:"payload" example:"{\"message\":\"ok\"}"`
	// Extra response headers
	ResponseHeaders map[string]string `json:"response_headers"`
	NotifyOnEvent   bool              `json:"notify_on_event"`
} // @name CreateWebhookRequest

// UpdateWebhookRequest changes only the fields it includes.
type UpdateWebhookRequest struct {
	Title         *string `json:"title" example:"Payment events"`
	ResponseCode  *int    `json:"response_code" example:"200"`
	ResponseDelay *uint   `json:"response_delay" example:"0"`
	ContentType   *string `json:"content_type" example:"application/json"`
	Payload       *string `json:"payload" example:"{\"message\":\"ok\"}"`
	// Replaces all response headers
	ResponseHeaders *map[string]string `json:"response_headers"`
	NotifyOnEvent   *bool              `json:"notify_on_event"`
} // @name UpdateWebhookRequest

// ErrorResponse represents an error payload
type ErrorResponse struct {
	Error string `json:"error" example:"webhook not found"`
} // @name ErrorResponse

type WebhookRequest struct {
	ID         string            `json:"id"`
	WebhookID  string            `json:"webhook_id"`
	Method     string            `json:"method"`
	Path       string            `json:"path" example:"/orders/42"`
	Headers    map[string]string `json:"headers"`
	Query      map[string]string `json:"query"`
	Body       string            `json:"body"`
	ReceivedAt time.Time         `json:"received_at"`
} // @name WebhookRequest

// swagger:model
type Webhook struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	ResponseCode    int               `json:"response_code"`
	ResponseDelay   uint              `json:"response_delay"` // milliseconds
	ContentType     string            `json:"content_type"`
	Payload         string            `json:"payload"`
	ResponseHeaders map[string]string `json:"response_headers"`
	NotifyOnEvent   bool              `json:"notify_on_event"`
	UserID          int               `json:"user_id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Requests        []WebhookRequest  `json:"requests"`
} // @name Webhook

// NewWebhookDTO creates a Webhook DTO from a models.Webhook. Timestamps are
// normalized to UTC, and absent maps and lists are rendered empty, not null.
func NewWebhookDTO(w models.Webhook) Webhook {
	dto := Webhook{
		ID:              w.ID,
		Title:           w.Title,
		ResponseCode:    w.ResponseCode,
		ResponseDelay:   w.ResponseDelay,
		ContentType:     deref(w.ContentType),
		Payload:         deref(w.Payload),
		ResponseHeaders: stringMap(w.ResponseHeaders),
		UserID:          w.UserID,
		CreatedAt:       w.CreatedAt.UTC(),
		UpdatedAt:       w.UpdatedAt.UTC(),
		NotifyOnEvent:   w.NotifyOnEvent,
		Requests:        make([]WebhookRequest, len(w.Requests)),
	}
	for i, r := range w.Requests {
		dto.Requests[i] = NewWebhookRequestDTO(r)
	}
	return dto
}

// NewWebhookRequestDTO creates a WebhookRequest DTO from a captured request.
func NewWebhookRequestDTO(r models.WebhookRequest) WebhookRequest {
	return WebhookRequest{
		ID:         r.ID,
		WebhookID:  r.WebhookID,
		Method:     r.Method,
		Path:       r.Path,
		Headers:    stringMap(r.Headers),
		Query:      stringMap(r.Query),
		Body:       r.Body,
		ReceivedAt: r.ReceivedAt.UTC(),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringMap(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}
