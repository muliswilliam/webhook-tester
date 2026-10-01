package models

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/datatypes"
)

// swagger:model WebhookRequest
type WebhookRequest struct {
	ID        string `gorm:"primaryKey" json:"id"`
	WebhookID string `json:"webhook_id"`
	Method    string `json:"method"`
	// Path is the subpath after the webhook URL as sent, percent-encoding
	// included, e.g. "/orders/42"; "" for none. Rows captured before it was
	// kept escaped may hold the decoded path.
	Path string `json:"path"`
	// Headers and Query map each name to its value, or to the list of its
	// values when it was repeated; see CapturedValues. Rows captured before
	// repeated ones were kept hold a single comma-joined value instead.
	Headers datatypes.JSONMap `json:"headers"`
	Query   datatypes.JSONMap `json:"query"`
	// RawQuery is the query string as sent, without the "?". Rows captured
	// before it was kept have "" and only Query; see QueryString.
	RawQuery   string    `gorm:"not null;default:''" json:"raw_query"`
	Body       string    `json:"body"`
	ReceivedAt time.Time `json:"received_at"`

	// Deliveries are the attempts to relay this request to a forward
	// target. They aren't part of the API. The delete paths remove them
	// explicitly; the cascade also removes one a forward records while its
	// request is being deleted, which would otherwise fail the delete.
	Deliveries []Delivery `gorm:"foreignKey:RequestID;constraint:OnDelete:CASCADE" json:"-"`
} // @name WebhookRequest

// CapturedValues stores headers or query parameters as a captured request
// keeps them: a name sent once maps to its value, and a repeated one to the
// list of its values, in the order sent.
func CapturedValues(values map[string][]string) datatypes.JSONMap {
	m := make(datatypes.JSONMap, len(values))
	for k, vs := range values {
		if len(vs) == 1 {
			m[k] = vs[0]
		} else {
			m[k] = vs
		}
	}
	return m
}

// FieldValues returns the values of one captured header or query parameter.
// It accepts every form a stored value can take: a string, a list of
// strings, or, from a hand-edited row, anything else, which is formatted.
func FieldValues(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, len(v))
		for i, e := range v {
			out[i] = formatField(e)
		}
		return out
	default:
		return []string{formatField(v)}
	}
}

// FieldValue is a captured header's or query parameter's values joined with
// commas, which is how they are displayed and returned by the API.
func FieldValue(v any) string {
	return strings.Join(FieldValues(v), ",")
}

func formatField(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// HeaderValues returns the captured headers with all their values.
func (wr WebhookRequest) HeaderValues() http.Header {
	h := make(http.Header, len(wr.Headers))
	for k, v := range wr.Headers {
		h[k] = FieldValues(v)
	}
	return h
}

// QueryString is the query string the request was sent with. For rows
// captured before RawQuery was kept, it is rebuilt from Query.
func (wr WebhookRequest) QueryString() string {
	if wr.RawQuery != "" || len(wr.Query) == 0 {
		return wr.RawQuery
	}
	q := make(url.Values, len(wr.Query))
	for k, v := range wr.Query {
		q[k] = FieldValues(v)
	}
	return q.Encode()
}

// URLAt is the URL the request addresses when relayed under base: base's
// path followed by the request's subpath, and base's query followed by the
// request's query string. Both are kept as written, percent-encoding and
// parameter order included, except that the subpath's dot segments are
// resolved, so it can't lead out of base's path.
func (wr WebhookRequest) URLAt(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if wr.Path != "" {
		sub := removeDotSegments(wr.Path)
		escaped := strings.TrimSuffix(u.EscapedPath(), "/") + sub
		if decoded, err := url.PathUnescape(escaped); err == nil {
			u.Path, u.RawPath = decoded, escaped
		} else {
			// An older row whose decoded path holds a lone "%".
			u.Path, u.RawPath = strings.TrimSuffix(u.Path, "/")+sub, ""
		}
	}
	if q := wr.QueryString(); q != "" {
		if u.RawQuery != "" {
			u.RawQuery += "&" + q
		} else {
			u.RawQuery = q
		}
	}
	return u.String(), nil
}

// removeDotSegments resolves the "." and ".." segments of a subpath, which
// starts with "/", as RFC 3986 section 5.2.4 does: a ".." beyond the start
// is dropped, so the result never climbs above it. Segments are compared
// decoded, since many servers treat "%2e" as ".", and the others are kept
// as written.
func removeDotSegments(p string) string {
	segments := strings.Split(strings.TrimPrefix(p, "/"), "/")
	out := make([]string, 0, len(segments))
	for i, seg := range segments {
		decoded, err := url.PathUnescape(seg)
		if err != nil || (decoded != "." && decoded != "..") {
			out = append(out, seg)
			continue
		}
		if decoded == ".." && len(out) > 0 {
			out = out[:len(out)-1]
		}
		if i == len(segments)-1 {
			out = append(out, "") // "/a/b/.." is "/a/"
		}
	}
	return "/" + strings.Join(out, "/")
}
