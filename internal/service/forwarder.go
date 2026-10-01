package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"gorm.io/datatypes"

	"webhook-tester/config"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
	"webhook-tester/internal/utils"
)

// RequestIDHeader is added to every forwarded request, carrying the ID of
// the captured request, so the target's logs can be matched to it.
const RequestIDHeader = "X-Webhook-Tester-Request-Id"

// Delivery errors recorded for forwards that weren't attempted.
const (
	// ErrMsgQueueFull is the error of an automatic forward dropped because
	// the maximum number of forwards were already in flight.
	ErrMsgQueueFull = "forwarding queue full"
	// ErrMsgDestinationNotAllowed starts the error of a forward whose
	// destination resolved to an address forwarding may not reach.
	ErrMsgDestinationNotAllowed = "destination not allowed"
)

// errDestinationNotAllowed is returned by the dial guard for an address
// forwarding may not reach.
var errDestinationNotAllowed = errors.New(ErrMsgDestinationNotAllowed)

// hopByHopHeaders apply to a single connection, so they are never relayed
// (RFC 9110 section 7.6.1). Host and Content-Length are set by the client
// for the outgoing request instead.
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
	"Host":                true,
	"Content-Length":      true,
}

// DeliveryPublisher publishes recorded deliveries to live subscribers.
// WebhookService implements it.
type DeliveryPublisher interface {
	PublishDelivery(d models.Delivery)
}

// Forwarder relays captured requests to their webhook's forward URL and
// records each attempt as a delivery.
type Forwarder struct {
	client     *http.Client
	timeout    time.Duration
	deliveries repository.DeliveryRepository
	publisher  DeliveryPublisher
	metrics    metrics.Recorder
	logger     *log.Logger

	// slots bounds the automatic forwards in flight; inFlight tracks them
	// so Wait can drain them.
	slots    chan struct{}
	inFlight sync.WaitGroup
}

// NewForwarder builds a Forwarder from the forwarding settings. A zero
// Timeout or MaxConcurrent falls back to the config defaults.
func NewForwarder(
	cfg config.Forwarding,
	deliveries repository.DeliveryRepository,
	publisher DeliveryPublisher,
	rec metrics.Recorder,
	logger *log.Logger,
) *Forwarder {
	if cfg.Timeout <= 0 {
		cfg.Timeout = config.DefaultForwardTimeout
	}
	if cfg.MaxConcurrent < 1 {
		cfg.MaxConcurrent = config.DefaultForwardMaxConcurrent
	}
	return &Forwarder{
		client:     newForwardClient(cfg),
		timeout:    cfg.Timeout,
		deliveries: deliveries,
		publisher:  publisher,
		metrics:    rec,
		logger:     logger,
		slots:      make(chan struct{}, cfg.MaxConcurrent),
	}
}

// newForwardClient returns the client forwards are sent with: bounded by the
// timeout, without compression (which would add an Accept-Encoding header
// the original request didn't have), without proxies, and without following
// redirects, so a redirect is recorded as the answer and can't lead the
// request past the dial guard's checks to a different host.
func newForwardClient(cfg config.Forwarding) *http.Client {
	dialer := &net.Dialer{Timeout: cfg.Timeout, KeepAlive: 30 * time.Second}
	if !cfg.AllowPrivateNetworks {
		dialer.Control = denyPrivateAddresses
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	transport.DisableCompression = true
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// denyPrivateAddresses is a net.Dialer Control function. It runs for each
// resolved address just before connecting, so it also catches hostnames that
// resolve, or are rebound, to an address forwarding mustn't reach.
func denyPrivateAddresses(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errDestinationNotAllowed
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !isPublicAddr(ip) {
		return errDestinationNotAllowed
	}
	return nil
}

// nonPublicPrefixes are ranges outside the standard library's predicates
// that still don't lead to the public internet.
var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network"
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT, used internally by some clouds
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),   // reserved, incl. broadcast
	netip.MustParsePrefix("64:ff9b::/96"),  // NAT64, which maps to any IPv4 address
}

// isPublicAddr reports whether ip is a public unicast address: not private,
// loopback, link-local, unspecified, multicast or otherwise reserved.
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

