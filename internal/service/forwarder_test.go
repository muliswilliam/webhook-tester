package service

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"

	"webhook-tester/internal/models"
)

func TestIsPublicAddr(t *testing.T) {
	for _, ip := range []string{"8.8.8.8", "93.184.216.34", "2606:4700:4700::1111", "2001:4860:4860::8888"} {
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
		"64:ff9b::7f00:1", "64:ff9b:1::a00:1", // NAT64 of 127.0.0.1, local-use NAT64 of 10.0.0.1
		"2002:7f00:1::1", "2002:a9fe:a9fe::1", // 6to4 of 127.0.0.1 and 169.254.169.254
		"2001:0:4136:e378:8000:63bf:3fff:fdd2", // Teredo
		"::7f00:1", "::a00:1",                  // IPv4-compatible (deprecated) of 127.0.0.1 and 10.0.0.1
		"::ffff:0:7f00:1", // SIIT of 127.0.0.1
		"fec0::1",         // site-local (deprecated)
		"100::1",          // discard-only
		"4000::1",         // outside 2000::/3, the global unicast block
	} {
		assert.False(t, isPublicAddr(netip.MustParseAddr(ip)), ip)
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
			"X-Multi":             []any{"one", "two, three"},
			"X-Number":            5.0,
		},
	})
	assert.Equal(t, "application/json", h.Get("Content-Type"))
	assert.Equal(t, "sha256=abc", h.Get("X-Hub-Signature-256"))
	assert.Equal(t, "GitHub-Hookshot/abc", h.Get("User-Agent"))
	assert.Equal(t, "req-1", h.Get(RequestIDHeader))
	assert.Equal(t, []string{"one", "two, three"}, h.Values("X-Multi"), "repeated values stay separate")
	assert.Equal(t, "5", h.Get("X-Number"), "a non-string value is formatted, not dropped")
	for _, dropped := range []string{"Connection", "X-Hop", "Transfer-Encoding", "Te", "Proxy-Authorization", "Content-Length"} {
		assert.NotContains(t, h, dropped)
	}
}
