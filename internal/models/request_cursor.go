package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RequestCursor is a position in a webhook's request history, which is
// ordered by (ReceivedAt, ID). The zero value is the position before the
// first request.
type RequestCursor struct {
	ReceivedAt time.Time
	ID         string
}

// CursorAt returns the position of wr.
func CursorAt(wr WebhookRequest) RequestCursor {
	return RequestCursor{ReceivedAt: wr.ReceivedAt, ID: wr.ID}
}

// LatestCursor returns the position of the newest request in requests, or
// the zero cursor if there are none.
func LatestCursor(requests []WebhookRequest) RequestCursor {
	var latest RequestCursor
	for _, wr := range requests {
		if c := CursorAt(wr); latest.Before(c) {
			latest = c
		}
	}
	return latest
}

func (c RequestCursor) IsZero() bool {
	return c.ID == "" && c.ReceivedAt.IsZero()
}

// Before reports whether c is strictly earlier than other.
func (c RequestCursor) Before(other RequestCursor) bool {
	if !c.ReceivedAt.Equal(other.ReceivedAt) {
		return c.ReceivedAt.Before(other.ReceivedAt)
	}
	return c.ID < other.ID
}

// String encodes c as "<unix microseconds>:<id>", or "" for the zero cursor.
func (c RequestCursor) String() string {
	if c.IsZero() {
		return ""
	}
	return strconv.FormatInt(c.ReceivedAt.UnixMicro(), 10) + ":" + c.ID
}

// ParseRequestCursor decodes the output of RequestCursor.String.
func ParseRequestCursor(s string) (RequestCursor, error) {
	if s == "" {
		return RequestCursor{}, nil
	}
	micros, id, ok := strings.Cut(s, ":")
	if !ok || id == "" {
		return RequestCursor{}, fmt.Errorf("malformed request cursor %q", s)
	}
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return RequestCursor{}, fmt.Errorf("malformed request cursor %q: %w", s, err)
	}
	return RequestCursor{ReceivedAt: time.UnixMicro(us).UTC(), ID: id}, nil
}