// Forward relays wr to wh's forward URL, records the attempt as a delivery
// with the given trigger, publishes it and returns it. Every outcome,
// including a network error, is a delivery; failing to store it is only
// logged, since that happens when wr was deleted while it was in flight.
func (f *Forwarder) Forward(ctx context.Context, wh models.Webhook, wr models.WebhookRequest, trigger models.DeliveryTrigger) models.Delivery {
	d := models.Delivery{
		ID:        utils.GenerateID(),
		RequestID: wr.ID,
		WebhookID: wr.WebhookID,
		Trigger:   trigger,
		StartedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	outcome := f.send(ctx, wh, wr, &d)
	f.record(&d, outcome)
	return d
}

// ForwardAsync forwards wr in the background with trigger auto. If the
// maximum number of forwards are already in flight, it records a "forwarding
// queue full" delivery instead of waiting.
func (f *Forwarder) ForwardAsync(wh models.Webhook, wr models.WebhookRequest) {
	select {
	case f.slots <- struct{}{}:
	default:
		d := models.Delivery{
			ID:        utils.GenerateID(),
			RequestID: wr.ID,
			WebhookID: wr.WebhookID,
			Trigger:   models.DeliveryTriggerAuto,
			StartedAt: time.Now().UTC().Truncate(time.Microsecond),
		}
		if wh.ForwardURL != nil {
			d.TargetURL, _ = forwardTarget(*wh.ForwardURL, wr)
		}
		d.Error = ptr(ErrMsgQueueFull)
		f.record(&d, metrics.DeliveryOutcomeError)
		return
	}

	f.inFlight.Add(1)
	go func() {
		defer func() {
			<-f.slots
			f.inFlight.Done()
		}()
		// Not tied to the capture's request context, which ends as soon as
		// the provider has its response.
		f.Forward(context.Background(), wh, wr, models.DeliveryTriggerAuto)
	}()
}

// Wait blocks until the automatic forwards in flight have finished, or ctx
// is done.
func (f *Forwarder) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		f.inFlight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// send makes the outbound request and fills in d with what came back,
// returning the outcome for the metrics.
func (f *Forwarder) send(ctx context.Context, wh models.Webhook, wr models.WebhookRequest, d *models.Delivery) metrics.DeliveryOutcome {
	start := time.Now()
	defer func() { d.DurationMs = time.Since(start).Milliseconds() }()

	if wh.ForwardURL == nil {
		d.Error = ptr("the webhook has no forward URL")
		return metrics.DeliveryOutcomeError
	}
	target, err := forwardTarget(*wh.ForwardURL, wr)
	if err != nil {
		d.Error = ptr(fmt.Sprintf("invalid forward URL: %v", err))
		return metrics.DeliveryOutcomeError
	}
	d.TargetURL = target

	req, err := http.NewRequestWithContext(ctx, wr.Method, target, strings.NewReader(wr.Body))
	if err != nil {
		d.Error = ptr(fmt.Sprintf("couldn't build the request: %v", err))
		return metrics.DeliveryOutcomeError
	}
	req.Header = forwardHeaders(wr)

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, errDestinationNotAllowed) {
			d.Error = ptr(blockedMessage(req.URL.Hostname()))
			return metrics.DeliveryOutcomeBlocked
		}
		d.Error = ptr(f.describeError(err))
		return metrics.DeliveryOutcomeError
	}
	defer func() { _ = resp.Body.Close() }()

	d.StatusCode = &resp.StatusCode
	d.ResponseHeaders = datatypes.JSONMap{}
	for k, v := range resp.Header {
		d.ResponseHeaders[k] = strings.Join(v, ",")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, models.MaxDeliveryResponseBody+1))
	if len(body) > models.MaxDeliveryResponseBody {
		body = body[:models.MaxDeliveryResponseBody]
		d.ResponseBodyTruncated = true
	}
	d.ResponseBody = storableText(body)
	if err != nil {
		d.Error = ptr("reading the response body: " + f.describeError(err))
	}
	return metrics.DeliveryOutcomeForStatus(resp.StatusCode)
}

