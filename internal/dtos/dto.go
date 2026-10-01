package dtos

import (
	"encoding/json"
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
	// Absolute http or https URL every captured request is also relayed to.
	// Omit it, or send "", to leave forwarding off.
	ForwardURL string `json:"forward_url" example:"https://example.ngrok-free.app/webhooks/stripe"`
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
	// Absolute http or https URL every captured request is also relayed to.
	// null or "" turns forwarding off.
	ForwardURL NullableString `json:"forward_url,omitzero" swaggertype:"string" extensions:"x-nullable" example:"https://example.ngrok-free.app/webhooks/stripe"`
} // @name UpdateWebhookRequest

// NullableString is an optional JSON string field that tells an explicit
// null apart from a missing field: Set is true whenever the field is
// present, and Value is nil for null.
type NullableString struct {
	Set   bool
	Value *string
}

// NewNullableString returns a present field holding s, or null when s is nil.
func NewNullableString(s *string) NullableString {
	return NullableString{Set: true, Value: s}
}

// UnmarshalJSON records that the field is present. encoding/json calls it
// for null too, which is what tells null from missing.
func (n *NullableString) UnmarshalJSON(data []byte) error {
	n.Set = true
	n.Value = nil
	if string(data) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("expected a string or null, got %s", jsonKind(data))
	}
	n.Value = &s
	return nil
}

// jsonKind names the kind of a valid JSON value, for error messages.
func jsonKind(data []byte) string {
	if len(data) == 0 {
		return "nothing"
	}
	switch data[0] {
	case '{':
		return "an object"
	case '[':
		return "an array"
	case 't', 'f':
		return "a boolean"
	default:
		return "a number"
	}
}

// MarshalJSON encodes the value, or null. Pair it with omitzero so an unset
// field is left out.
func (n NullableString) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.Value)
}

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
	// Where captured requests are relayed; null when forwarding is off
	ForwardURL *string          `json:"forward_url" extensions:"x-nullable" example:"https://example.ngrok-free.app/webhooks/stripe"`
	UserID     int              `json:"user_id"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Requests   []WebhookRequest `json:"requests"`
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
		ForwardURL:      w.ForwardURL,
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
