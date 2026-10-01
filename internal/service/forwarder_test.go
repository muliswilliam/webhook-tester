package service

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"webhook-tester/internal/models"
)

func TestIsPublicAddr(t *testing.T) {
	for _, ip := range []string{"8.8.8.8", "93.184.216.34", "2606:4700:4700::1111"} {
		assert.True(t, isPublicAddr(netip.MustParseAddr(ip)), ip)
	}
	for _, ip := range []string{
		"127.0.0.1", "127.8.9.10", "::1", // loopback
		"10.0.0.1", "172.16.5.4", "192.168.1.1", "fd12::1", // private
		"169.254.169.254", "fe80::1", // link-local, incl. cloud metadata
		"0.0.0.0", "::", "0.1.2.3", // unspecified, "this network"
		"224.0.0.1", "ff02::1", // multicast
		"100.64.0.1", "198.18.0.1", "192.0.0.1", "255.255.255.255", "240.0.0.1", // reserved ranges
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", // IPv4-mapped
		"64:ff9b::7f00:1", // NAT64 of 127.0.0.1
	} {
		assert.False(t, isPublicAddr(netip.MustParseAddr(ip)), ip)
	}
}

func TestForwardTarget(t *testing.T) {
	for name, tc := range map[string]struct {
		forwardURL string
		path       string
		query      datatypes.JSONMap
		want       string
	}{
		"no subpath":                  {forwardURL: "https://api.example.com/hooks", want: "https://api.example.com/hooks"},
		"subpath":                     {forwardURL: "https://api.example.com/hooks", path: "/orders/42", want: "https://api.example.com/hooks/orders/42"},
		"subpath, trailing slash":     {forwardURL: "https://api.example.com/hooks/", path: "/orders/42", want: "https://api.example.com/hooks/orders/42"},
		"root subpath":                {forwardURL: "https://api.example.com/hooks", path: "/", want: "https://api.example.com/hooks/"},
		"bare host":                   {forwardURL: "https://api.example.com", path: "/orders", want: "https://api.example.com/orders"},
		"query merged":                {forwardURL: "https://api.example.com/hooks?token=abc", query: datatypes.JSONMap{"x": "1"}, want: "https://api.example.com/hooks?token=abc&x=1"},
		"same query key keeps both":   {forwardURL: "https://api.example.com/hooks?x=0", query: datatypes.JSONMap{"x": "1"}, want: "https://api.example.com/hooks?x=0&x=1"},
		"forward URL query untouched": {forwardURL: "https://api.example.com/hooks?b=2&a=1", want: "https://api.example.com/hooks?b=2&a=1"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := forwardTarget(tc.forwardURL, models.WebhookRequest{Path: tc.path, Query: tc.query})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestForwardHeaders(t *testing.T) {
	h := forwardHeaders(models.WebhookRequest{
		ID: "req-1",
		Headers: datatypes.JSONMap{
			"Content-Type":        "application/json",
			"X-Hub-Signature-256": "sha256=abc",
			"Connection":          "keep-alive, X-Hop",
			"X-Hop":               "1",
			"Transfer-Encoding":   "chunked",
			"Te":                  "trailers",
			"Proxy-Authorization": "Basic xyz",
			"Content-Length":      "12",
			"User-Agent":          "GitHub-Hookshot/abc",
		},
	})
	assert.Equal(t, "application/json", h.Get("Content-Type"))
	assert.Equal(t, "sha256=abc", h.Get("X-Hub-Signature-256"))
	assert.Equal(t, "GitHub-Hookshot/abc", h.Get("User-Agent"))
	assert.Equal(t, "req-1", h.Get(RequestIDHeader))
	for _, dropped := range []string{"Connection", "X-Hop", "Transfer-Encoding", "Te", "Proxy-Authorization", "Content-Length"} {
		assert.NotContains(t, h, dropped)
	}
}