// record stores, publishes and counts the delivery.
func (f *Forwarder) record(d *models.Delivery, outcome metrics.DeliveryOutcome) {
	f.metrics.ObserveDelivery(outcome, time.Duration(d.DurationMs)*time.Millisecond)
	if err := f.deliveries.Insert(d); err != nil {
		f.logger.Printf("forward: delivery for request %s not stored (deleted meanwhile?): %v", d.RequestID, err)
		return
	}
	f.publisher.PublishDelivery(*d)
}

// forwardTarget joins the forward URL with the captured request's subpath
// and merges in its query parameters, keeping any the forward URL has.
func forwardTarget(forwardURL string, wr models.WebhookRequest) (string, error) {
	u, err := url.Parse(forwardURL)
	if err != nil {
		return "", err
	}
	if wr.Path != "" {
		u.Path = strings.TrimSuffix(u.Path, "/") + wr.Path
		u.RawPath = ""
	}
	if len(wr.Query) > 0 {
		q := u.Query()
		for k, v := range wr.Query {
			if s, ok := v.(string); ok {
				q.Add(k, s)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// forwardHeaders are the captured request's headers minus the hop-by-hop
// ones, including any the Connection header names, plus RequestIDHeader.
func forwardHeaders(wr models.WebhookRequest) http.Header {
	drop := map[string]bool{}
	if c, ok := wr.Headers["Connection"].(string); ok {
		for _, name := range strings.Split(c, ",") {
			drop[textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))] = true
		}
	}

	h := http.Header{}
	for k, v := range wr.Headers {
		s, ok := v.(string)
		key := textproto.CanonicalMIMEHeaderKey(k)
		if !ok || hopByHopHeaders[key] || drop[key] {
			continue
		}
		h.Set(key, s)
	}
	// Go adds its own User-Agent unless one is set; an empty value sends
	// none, as the original request did.
	if _, ok := h["User-Agent"]; !ok {
		h.Set("User-Agent", "")
	}
	h.Set(RequestIDHeader, wr.ID)
	return h
}

// blockedMessage explains why forwarding to host was refused.
func blockedMessage(host string) string {
	verb := "resolves to"
	if _, err := netip.ParseAddr(host); err == nil {
		verb = "is"
	}
	return fmt.Sprintf("%s: %s %s a private or reserved address", ErrMsgDestinationNotAllowed, host, verb)
}

// describeError turns a client error into a short message saying why the
// target wasn't reached.
func (f *Forwarder) describeError(err error) string {
	var (
		dnsErr  *net.DNSError
		certErr *tls.CertificateVerificationError
		uaErr   x509.UnknownAuthorityError
		hostErr x509.HostnameError
		invErr  x509.CertificateInvalidError
		recErr  tls.RecordHeaderError
		netErr  net.Error
	)
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled before the target answered"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return fmt.Sprintf("timed out after %s", f.timeout)
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection reset by the target"
	case errors.As(err, &dnsErr):
		if dnsErr.IsNotFound {
			return fmt.Sprintf("DNS lookup failed: no such host %s", dnsErr.Name)
		}
		return fmt.Sprintf("DNS lookup failed for %s: %s", dnsErr.Name, dnsErr.Err)
	case errors.As(err, &certErr), errors.As(err, &uaErr), errors.As(err, &hostErr), errors.As(err, &invErr):
		return "TLS certificate not accepted: " + innermost(err).Error()
	case errors.As(err, &recErr):
		return "TLS handshake failed: the target didn't answer with TLS"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "the target closed the connection without answering"
	}
	return innermost(err).Error()
}

// innermost drops the url.Error wrapper, whose message repeats the method
// and URL.
func innermost(err error) error {
	var uErr *url.Error
	if errors.As(err, &uErr) {
		return uErr.Err
	}
	return err
}

// storableText makes a response body safe to store as text: Postgres
// rejects invalid UTF-8 and NUL bytes. A body truncated mid-character loses
// only that character.
func storableText(b []byte) string {
	s := strings.ToValidUTF8(string(b), "�")
	return strings.ReplaceAll(s, "\x00", "�")
}

func ptr[T any](v T) *T { return &v }
