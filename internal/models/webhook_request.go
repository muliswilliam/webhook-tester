package models

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
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
// commas, which is how the API returns them. Pages show one value per line.
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

// ErrSubpathLeavesBase is returned by URLAt for a subpath that could lead
// out of the base URL's path.
var ErrSubpathLeavesBase = errors.New("the subpath could lead out of the base URL's path")

// URLAt is the URL the request addresses when relayed under base: base's
// path followed by the request's subpath, and base's query followed by the
// request's query string. Both are kept as written, percent-encoding and
// parameter order included. A subpath that some server could read as
// climbing above base's path is refused with ErrSubpathLeavesBase; see
// climbsOut.
func (wr WebhookRequest) URLAt(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if wr.Path != "" {
		if climbsOut(wr.Path) {
			return "", fmt.Errorf("%w: %q", ErrSubpathLeavesBase, wr.Path)
		}
		escaped := strings.TrimSuffix(u.EscapedPath(), "/") + wr.Path
		if decoded, err := url.PathUnescape(escaped); err == nil {
			u.Path, u.RawPath = decoded, escaped
		} else {
			// An older row whose decoded path holds a lone "%".
			u.Path, u.RawPath = strings.TrimSuffix(u.Path, "/")+wr.Path, ""
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

// climbsOut reports whether some server could read a ".." segment in the
// subpath p, and so resolve it to a path above the one p is appended to.
// Servers disagree on what a ".." is, so rather than rewrite p for one
// reading, it is checked against all of them:
//   - Some decode "%2e", "%2f" and "%5c" before splitting, some decode more
//     than once, and some decode the valid escapes around an invalid one.
//   - Old IIS decoded overlong UTF-8 ("%c0%ae" for "."), and some servers
//     apply NFKC normalization ("\uff0e" for ".").
//
// So p is decoded, skipping invalid escapes, overlong UTF-8 is decoded and
// NFKC applied, until nothing changes; a p that keeps changing is refused.
// Then:
//   - Some split on "\" too, and some (Tomcat, Spring) ignore ";parameters"
//     in a segment.
//   - Windows drops a segment's trailing dots and spaces, and C servers stop
//     at a NUL, so a segment that is ".." followed only by dots, spaces,
//     tabs or NULs counts as one.
func climbsOut(p string) bool {
	for i := 0; ; i++ {
		decoded := norm.NFKC.String(decodeOverlongUTF8(unescapeLeniently(p)))
		if decoded == p {
			break
		}
		if i == maxSubpathDecodings {
			return true
		}
		p = decoded
	}
	for _, seg := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		seg, _, _ = strings.Cut(seg, ";")
		if strings.HasPrefix(seg, "..") && strings.Trim(seg, ". \t\x00") == "" {
			return true
		}
	}
	return false
}

// maxSubpathDecodings bounds climbsOut's decoding. Real subpaths settle
// after one or two rounds.
const maxSubpathDecodings = 16

// unescapeLeniently decodes p's valid percent-escapes, keeping invalid ones
// as written.
func unescapeLeniently(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '%' && i+2 < len(p) && isHex(p[i+1]) && isHex(p[i+2]) {
			b.WriteByte(unhex(p[i+1])<<4 | unhex(p[i+2]))
			i += 2
			continue
		}
		b.WriteByte(p[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c <= 'F':
		return c - 'A' + 10
	default:
		return c - 'a' + 10
	}
}

// decodeOverlongUTF8 replaces each overlong UTF-8 sequence in s, such as
// "\xc0\xae" for ".", with the character it encodes. Other bytes, valid or
// not, are kept.
func decodeOverlongUTF8(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if r, n := overlongRune(s[i:]); n > 0 {
			b.WriteRune(r)
			i += n
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// overlongRune decodes an overlong UTF-8 sequence at the start of s,
// returning its character and length, or a length of 0 if there is none.
func overlongRune(s string) (rune, int) {
	for _, form := range [...]struct {
		lead, mask byte
		n          int
		min        rune // the least character that needs n bytes
	}{
		{0xc0, 0x1f, 2, 0x80},
		{0xe0, 0x0f, 3, 0x800},
		{0xf0, 0x07, 4, 0x10000},
	} {
		if len(s) < form.n || s[0]&^form.mask != form.lead {
			continue
		}
		r := rune(s[0] & form.mask)
		for i := 1; i < form.n; i++ {
			if s[i]&0xc0 != 0x80 {
				return 0, 0
			}
			r = r<<6 | rune(s[i]&0x3f)
		}
		if r < form.min {
			return r, form.n
		}
		return 0, 0
	}
	return 0, 0
}
