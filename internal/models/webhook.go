package models

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/datatypes"
)

// Bounds and defaults for a webhook's configured response.
const (
	MinResponseCode     = 100
	MaxResponseCode     = 599
	MaxResponseDelay    = 30_000 // milliseconds
	DefaultResponseCode = 200
	DefaultContentType  = "application/json"
)

// swagger:model [Webhook]
type Webhook struct {
	ID              string            `gorm:"primaryKey" json:"id"`
	Title           string            `json:"title"`
	ResponseCode    int               `json:"response_code"`
	ResponseDelay   uint              `json:"response_delay"` // milliseconds
	ContentType     *string           `json:"content_type"`
	Payload         *string           `json:"payload"`
	ResponseHeaders datatypes.JSONMap `json:"response_headers"`
	NotifyOnEvent   bool              `json:"notify_on_event"`
	UserID          int               `json:"user_id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at,omitempty"`

	Requests []WebhookRequest `gorm:"foreignKey:WebhookID" json:"requests,omitempty"`
}

// ValidateResponseCode reports whether code can be sent as an HTTP status.
func ValidateResponseCode(code int) error {
	if code < MinResponseCode || code > MaxResponseCode {
		return fmt.Errorf("response code must be between %d and %d", MinResponseCode, MaxResponseCode)
	}
	return nil
}

// serverManagedHeaders are set by the server itself; overriding them would
// corrupt the response.
var serverManagedHeaders = map[string]bool{
	"Content-Length":    true,
	"Transfer-Encoding": true,
}

// validateResponseHeaders reports whether every header is safe to send.
func validateResponseHeaders(headers map[string]any) error {
	for name, value := range headers {
		if !isHeaderToken(name) {
			return fmt.Errorf("invalid header name %q", name)
		}
		if serverManagedHeaders[http.CanonicalHeaderKey(name)] {
			return fmt.Errorf("header %q is set by the server and can't be overridden", name)
		}
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("header %q must have a string value", name)
		}
		if strings.ContainsFunc(s, func(r rune) bool { return (r < ' ' && r != '\t') || r == 0x7f }) {
			return fmt.Errorf("header %q has an invalid value", name)
		}
	}
	return nil
}

// Normalize trims the title and fills in the defaults for an unset response
// code and content type. Call it before Validate.
func (w *Webhook) Normalize() {
	w.Title = strings.TrimSpace(w.Title)
	if w.ResponseCode == 0 {
		w.ResponseCode = DefaultResponseCode
	}
	if w.ContentType == nil || *w.ContentType == "" {
		ct := DefaultContentType
		w.ContentType = &ct
	}
	if w.Payload == nil {
		empty := ""
		w.Payload = &empty
	}
}

// Validate checks the webhook's title and configured response, returning
// the first problem found.
func (w *Webhook) Validate() error {
	if w.Title == "" {
		return errors.New("title is required")
	}
	if err := ValidateResponseCode(w.ResponseCode); err != nil {
		return err
	}
	if w.ResponseDelay > MaxResponseDelay {
		return fmt.Errorf("response delay must be between 0 and %d ms", MaxResponseDelay)
	}
	return validateResponseHeaders(w.ResponseHeaders)
}

// isHeaderToken reports whether s is an RFC 9110 token, the syntax of a
// header field name.
func isHeaderToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", r):
		default:
			return false
		}
	}
	return true
}
