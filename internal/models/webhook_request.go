package models

import "time"

type WebhookRequest struct {
	ID         string            `json:"id"`
	WebhookID  string            `json:"webhook_id"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	Query      map[string]string `json:"query"`
	Body       string            `json:"body"`
	ReceivedAt time.Time         `json:"received_at"`
}
