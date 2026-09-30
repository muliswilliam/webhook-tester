package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateResponseCode(t *testing.T) {
	for _, code := range []int{100, 200, 418, 599} {
		assert.NoError(t, ValidateResponseCode(code), code)
	}
	for _, code := range []int{0, 99, 600, 999, 1000, -1} {
		assert.Error(t, ValidateResponseCode(code), code)
	}
}

func TestValidateResponseHeaders(t *testing.T) {
	valid := []map[string]any{
		nil,
		{},
		{"X-Test": "1", "Cache-Control": "no-cache", "x_under.score~": "a\tb"},
	}
	for _, h := range valid {
		assert.NoError(t, validateResponseHeaders(h), h)
	}

	invalid := []map[string]any{
		{"": "1"},
		{"Bad Name": "1"},
		{"X-Ünicode": "1"},
		{"X:Colon": "1"},
		{"X-Test": 1},
		{"X-Test": "a\r\nInjected: yes"},
		{"X-Test": "nul\x00"},
		{"content-length": "10"},
		{"Transfer-Encoding": "chunked"},
	}
	for _, h := range invalid {
		assert.Error(t, validateResponseHeaders(h), h)
	}
}

func TestWebhookValidate(t *testing.T) {
	assert.NoError(t, (&Webhook{Title: "t", ResponseCode: 200, ResponseDelay: MaxResponseDelay}).Validate())
	assert.ErrorContains(t, (&Webhook{ResponseCode: 200}).Validate(), "title")
	assert.ErrorContains(t, (&Webhook{Title: "t", ResponseCode: 1000}).Validate(), "response code")
	assert.ErrorContains(t, (&Webhook{Title: "t", ResponseCode: 200, ResponseDelay: MaxResponseDelay + 1}).Validate(), "response delay")
	assert.ErrorContains(t, (&Webhook{Title: "t", ResponseCode: 200, ResponseHeaders: map[string]any{"a b": "1"}}).Validate(), "header")
}

func TestWebhookNormalize(t *testing.T) {
	w := Webhook{Title: "  Payments  "}
	w.Normalize()
	assert.Equal(t, "Payments", w.Title)
	assert.Equal(t, DefaultResponseCode, w.ResponseCode)
	assert.Equal(t, DefaultContentType, *w.ContentType)
	assert.Equal(t, "", *w.Payload)

	ct, payload := "text/plain", "hi"
	w = Webhook{ResponseCode: 201, ContentType: &ct, Payload: &payload}
	w.Normalize()
	assert.Equal(t, 201, w.ResponseCode, "set values are kept")
	assert.Equal(t, "text/plain", *w.ContentType)
	assert.Equal(t, "hi", *w.Payload)

	empty := ""
	w = Webhook{ContentType: &empty}
	w.Normalize()
	assert.Equal(t, DefaultContentType, *w.ContentType, "an empty content type gets the default")
}
