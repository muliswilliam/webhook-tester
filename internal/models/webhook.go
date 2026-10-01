package models

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
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
	MaxForwardURLLength = 2048
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
	// ForwardURL is where captured requests are relayed; nil when
	// forwarding is off. Only webhooks with an owner forward.
	ForwardURL *string   `json:"forward_url"`
	UserID     int       `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`

	Requests []WebhookRequest `gorm:"foreignKey:WebhookID" json:"requests,omitempty"`
}

// Forwards reports whether the webhook relays its captured requests: it has
// a forward URL and an owner. Guest webhooks never forward, so they can't be
// used as an open relay.
func (w *Webhook) Forwards() bool {
	return w.ForwardURL != nil && w.UserID != 0
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
	if w.ForwardURL != nil {
		if trimmed := strings.TrimSpace(*w.ForwardURL); trimmed == "" {
			w.ForwardURL = nil
		} else {
			w.ForwardURL = &trimmed
		}
	}
}

// Validate checks the webhook's title, configured response and forward URL,
// returning the first problem found. The forward URL's loop check compares
// against the DOMAIN setting, the same source replay builds its target from.
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
	if err := validateResponseHeaders(w.ResponseHeaders); err != nil {
		return err
	}
	if w.ForwardURL != nil {
		return ValidateForwardURL(*w.ForwardURL, os.Getenv("DOMAIN"))
	}
	return nil
}

// ValidateForwardURL reports whether raw can be used as a forward URL: an
// absolute http or https URL with a host, which doesn't point back at the
// webhook endpoints of the instance served at domain (the DOMAIN setting).
// An empty domain skips that loop check.
func ValidateForwardURL(raw, domain string) error {
	if len(raw) > MaxForwardURLLength {
		return fmt.Errorf("forward URL must be at most %d characters", MaxForwardURLLength)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("forward URL must be an absolute http or https URL")
	}
	if u.Hostname() == "" {
		return errors.New("forward URL must include a host")
	}
	if pointsAtOwnWebhooks(u, domain) {
		return errors.New("forward URL can't point at this Webhook Tester's own webhook endpoints, since that would loop")
	}
	return nil
}

// pointsAtOwnWebhooks reports whether u addresses a webhook endpoint of the
// instance served at domain, i.e. the same host and port with a path under
// <domain>/webhooks. The path is compared decoded and cleaned, so dot
// segments, doubled slashes and percent-encoding don't slip past.
func pointsAtOwnWebhooks(u *url.URL, domain string) bool {
	self, err := url.Parse(domain)
	if domain == "" || err != nil || self.Hostname() == "" {
		return false
	}
	if canonicalHost(u.Hostname()) != canonicalHost(self.Hostname()) || !samePort(u, self) {
		return false
	}
	base := path.Join("/", self.Path, "webhooks")
	p := path.Clean("/" + u.Path)
	return p == base || strings.HasPrefix(p, base+"/")
}

func canonicalHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// samePort reports whether a and b are served on the same port. The default
// http and https ports count as one, since an instance behind a TLS proxy
// usually answers on both.
func samePort(a, b *url.URL) bool {
	pa, pb := effectivePort(a), effectivePort(b)
	isDefault := func(p string) bool { return p == "80" || p == "443" }
	return pa == pb || (isDefault(pa) && isDefault(pb))
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
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
