package models

import "time"

type Webhook struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	ResponseCode  int       `json:"response_code"`
	ResponseDelay uint      `json:"response_delay"` // milliseconds
	ContentType   string    `json:"content_type"`
	Payload       string    `json:"payload"`
	CreatedAt     time.Time `json:"created_at"`
	NofifyOnEvent bool      `json:"notify_on_event"`
}
