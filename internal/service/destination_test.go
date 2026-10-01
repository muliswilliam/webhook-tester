package service

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"webhook-tester/internal/models"
)

// staticResolver resolves the hosts it lists and fails every other lookup,
// as for an unknown host, so tests never query real DNS.
type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if addrs, ok := r[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// slowResolver answers only when its context ends.
type slowResolver struct{}

func (slowResolver) LookupNetIP(ctx context.Context, _, _ string) ([]netip.Addr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// offlinePolicy is the default policy without real DNS.
var offlinePolicy = ForwardPolicy{Resolver: staticResolver{}}

func addrs(ips ...string) []netip.Addr {
	list := make([]netip.Addr, len(ips))
	for i, ip := range ips {
		list[i] = netip.MustParseAddr(ip)
	}
	return list
}

func TestWebhookService_ValidateWebhook_PrivateForwardURLs(t *testing.T) {
	resolver := staticResolver{
		"api.example.com":      addrs("93.184.215.14"),
		"db.internal.example":  addrs("10.0.0.7"),
		"loop.example.com":     addrs("127.0.0.1", "::1"),
		"mixed.example.com":    addrs("10.0.0.7", "93.184.215.14"),
		"metadata.example.com": addrs("169.254.169.254"),
	}
	cases := map[string]struct {
		url     string
		private bool // rejected unless private networks are allowed
	}{
		"public hostname":                   {url: "https://api.example.com/hooks"},
		"public IPv4":                       {url: "http://93.184.215.14:8080/hooks"},
		"public IPv6":                       {url: "http://[2606:4700::1]/hooks"},
		"unresolvable hostname":             {url: "https://nowhere.example.com/hooks"},
		"hostname with a public address":    {url: "https://mixed.example.com/hooks"},
		"loopback IPv4":                     {url: "http://127.0.0.1:8080/hooks", private: true},
		"loopback IPv6":                     {url: "http://[::1]:8080/hooks", private: true},
		"IPv4-mapped loopback":              {url: "http://[::ffff:127.0.0.1]/hooks", private: true},
		"private network":                   {url: "http://192.168.1.20/hooks", private: true},
		"link-local (cloud metadata)":       {url: "http://169.254.169.254/latest/meta-data", private: true},
		"unspecified address":               {url: "http://0.0.0.0:8080/", private: true},
		"carrier-grade NAT":                 {url: "http://100.64.0.1/", private: true},
		"localhost":                         {url: "http://localhost:8080/hooks", private: true},
		"localhost, any case, trailing dot": {url: "http://LocalHost.:8080/hooks", private: true},
		"subdomain of localhost":            {url: "http://api.localhost:3000/hooks", private: true},
		"hostname resolving to private":     {url: "http://db.internal.example/hooks", private: true},
		"hostname resolving to loopback":    {url: "https://loop.example.com/hooks", private: true},
		"hostname resolving to link-local":  {url: "http://metadata.example.com/", private: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			wh := &models.Webhook{Title: "t", ResponseCode: 200, ForwardURL: &c.url}

			deny := NewWebhookService(newFakeWebhookRepo(), &fakeDeliveryRepo{}, "", ForwardPolicy{Resolver: resolver})
			err := deny.ValidateWebhook(context.Background(), wh, nil)
			if c.private {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "forward URL points to a private or local address")
				assert.Contains(t, err.Error(), "use a public tunnel URL (ngrok, cloudflared)")
			} else {
				assert.NoError(t, err)
			}

			allow := NewWebhookService(newFakeWebhookRepo(), &fakeDeliveryRepo{}, "",
				ForwardPolicy{AllowPrivateNetworks: true, Resolver: resolver})
			assert.NoError(t, allow.ValidateWebhook(context.Background(), wh, nil), "allowed by the policy")
		})
	}
}

// The error names the host, so the user sees which part is wrong.
func TestWebhookService_ValidateWebhook_PrivateForwardURLMessage(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo(), &fakeDeliveryRepo{}, "", offlinePolicy)
	forwardURL := "http://localhost:8080/hooks"
	err := svc.ValidateWebhook(context.Background(), &models.Webhook{Title: "t", ResponseCode: 200, ForwardURL: &forwardURL}, nil)
	assert.EqualError(t, err, "forward URL points to a private or local address (localhost), which this server can't reach. "+
		"To reach a local server, use a public tunnel URL (ngrok, cloudflared)")
}

// A lookup that doesn't answer in time doesn't block saving: the dial-time
// guard still applies to every forward. The lookup is bounded by the
// caller's context too, which keeps the test fast.
func TestWebhookService_ValidateWebhook_SlowLookupAccepts(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo(), &fakeDeliveryRepo{}, "", ForwardPolicy{Resolver: slowResolver{}})
	forwardURL := "https://slow.example.com/hooks"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := svc.ValidateWebhook(ctx, &models.Webhook{Title: "t", ResponseCode: 200, ForwardURL: &forwardURL}, nil)
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), time.Second)
}

// Other validation problems are reported before the address is checked.
func TestWebhookService_ValidateWebhook_ModelErrorsFirst(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo(), &fakeDeliveryRepo{}, "", offlinePolicy)
	forwardURL := "http://localhost/hooks"
	err := svc.ValidateWebhook(context.Background(), &models.Webhook{ResponseCode: 200, ForwardURL: &forwardURL}, nil)
	assert.EqualError(t, err, "title is required")
	assert.NoError(t, svc.ValidateWebhook(context.Background(), &models.Webhook{Title: "t", ResponseCode: 200}, nil), "no forward URL")
}
