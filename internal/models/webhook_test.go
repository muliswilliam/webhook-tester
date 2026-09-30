package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestValidateForwardURL(t *testing.T) {
	const domain = "https://webhooks.example.com"
	cases := []struct {
		name, url, domain, wantErr string
	}{
		{name: "https URL", url: "https://api.example.com/hooks/stripe", domain: domain},
		{name: "http URL with port, query and userinfo", url: "http://user:pw@localhost:8080/hooks?x=1", domain: domain},
		{name: "tunnel URL", url: "https://abc123.ngrok-free.app", domain: domain},
		{name: "same host, other path", url: "https://webhooks.example.com/api/webhooks", domain: domain},
		{name: "same host, other port", url: "https://webhooks.example.com:8443/webhooks/abc", domain: domain},
		{name: "same host and port, path not under webhooks", url: "http://localhost:3000/webhooksx", domain: "http://localhost:3000"},
		{name: "other port on a local instance", url: "http://localhost:8080/webhooks/stripe", domain: "http://localhost:3000"},
		{name: "no DOMAIN skips the loop check", url: "https://webhooks.example.com/webhooks/abc", domain: ""},

		{name: "relative URL", url: "/hooks/stripe", domain: domain, wantErr: "absolute http or https URL"},
		{name: "no scheme", url: "api.example.com/hooks", domain: domain, wantErr: "absolute http or https URL"},
		{name: "ftp scheme", url: "ftp://files.example.com/hooks", domain: domain, wantErr: "absolute http or https URL"},
		{name: "javascript scheme", url: "javascript:alert(1)", domain: domain, wantErr: "absolute http or https URL"},
		{name: "no host", url: "https:///hooks", domain: domain, wantErr: "host"},
		{name: "port only", url: "https://:8080/hooks", domain: domain, wantErr: "host"},
		{name: "unparsable", url: "https://example.com/%zz", domain: domain, wantErr: "absolute http or https URL"},
		{name: "too long", url: "https://example.com/" + strings.Repeat("a", MaxForwardURLLength), domain: domain, wantErr: "at most"},

		{name: "own webhook endpoint", url: "https://webhooks.example.com/webhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own webhook subpath", url: "https://webhooks.example.com/webhooks/abc/orders/42?x=1", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own webhooks root", url: "https://webhooks.example.com/webhooks", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint, host case and trailing dot", url: "https://WebHooks.Example.com./webhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint, explicit default port", url: "https://webhooks.example.com:443/webhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint over plain http", url: "http://webhooks.example.com/webhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint, dot segments", url: "https://webhooks.example.com/api/../webhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint, double slash", url: "https://webhooks.example.com//webhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint, percent-encoded path", url: "https://webhooks.example.com/%77ebhooks/abc", domain: domain, wantErr: "own webhook endpoints"},
		{name: "own endpoint on a local instance", url: "http://localhost:3000/webhooks/abc", domain: "http://localhost:3000", wantErr: "own webhook endpoints"},
		{name: "own endpoint under a DOMAIN path prefix", url: "https://example.com/tester/webhooks/abc", domain: "https://example.com/tester/", wantErr: "own webhook endpoints"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateForwardURL(c.url, c.domain)
			if c.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, c.wantErr)
			}
		})
	}
}

func TestWebhookValidate_ForwardURL(t *testing.T) {
	t.Setenv("DOMAIN", "https://webhooks.example.com")
	withForwardURL := func(forwardURL *string) *Webhook {
		return &Webhook{Title: "t", ResponseCode: 200, ForwardURL: forwardURL}
	}
	ok, bad, loop := "https://api.example.com/hooks", "not a url", "https://webhooks.example.com/webhooks/abc"

	assert.NoError(t, withForwardURL(nil).Validate(), "no forward URL is valid")
	assert.NoError(t, withForwardURL(&ok).Validate())
	assert.ErrorContains(t, withForwardURL(&bad).Validate(), "forward URL")
	assert.ErrorContains(t, withForwardURL(&loop).Validate(), "own webhook endpoints", "the loop check uses DOMAIN")
}

func TestWebhookNormalize_ForwardURL(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t\n"} {
		w := Webhook{ForwardURL: &blank}
		w.Normalize()
		assert.Nil(t, w.ForwardURL, "a blank forward URL is unset: %q", blank)
	}

	padded := "  https://api.example.com/hooks \n"
	w := Webhook{ForwardURL: &padded}
	w.Normalize()
	require.NotNil(t, w.ForwardURL)
	assert.Equal(t, "https://api.example.com/hooks", *w.ForwardURL)

	w = Webhook{}
	w.Normalize()
	assert.Nil(t, w.ForwardURL)
}
