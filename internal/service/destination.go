package service

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// forwardURLLookupTimeout bounds the DNS lookup of a forward URL's host when
// it's saved, so a slow resolver doesn't hold up the form or API call.
const forwardURLLookupTimeout = 2 * time.Second

// Resolver looks up a host's IP addresses. *net.Resolver implements it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// ForwardPolicy says which destinations this instance forwards to. It's set
// from the forwarding settings, and lets a forward URL the forwarder could
// never reach be rejected when it's saved, rather than with the first
// blocked delivery.
type ForwardPolicy struct {
	// AllowPrivateNetworks mirrors config.Forwarding.AllowPrivateNetworks:
	// when set, forwards may reach private and loopback addresses.
	AllowPrivateNetworks bool
	// Resolver looks up forward URL hosts; nil uses net.DefaultResolver.
	Resolver Resolver
}

// checkDestination reports an error if forwardURL, an already validated
// absolute URL, points only at addresses the forwarder refuses to dial: a
// literal non-public IP, localhost or a *.localhost name, or a hostname that
// resolves to non-public addresses alone. A failed or slow lookup passes,
// since the forwarder's dial-time guard has the final say anyway.
func (p ForwardPolicy) checkDestination(ctx context.Context, forwardURL string) error {
	if p.AllowPrivateNetworks {
		return nil
	}
	u, err := url.Parse(forwardURL)
	if err != nil {
		return nil // Validate reports malformed URLs
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if !isPublicAddr(ip) {
			return privateDestinationError(host)
		}
		return nil
	}
	name := strings.TrimSuffix(strings.ToLower(host), ".")
	if name == "localhost" || strings.HasSuffix(name, ".localhost") {
		return privateDestinationError(host)
	}

	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(ctx, forwardURLLookupTimeout)
	defer cancel()
	addrs, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addrs) == 0 {
		return nil
	}
	for _, ip := range addrs {
		if isPublicAddr(ip) {
			return nil
		}
	}
	return privateDestinationError(host)
}

func privateDestinationError(host string) error {
	return fmt.Errorf("forward URL points to a private or local address (%s), which this server can't reach. "+
		"To reach a local server, use a public tunnel URL (ngrok, cloudflared)", host)
}

// nonPublicPrefixes are ranges outside the standard library's predicates
// that still don't lead to the public internet.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network"
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT, used internally by some clouds
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved, incl. broadcast
	// IPv6 ranges that embed an IPv4 address, which may be a private one:
	netip.MustParsePrefix("64:ff9b::/96"),   // NAT64
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	netip.MustParsePrefix("2002::/16"),      // 6to4
	netip.MustParsePrefix("2001::/32"),      // Teredo
}

// isPublicAddr reports whether ip is a public unicast address: not private,
// loopback, link-local, unspecified, multicast or otherwise reserved. Both
// the forwarder's dial guard and the save-time check use it.
func isPublicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range nonPublicPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
