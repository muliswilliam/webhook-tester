package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestCursor_RoundTrip(t *testing.T) {
	c := RequestCursor{ReceivedAt: time.Date(2026, 9, 30, 12, 0, 0, 123456000, time.UTC), ID: "a:b-c_d"}
	parsed, err := ParseRequestCursor(c.String())
	require.NoError(t, err)
	assert.True(t, parsed.ReceivedAt.Equal(c.ReceivedAt))
	assert.Equal(t, c.ID, parsed.ID)

	zero, err := ParseRequestCursor("")
	require.NoError(t, err)
	assert.True(t, zero.IsZero())
	assert.Equal(t, "", RequestCursor{}.String())
}

func TestParseRequestCursor_Malformed(t *testing.T) {
	for _, s := range []string{"nope", "12:", "x:id", ":id"} {
		_, err := ParseRequestCursor(s)
		assert.Error(t, err, s)
	}
}

func TestRequestCursor_Before(t *testing.T) {
	t0 := time.Now()
	a := RequestCursor{ReceivedAt: t0, ID: "a"}
	b := RequestCursor{ReceivedAt: t0, ID: "b"}
	later := RequestCursor{ReceivedAt: t0.Add(time.Microsecond), ID: "a"}

	assert.True(t, RequestCursor{}.Before(a))
	assert.True(t, a.Before(b), "ties break on ID")
	assert.True(t, b.Before(later))
	assert.False(t, a.Before(a))
}

func TestLatestCursor(t *testing.T) {
	t0 := time.Now()
	requests := []WebhookRequest{
		{ID: "old", ReceivedAt: t0},
		{ID: "new", ReceivedAt: t0.Add(time.Second)},
		{ID: "mid", ReceivedAt: t0.Add(time.Millisecond)},
	}
	assert.Equal(t, "new", LatestCursor(requests).ID)
	assert.True(t, LatestCursor(nil).IsZero())
}
